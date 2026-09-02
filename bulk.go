package scim

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	MaximumBulkOperations = 100
	MaximumBulkBytes      = 1 << 20
)

// BulkRequest is the bounded RFC 7644 bulk envelope.
type BulkRequest struct {
	Schemas      []string        `json:"schemas"`
	FailOnErrors int             `json:"failOnErrors,omitempty"`
	Operations   []BulkOperation `json:"Operations"`
}

// BulkOperation is one independent SCIM operation. Execution and transaction
// policy remain caller-owned.
type BulkOperation struct {
	Method  string          `json:"method"`
	BulkID  string          `json:"bulkId,omitempty"`
	Path    string          `json:"path"`
	Version string          `json:"version,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// DecodeBulk performs bounded strict decoding and generic method/path/bulkId
// validation without executing or partially committing anything.
func DecodeBulk(raw []byte) (BulkRequest, error) {
	if len(raw) == 0 || len(raw) > MaximumBulkBytes {
		return BulkRequest{}, fmt.Errorf("Bulk request size is invalid")
	}
	var request BulkRequest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || decoder.Decode(&struct{}{}) == nil {
		return BulkRequest{}, fmt.Errorf("Bulk request JSON is invalid")
	}
	if err := ValidateSchemas(request.Schemas, []string{BulkRequestSchema}); err != nil {
		return BulkRequest{}, err
	}
	if len(request.Operations) == 0 || len(request.Operations) > MaximumBulkOperations || request.FailOnErrors < 0 || request.FailOnErrors > MaximumBulkOperations {
		return BulkRequest{}, fmt.Errorf("Bulk operation boundary is invalid")
	}
	seenBulkIDs := make(map[string]struct{}, len(request.Operations))
	for index := range request.Operations {
		operation := &request.Operations[index]
		operation.Method = strings.ToUpper(operation.Method)
		collection, resourceID, bulkReference, err := ParseBulkPath(operation.Path)
		if err != nil {
			return BulkRequest{}, fmt.Errorf("Bulk operation %d: %w", index, err)
		}
		switch operation.Method {
		case "POST":
			if resourceID != "" || bulkReference != "" || !validBulkID(operation.BulkID) || !isJSONObject(operation.Data) {
				return BulkRequest{}, fmt.Errorf("Bulk POST operation %d is invalid", index)
			}
			if _, exists := seenBulkIDs[operation.BulkID]; exists {
				return BulkRequest{}, fmt.Errorf("Bulk bulkId %q is duplicated", operation.BulkID)
			}
			seenBulkIDs[operation.BulkID] = struct{}{}
		case "PUT", "PATCH":
			if collection == "" || resourceID == "" && bulkReference == "" || !isJSONObject(operation.Data) || operation.BulkID != "" {
				return BulkRequest{}, fmt.Errorf("Bulk mutation operation %d is invalid", index)
			}
			if _, resolved := seenBulkIDs[bulkReference]; bulkReference != "" && !resolved {
				return BulkRequest{}, fmt.Errorf("Bulk operation %d references an unknown or later bulkId", index)
			}
		case "DELETE":
			if collection == "" || resourceID == "" && bulkReference == "" || len(operation.Data) != 0 || operation.BulkID != "" {
				return BulkRequest{}, fmt.Errorf("Bulk DELETE operation %d is invalid", index)
			}
			if _, resolved := seenBulkIDs[bulkReference]; bulkReference != "" && !resolved {
				return BulkRequest{}, fmt.Errorf("Bulk operation %d references an unknown or later bulkId", index)
			}
		default:
			return BulkRequest{}, fmt.Errorf("Bulk operation %d uses an unsupported method", index)
		}
	}
	return request, nil
}

func validBulkID(value string) bool {
	return len(value) > 0 && len(value) <= 128 && utf8.ValidString(value) && strings.IndexFunc(value, unicode.IsControl) < 0
}

func isJSONObject(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var object map[string]json.RawMessage
	return json.Unmarshal(raw, &object) == nil && object != nil
}

// ParseBulkPath admits only relative Users or Groups collection/resource paths.
// A bulkId reference is returned separately and must be resolved only from a
// prior successful operation.
func ParseBulkPath(raw string) (collection, resourceID, bulkReference string, err error) {
	if raw == "" {
		return "", "", "", fmt.Errorf("Bulk path is empty")
	}
	parsed, parseErr := url.Parse(raw)
	if parseErr != nil || parsed.IsAbs() || parsed.Host != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", "", "", fmt.Errorf("Bulk path is not a relative SCIM path")
	}
	path := strings.TrimPrefix(parsed.EscapedPath(), "/")
	parts := strings.Split(path, "/")
	if len(parts) < 1 || len(parts) > 2 || (parts[0] != "Users" && parts[0] != "Groups") {
		return "", "", "", fmt.Errorf("Bulk path collection is unsupported")
	}
	if len(parts) == 1 {
		return parts[0], "", "", nil
	}
	if parts[1] == "" {
		return "", "", "", fmt.Errorf("Bulk resource path has an empty ID")
	}
	decoded, decodeErr := url.PathUnescape(parts[1])
	if decodeErr != nil || decoded == "" || strings.Contains(decoded, "/") {
		return "", "", "", fmt.Errorf("Bulk resource ID is invalid")
	}
	if strings.HasPrefix(decoded, "bulkId:") {
		reference := strings.TrimPrefix(decoded, "bulkId:")
		if reference == "" {
			return "", "", "", fmt.Errorf("Bulk bulkId reference is empty")
		}
		return parts[0], "", reference, nil
	}
	return parts[0], decoded, "", nil
}
