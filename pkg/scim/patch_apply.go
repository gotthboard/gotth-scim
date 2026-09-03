package scim

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

type patchPath struct {
	extension string
	attribute string
	filter    string
	sub       string
}

// ApplyPatch applies an RFC 7644 PATCH request to a private copy of current
// and returns a fully revalidated replacement document.
func ApplyPatch(definition ResourceDefinition, current Document, request PatchRequest, resourceID string) (Document, []IndexKey, string, error) {
	working, err := applyPatchDocument(definition, current, request)
	if err != nil {
		return nil, nil, "", err
	}
	prepared, indexes, externalID, err := prepareResource(definition, working, ReplaceMode, resourceID)
	if err != nil {
		return nil, nil, "", clientError(400, "invalidValue", "patched resource is invalid")
	}
	return prepared, indexes, externalID, nil
}

func applyPatchDocument(definition ResourceDefinition, current Document, request PatchRequest) (Document, error) {
	working, err := cloneDocument(current)
	if err != nil {
		return nil, err
	}
	for index, operation := range request.Operations {
		var value any
		if len(operation.Value) != 0 {
			value, err = decodePatchValue(operation.Value)
			if err != nil {
				return nil, clientError(400, "invalidValue", fmt.Sprintf("PATCH operation %d value is invalid", index))
			}
		}
		if err := applyPatchOperation(definition, working, operation.Op, operation.Path, value); err != nil {
			return nil, err
		}
	}
	for _, extension := range definition.Extensions {
		if _, exists := working[extension.Schema]; exists {
			ensureSchema(working, extension.Schema)
		}
	}
	return working, nil
}

func decodePatchValue(raw json.RawMessage) (any, error) {
	wrapped := append([]byte(`{"value":`), raw...)
	wrapped = append(wrapped, '}')
	document, err := DecodeDocument(wrapped)
	if err != nil {
		return nil, err
	}
	return document["value"], nil
}

func applyPatchOperation(definition ResourceDefinition, document Document, operation, rawPath string, value any) error {
	if rawPath == "" {
		if operation == "remove" {
			return clientError(400, "noTarget", "PATCH remove requires a path")
		}
		object, ok := value.(map[string]any)
		if !ok {
			return clientError(400, "invalidValue", "pathless PATCH value must be an object")
		}
		for name, item := range object {
			if operation == "add" {
				addValue(document, name, item)
			} else {
				document[name] = item
			}
		}
		return nil
	}
	path, err := parsePatchPath(definition, rawPath)
	if err != nil {
		return clientError(400, "invalidPath", "PATCH path is unsupported")
	}
	target := map[string]any(document)
	if path.extension != "" {
		raw, exists := target[path.extension]
		if !exists {
			if operation != "add" || path.filter != "" {
				return clientError(400, "noTarget", "PATCH extension target does not exist")
			}
			nested := make(map[string]any)
			target[path.extension] = nested
			ensureSchema(document, path.extension)
			target = nested
		} else {
			nested, ok := raw.(map[string]any)
			if !ok {
				return clientError(400, "invalidPath", "PATCH extension target is not an object")
			}
			target = nested
		}
	}
	if path.filter == "" {
		if path.extension == "" && path.attribute == "password" && path.sub == "" && (operation == "add" || operation == "replace") {
			target[path.attribute] = value
			return nil
		}
		return applyDirectPatch(target, path.attribute, path.sub, operation, value)
	}
	return applyFilteredPatch(definition, target, path, operation, value)
}

func parsePatchPath(definition ResourceDefinition, raw string) (patchPath, error) {
	if !validAttributePath(raw) {
		return patchPath{}, fmt.Errorf("invalid path")
	}
	result := patchPath{}
	remaining := raw
	for _, extension := range definition.Extensions {
		prefix := extension.Schema + ":"
		if len(remaining) > len(prefix) && strings.EqualFold(remaining[:len(prefix)], prefix) {
			result.extension = extension.Schema
			remaining = remaining[len(prefix):]
			break
		}
	}
	open := strings.IndexByte(remaining, '[')
	if open >= 0 {
		close := strings.LastIndexByte(remaining, ']')
		if close <= open || strings.Contains(remaining[open+1:close], "[") {
			return patchPath{}, fmt.Errorf("invalid value path")
		}
		result.attribute = remaining[:open]
		result.filter = remaining[open+1 : close]
		if close+1 < len(remaining) {
			if remaining[close+1] != '.' {
				return patchPath{}, fmt.Errorf("invalid value sub-path")
			}
			result.sub = remaining[close+2:]
		}
	} else {
		parts := strings.Split(remaining, ".")
		if len(parts) > 2 {
			return patchPath{}, fmt.Errorf("path is too deep")
		}
		result.attribute = parts[0]
		if len(parts) == 2 {
			result.sub = parts[1]
		}
	}
	if !validName(result.attribute) || result.sub != "" && !validName(result.sub) {
		return patchPath{}, fmt.Errorf("attribute name is invalid")
	}
	canonical := CoreKeyCases()
	if known, exists := canonical[strings.ToLower(result.attribute)]; exists {
		result.attribute = known
	}
	if known, exists := canonical[strings.ToLower(result.sub)]; result.sub != "" && exists {
		result.sub = known
	}
	qualified := qualifiedPatchAttribute(result)
	if result.sub != "" {
		qualified += "." + result.sub
	}
	if _, _, ok := resolveAttributeContract(definition, nil, qualified); !ok {
		return patchPath{}, fmt.Errorf("attribute is not in the resource schema")
	}
	return result, nil
}

