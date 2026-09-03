package scim

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

const (
	MaximumResourceBytes = 1 << 20
	maximumJSONDepth     = 64
	maximumJSONNodes     = 10000
)

// Document is one decoded SCIM resource body. Callers must treat values
// returned by this package as immutable.
type Document map[string]any

// DecodeDocument strictly decodes one bounded JSON object. Unlike ordinary
// encoding/json map decoding, it rejects duplicate and case-equivalent names.
func DecodeDocument(raw []byte) (Document, error) {
	if len(raw) == 0 || len(raw) > MaximumResourceBytes || !utf8.Valid(raw) {
		return nil, fmt.Errorf("SCIM resource size or encoding is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	nodes := 0
	value, err := decodeJSONValue(decoder, 0, &nodes)
	if err != nil {
		return nil, fmt.Errorf("SCIM resource JSON is invalid: %w", err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("SCIM resource JSON has trailing data")
	}
	document, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("SCIM resource must be a JSON object")
	}
	return Document(document), nil
}

func decodeJSONValue(decoder *json.Decoder, depth int, nodes *int) (any, error) {
	if depth > maximumJSONDepth || *nodes >= maximumJSONNodes {
		return nil, fmt.Errorf("JSON structural boundary exceeded")
	}
	*nodes++
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delimiter, composite := token.(json.Delim)
	if !composite {
		return token, nil
	}
	switch delimiter {
	case '{':
		result := make(map[string]any)
		seen := make(map[string]string)
		canonical := CoreKeyCases()
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok || key == "" || !utf8.ValidString(key) {
				return nil, fmt.Errorf("JSON object name is invalid")
			}
			folded := strings.ToLower(key)
			if previous, exists := seen[folded]; exists {
				return nil, fmt.Errorf("attributes %q and %q are case-equivalent", previous, key)
			}
			admitted := key
			if known, exists := canonical[folded]; exists {
				admitted = known
			}
			value, err := decodeJSONValue(decoder, depth+1, nodes)
			if err != nil {
				return nil, err
			}
			seen[folded] = key
			result[admitted] = value
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return nil, fmt.Errorf("JSON object is not closed")
		}
		return result, nil
	case '[':
		result := make([]any, 0)
		for decoder.More() {
			value, err := decodeJSONValue(decoder, depth+1, nodes)
			if err != nil {
				return nil, err
			}
			result = append(result, value)
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return nil, fmt.Errorf("JSON array is not closed")
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unexpected JSON delimiter")
	}
}

func canonicalDocument(document Document) ([]byte, error) {
	encoded, err := json.Marshal(document)
	if err != nil || len(encoded) == 0 || len(encoded) > MaximumResourceBytes {
		return nil, fmt.Errorf("canonical SCIM resource is invalid")
	}
	return encoded, nil
}

func cloneDocument(document Document) (Document, error) {
	encoded, err := canonicalDocument(document)
	if err != nil {
		return nil, err
	}
	return DecodeDocument(encoded)
}
