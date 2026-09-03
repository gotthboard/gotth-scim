package scim

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaximumPatchBytes = 1 << 20

// PatchRequest is the RFC 7644 PATCH envelope.
type PatchRequest struct {
	Schemas    []string  `json:"schemas"`
	Operations []PatchOp `json:"Operations"`
}

// PatchOp is one add, remove, or replace operation.
type PatchOp struct {
	Op    string          `json:"op"`
	Path  string          `json:"path,omitempty"`
	Value json.RawMessage `json:"value,omitempty"`
}

// DecodePatch performs bounded strict JSON decoding and validates the generic
// PATCH envelope. It does not guess extension-schema or storage semantics.
func DecodePatch(raw []byte, maximumOperations int) (PatchRequest, error) {
	if len(raw) == 0 || len(raw) > MaximumPatchBytes || maximumOperations < 1 || maximumOperations > 1000 {
		return PatchRequest{}, fmt.Errorf("PATCH request boundary is invalid")
	}
	if _, err := DecodeDocument(raw); err != nil {
		return PatchRequest{}, fmt.Errorf("PATCH request JSON is invalid")
	}
	var request PatchRequest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || decoder.Decode(&struct{}{}) == nil {
		return PatchRequest{}, fmt.Errorf("PATCH request JSON is invalid")
	}
	if err := ValidateSchemas(request.Schemas, []string{PatchSchema}); err != nil {
		return PatchRequest{}, err
	}
	if len(request.Operations) == 0 || len(request.Operations) > maximumOperations {
		return PatchRequest{}, fmt.Errorf("PATCH operation count is invalid")
	}
	for index := range request.Operations {
		operation := &request.Operations[index]
		operation.Op = strings.ToLower(operation.Op)
		switch operation.Op {
		case "add", "replace":
			if len(operation.Value) == 0 || bytes.Equal(bytes.TrimSpace(operation.Value), []byte("null")) {
				return PatchRequest{}, fmt.Errorf("PATCH operation %d requires a value", index)
			}
		case "remove":
			if operation.Path == "" || len(operation.Value) != 0 {
				return PatchRequest{}, fmt.Errorf("PATCH remove operation %d has an invalid path or value", index)
			}
		default:
			return PatchRequest{}, fmt.Errorf("PATCH operation %d has an invalid op", index)
		}
		if operation.Path != "" && !validAttributePath(operation.Path) {
			return PatchRequest{}, fmt.Errorf("PATCH operation %d has an invalid attribute path", index)
		}
	}
	return request, nil
}

func validAttributePath(path string) bool {
	if len(path) > 1024 || !utf8.ValidString(path) || strings.TrimSpace(path) != path || strings.IndexFunc(path, unicode.IsControl) >= 0 {
		return false
	}
	if strings.ContainsAny(path, "{}<>\\") {
		return false
	}
	return path != ""
}