func applyDirectPatch(target map[string]any, attribute, sub, operation string, value any) error {
	if sub == "" {
		_, exists := target[attribute]
		switch operation {
		case "remove":
			if !exists {
				return clientError(400, "noTarget", "PATCH target does not exist")
			}
			delete(target, attribute)
		case "replace":
			if !exists {
				return clientError(400, "noTarget", "PATCH target does not exist")
			}
			target[attribute] = value
		case "add":
			addValue(target, attribute, value)
		}
		return nil
	}
	raw, exists := target[attribute]
	if !exists {
		if operation != "add" {
			return clientError(400, "noTarget", "PATCH parent target does not exist")
		}
		nested := make(map[string]any)
		nested[sub] = value
		target[attribute] = nested
		return nil
	}
	object, ok := raw.(map[string]any)
	if !ok {
		return clientError(400, "invalidPath", "PATCH parent target is not complex")
	}
	return applyDirectPatch(object, sub, "", operation, value)
}

func addValue(target map[string]any, attribute string, value any) {
	current, exists := target[attribute]
	if !exists {
		target[attribute] = value
		return
	}
	values, isArray := current.([]any)
	if !isArray {
		target[attribute] = value
		return
	}
	if additions, ok := value.([]any); ok {
		for _, addition := range additions {
			if !containsPatchValue(values, addition) {
				values = append(values, addition)
			}
		}
		target[attribute] = values
	} else {
		if !containsPatchValue(values, value) {
			target[attribute] = append(values, value)
		}
	}
}

func containsPatchValue(values []any, candidate any) bool {
	for _, value := range values {
		if reflect.DeepEqual(value, candidate) {
			return true
		}
	}
	return false
}

func applyFilteredPatch(definition ResourceDefinition, target map[string]any, path patchPath, operation string, value any) error {
	raw, exists := target[path.attribute]
	values, ok := raw.([]any)
	if !exists || !ok {
		return clientError(400, "noTarget", "PATCH value-path target does not exist")
	}
	_, parent, ok := resolveAttributeContract(definition, nil, qualifiedPatchAttribute(path))
	if !ok || parent.Type != "complex" || !parent.MultiValued {
		return clientError(400, "invalidPath", "PATCH value-path parent is invalid")
	}
	expression, err := parseValueFilter(path.filter, parent.SubAttributes)
	if err != nil {
		return clientError(400, "invalidFilter", "PATCH value-path filter is unsupported")
	}
	matches := make([]int, 0)
	for index, rawItem := range values {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		matched, matchErr := matchFilter(expression, item)
		if matchErr != nil {
			return clientError(400, "invalidFilter", "PATCH value-path filter failed")
		}
		if matched {
			matches = append(matches, index)
		}
	}
	if len(matches) == 0 {
		if operation == "remove" {
			return nil
		}
		return clientError(400, "noTarget", "PATCH value-path matched no values")
	}
	if operation == "remove" && path.sub == "" {
		remove := make(map[int]struct{}, len(matches))
		for _, index := range matches {
			remove[index] = struct{}{}
		}
		kept := values[:0]
		for index, item := range values {
			if _, drop := remove[index]; !drop {
				kept = append(kept, item)
			}
		}
		target[path.attribute] = kept
		return nil
	}
	for _, index := range matches {
		item := values[index].(map[string]any)
		if path.sub == "" {
			object, ok := value.(map[string]any)
			if !ok {
				return clientError(400, "invalidValue", "PATCH filtered replacement must be an object")
			}
			for name, field := range object {
				item[name] = field
			}
			continue
		}
		if err := applyDirectPatch(item, path.sub, "", operation, value); err != nil {
			return err
		}
	}
	return nil
}

func qualifiedPatchAttribute(path patchPath) string {
	if path.extension != "" {
		return path.extension + ":" + path.attribute
	}
	return path.attribute
}

func ensureSchema(document Document, schema string) {
	values, _ := document["schemas"].([]any)
	for _, value := range values {
		if current, ok := value.(string); ok && strings.EqualFold(current, schema) {
			return
		}
	}
	document["schemas"] = append(values, schema)
}
