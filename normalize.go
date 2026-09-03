package scim

import (
	"fmt"
	"strings"
)

// NormalizeKeys recursively canonicalizes known SCIM attribute spelling and
// rejects case-equivalent collisions instead of choosing one attacker-supplied
// value.
func NormalizeKeys(value any, canonical map[string]string) (any, error) {
	switch typed := value.(type) {
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			normalized, err := NormalizeKeys(item, canonical)
			if err != nil {
				return nil, err
			}
			result[index] = normalized
		}
		return result, nil
	case map[string]any:
		result := make(map[string]any, len(typed))
		seen := make(map[string]string, len(typed))
		for key, item := range typed {
			folded := strings.ToLower(key)
			normalizedKey := key
			if admitted, exists := canonical[folded]; exists {
				normalizedKey = admitted
			}
			collisionKey := strings.ToLower(normalizedKey)
			if previous, exists := seen[collisionKey]; exists {
				return nil, fmt.Errorf("attributes %q and %q are case-equivalent", previous, key)
			}
			normalized, err := NormalizeKeys(item, canonical)
			if err != nil {
				return nil, err
			}
			seen[collisionKey] = key
			result[normalizedKey] = normalized
		}
		return result, nil
	default:
		return value, nil
	}
}

// CoreKeyCases is the protocol key set used by User, Group, PATCH, and Bulk
// messages. Extensions can pass an augmented copy to NormalizeKeys.
func CoreKeyCases() map[string]string {
	keys := []string{
		"Operations", "Resources", "$ref", "active", "addresses", "attributes",
		"authenticationSchemes", "bulk", "bulkId", "caseExact", "changePassword",
		"country", "created", "data", "description", "display", "displayName",
		"emails", "endpoint", "entitlements", "excludedAttributes", "externalId",
		"failOnErrors", "familyName", "filter", "formatted", "givenName", "groups",
		"honorificPrefix", "honorificSuffix", "id", "ims", "itemsPerPage",
		"lastModified", "locale", "locality", "location", "maxOperations",
		"maxPayloadSize", "maxResults", "members", "meta", "method", "middleName",
		"multiValued", "mutability", "name", "nickName", "op", "password", "patch",
		"path", "phoneNumbers", "photos", "postalCode", "preferredLanguage", "primary",
		"profileUrl", "region", "required", "response", "returned", "roles", "schema",
		"schemaExtensions", "schemas", "sort", "startIndex", "status", "streetAddress",
		"subAttributes", "supported", "timezone", "title", "totalResults", "type",
		"uniqueness", "userName", "userType", "value", "version", "x509Certificates",
	}
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		result[strings.ToLower(key)] = key
	}
	return result
}
