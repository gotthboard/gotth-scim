// Package scim implements a storage-neutral SCIM 2.0 HTTP server, protocol
// validation, transactional storage contract, tombstones, and reconciliation.
// Applications own authentication, durable adapters, and product policy.
package scim

import (
	"fmt"
	"strings"
)

const (
	UserSchema                  = "urn:ietf:params:scim:schemas:core:2.0:User"
	GroupSchema                 = "urn:ietf:params:scim:schemas:core:2.0:Group"
	EnterpriseUserSchema        = "urn:ietf:params:scim:schemas:extension:enterprise:2.0:User"
	ListResponseSchema          = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	PatchSchema                 = "urn:ietf:params:scim:api:messages:2.0:PatchOp"
	ErrorSchema                 = "urn:ietf:params:scim:api:messages:2.0:Error"
	BulkRequestSchema           = "urn:ietf:params:scim:api:messages:2.0:BulkRequest"
	BulkResponseSchema          = "urn:ietf:params:scim:api:messages:2.0:BulkResponse"
	SearchRequestSchema         = "urn:ietf:params:scim:api:messages:2.0:SearchRequest"
	ServiceProviderConfigSchema = "urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"
	ResourceTypeSchema          = "urn:ietf:params:scim:schemas:core:2.0:ResourceType"
	SchemaSchema                = "urn:ietf:params:scim:schemas:core:2.0:Schema"
)

// ValidateSchemas requires an exact case-insensitive schema set with no
// duplicates. Extra schemas are not silently ignored.
func ValidateSchemas(actual, expected []string) error {
	if len(actual) != len(expected) || len(actual) == 0 {
		return fmt.Errorf("schemas do not match the required exact set")
	}
	want := make(map[string]struct{}, len(expected))
	for _, schema := range expected {
		if schema == "" {
			return fmt.Errorf("expected schema is empty")
		}
		want[strings.ToLower(schema)] = struct{}{}
	}
	seen := make(map[string]struct{}, len(actual))
	for _, schema := range actual {
		key := strings.ToLower(schema)
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("schemas contain a case-equivalent duplicate")
		}
		if _, admitted := want[key]; !admitted {
			return fmt.Errorf("schemas contain an unexpected value")
		}
		seen[key] = struct{}{}
	}
	return nil
}

// ErrorResponse is the RFC 7644 error envelope.
type ErrorResponse struct {
	Schemas  []string `json:"schemas"`
	Status   string   `json:"status"`
	SCIMType string   `json:"scimType,omitempty"`
	Detail   string   `json:"detail"`
}

// NewError creates a canonical SCIM error without logging or exposing an
// underlying implementation error.
func NewError(status int, scimType, detail string) (ErrorResponse, error) {
	if status < 400 || status > 599 || detail == "" || strings.ContainsAny(detail, "\x00\r\n") {
		return ErrorResponse{}, fmt.Errorf("SCIM error fields are invalid")
	}
	return ErrorResponse{Schemas: []string{ErrorSchema}, Status: fmt.Sprint(status), SCIMType: scimType, Detail: detail}, nil
}
