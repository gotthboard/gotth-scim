package scim

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

// BulkResponse is the RFC 7644 response envelope.
type BulkResponse struct {
	Schemas    []string                `json:"schemas"`
	Operations []BulkResponseOperation `json:"Operations"`
}

// BulkResponseOperation reports one attempted Bulk operation.
type BulkResponseOperation struct {
	Method   string         `json:"method"`
	BulkID   string         `json:"bulkId,omitempty"`
	Location string         `json:"location,omitempty"`
	Version  string         `json:"version,omitempty"`
	Status   string         `json:"status"`
	Response *ErrorResponse `json:"response,omitempty"`
}

type bulkReference struct {
	id       string
	location string
}

func (server *Server) serveBulk(writer http.ResponseWriter, request *http.Request, scope string) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer, "POST")
		return
	}
	if err := rejectQuery(request.URL.Query(), nil); err != nil {
		writeProtocolError(writer, err)
		return
	}
	raw, err := readBody(request)
	if err != nil {
		writeProtocolError(writer, err)
		return
	}
	bulk, err := DecodeBulk(raw)
	if err != nil {
		writeProtocolError(writer, clientError(400, "invalidSyntax", "Bulk request is invalid"))
		return
	}
	response := BulkResponse{Schemas: []string{BulkResponseSchema}, Operations: make([]BulkResponseOperation, 0, len(bulk.Operations))}
	references := make(map[string]bulkReference)
	failures := 0
	for _, operation := range bulk.Operations {
		result, reference, err := server.executeBulkOperation(request, scope, operation, references)
		if err != nil {
			result = bulkFailure(operation, err)
			failures++
		} else if operation.BulkID != "" {
			references[operation.BulkID] = reference
		}
		response.Operations = append(response.Operations, result)
		if bulk.FailOnErrors > 0 && failures >= bulk.FailOnErrors {
			break
		}
	}
	writeJSON(writer, http.StatusOK, response)
}

func (server *Server) executeBulkOperation(request *http.Request, scope string, operation BulkOperation, references map[string]bulkReference) (BulkResponseOperation, bulkReference, error) {
	collection, id, pathReference, err := ParseBulkPath(operation.Path)
	if err != nil {
		return BulkResponseOperation{}, bulkReference{}, err
	}
	if pathReference != "" {
		reference, exists := references[pathReference]
		if !exists {
			return BulkResponseOperation{}, bulkReference{}, clientError(400, "invalidValue", "Bulk reference is unresolved")
		}
		id = reference.id
	}
	definition, exists := server.registry.definitionByEndpoint(collection)
	if !exists {
		return BulkResponseOperation{}, bulkReference{}, clientError(400, "invalidPath", "Bulk collection is unsupported")
	}
	data, err := resolveBulkData(operation.Data, references)
	if err != nil {
		return BulkResponseOperation{}, bulkReference{}, clientError(400, "invalidValue", "Bulk data reference is unresolved")
	}
	result := BulkResponseOperation{Method: operation.Method, BulkID: operation.BulkID}
	switch operation.Method {
	case http.MethodPost:
		record, err := server.create(request.Context(), scope, "", definition, data)
		if err != nil {
			return result, bulkReference{}, err
		}
		result.Status = strconv.Itoa(http.StatusCreated)
		result.Location = server.resourceLocation(definition, record.ID)
		result.Version = record.Version
		return result, bulkReference{id: record.ID, location: result.Location}, nil
	case http.MethodPut:
		record, err := server.replace(request.Context(), scope, definition, id, data, operation.Version, "")
		if err != nil {
			return result, bulkReference{}, err
		}
		result.Status = strconv.Itoa(http.StatusOK)
		result.Location = server.resourceLocation(definition, record.ID)
		result.Version = record.Version
		return result, bulkReference{}, nil
	case http.MethodPatch:
		patch, err := DecodePatch(data, server.maximumPatchOperations)
		if err != nil {
			return result, bulkReference{}, clientError(400, "invalidSyntax", "Bulk PATCH data is invalid")
		}
		record, err := server.patch(request.Context(), scope, definition, id, patch, operation.Version, "")
		if err != nil {
			return result, bulkReference{}, err
		}
		result.Status = strconv.Itoa(http.StatusOK)
		result.Location = server.resourceLocation(definition, record.ID)
		result.Version = record.Version
		return result, bulkReference{}, nil
	case http.MethodDelete:
		if err := server.remove(request.Context(), scope, definition, id, operation.Version, ""); err != nil {
			return result, bulkReference{}, err
		}
		result.Status = strconv.Itoa(http.StatusNoContent)
		return result, bulkReference{}, nil
	default:
		return result, bulkReference{}, clientError(400, "invalidValue", "Bulk method is unsupported")
	}
}

func resolveBulkData(raw json.RawMessage, references map[string]bulkReference) ([]byte, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	document, err := DecodeDocument(raw)
	if err != nil {
		return nil, err
	}
	resolved, err := resolveBulkValue(map[string]any(document), "", references)
	if err != nil {
		return nil, err
	}
	return json.Marshal(resolved)
}

func resolveBulkValue(value any, field string, references map[string]bulkReference) (any, error) {
	switch typed := value.(type) {
	case string:
		if len(typed) <= len("bulkId:") || typed[:len("bulkId:")] != "bulkId:" {
			return typed, nil
		}
		reference, exists := references[typed[len("bulkId:"):]]
		if !exists {
			return nil, fmt.Errorf("unknown bulkId")
		}
		if field == "$ref" {
			return reference.location, nil
		}
		return reference.id, nil
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			resolved, err := resolveBulkValue(item, key, references)
			if err != nil {
				return nil, err
			}
			result[key] = resolved
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			resolved, err := resolveBulkValue(item, field, references)
			if err != nil {
				return nil, err
			}
			result[index] = resolved
		}
		return result, nil
	default:
		return value, nil
	}
}

func bulkFailure(operation BulkOperation, err error) BulkResponseOperation {
	status, scimType, detail := safeFailure(err)
	response, buildErr := NewError(status, scimType, detail)
	if buildErr != nil {
		response = ErrorResponse{Schemas: []string{ErrorSchema}, Status: "500", Detail: "SCIM operation failed"}
		status = 500
	}
	return BulkResponseOperation{Method: operation.Method, BulkID: operation.BulkID, Status: strconv.Itoa(status), Response: &response}
}

func safeFailure(err error) (int, string, string) {
	var protocol *ProtocolError
	switch {
	case errors.As(err, &protocol):
		return protocol.Status, protocol.SCIMType, protocol.Detail
	case errors.Is(err, ErrNotFound):
		return 404, "", "SCIM resource was not found"
	case errors.Is(err, ErrConflict), errors.Is(err, ErrTombstoned):
		return 409, "uniqueness", "SCIM resource conflicts with existing state"
	case errors.Is(err, ErrPrecondition):
		return 412, "", "SCIM resource precondition failed"
	default:
		return 500, "", "SCIM storage operation failed"
	}
}
