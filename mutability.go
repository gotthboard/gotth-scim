package scim

import (
	"fmt"
	"reflect"
)

func stripReadOnly(definition ResourceDefinition, document Document) {
	stripReadOnlyAttributes(map[string]any(document), definition.Attributes, nil)
	for _, extension := range definition.Extensions {
		if object, ok := document[extension.Schema].(map[string]any); ok {
			stripReadOnlyAttributes(object, extension.Attributes, nil)
		}
	}
}

func enforceReplacementMutability(definition ResourceDefinition, current, incoming Document) error {
	if err := enforceAttributeMutability(map[string]any(current), map[string]any(incoming), definition.Attributes); err != nil {
		return err
	}
	for _, extension := range definition.Extensions {
		before, _ := current[extension.Schema].(map[string]any)
		after, _ := incoming[extension.Schema].(map[string]any)
		if after != nil {
			if err := enforceAttributeMutability(before, after, extension.Attributes); err != nil {
				return err
			}
		}
	}
	return nil
}

func enforceAttributeMutability(current, incoming map[string]any, attributes []SchemaAttribute) error {
	for _, attribute := range attributes {
		before, hadBefore := current[attribute.Name]
		after, hadAfter := incoming[attribute.Name]
		switch attribute.Mutability {
		case "readOnly":
			if hadBefore {
				incoming[attribute.Name] = before
			} else {
				delete(incoming, attribute.Name)
			}
			continue
		case "immutable":
			if hadBefore && hadAfter && !reflect.DeepEqual(before, after) {
				return clientError(400, "mutability", fmt.Sprintf("attribute %s is immutable", attribute.Name))
			}
			if hadBefore && !hadAfter {
				incoming[attribute.Name] = before
			}
		}
		if attribute.Type == "complex" && !attribute.MultiValued {
			beforeObject, _ := before.(map[string]any)
			afterObject, _ := after.(map[string]any)
			if afterObject != nil {
				if err := enforceAttributeMutability(beforeObject, afterObject, attribute.SubAttributes); err != nil {
					return err
				}
			}
		} else if attribute.Type == "complex" && attribute.MultiValued {
			beforeValues, _ := before.([]any)
			afterValues, _ := after.([]any)
			if err := enforceMultiValueMutability(beforeValues, afterValues, attribute.SubAttributes); err != nil {
				return err
			}
		}
	}
	return nil
}

func stripReadOnlyAttributes(object map[string]any, attributes []SchemaAttribute, current map[string]any) {
	for _, attribute := range attributes {
		if attribute.Mutability == "readOnly" {
			if current != nil {
				if value, ok := current[attribute.Name]; ok {
					object[attribute.Name] = value
					continue
				}
			}
			delete(object, attribute.Name)
			continue
		}
		if attribute.Type == "complex" {
			switch nested := object[attribute.Name].(type) {
			case map[string]any:
				stripReadOnlyAttributes(nested, attribute.SubAttributes, nil)
			case []any:
				for _, raw := range nested {
					if child, ok := raw.(map[string]any); ok {
						stripReadOnlyAttributes(child, attribute.SubAttributes, nil)
					}
				}
			}
		}
	}
}

func enforceMultiValueMutability(before, after []any, attributes []SchemaAttribute) error {
	byValue := make(map[string]map[string]any)
	for _, raw := range before {
		if object, ok := raw.(map[string]any); ok {
			if value, ok := object["value"].(string); ok {
				byValue[value] = object
			}
		}
	}
	for index, raw := range after {
		object, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		var previous map[string]any
		if value, ok := object["value"].(string); ok {
			previous = byValue[value]
		}
		if previous == nil && index < len(before) {
			previous, _ = before[index].(map[string]any)
		}
		if previous != nil {
			if err := enforceAttributeMutability(previous, object, attributes); err != nil {
				return err
			}
		}
	}
	return nil
}
