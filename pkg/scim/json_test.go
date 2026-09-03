package scim

import (
	"strings"
	"testing"
)

func TestDecodeDocumentStrictness(t *testing.T) {
	document, err := DecodeDocument([]byte(`{"USERNAME":"member","emails":[{"VALUE":"m@example.com"}]}`))
	if err != nil || document["userName"] != "member" {
		t.Fatalf("DecodeDocument() = (%#v, %v)", document, err)
	}
	invalid := [][]byte{
		nil,
		[]byte(`[]`),
		[]byte(`{"a":1,"a":2}`),
		[]byte(`{"userName":"a","USERNAME":"b"}`),
		[]byte("{\"x\":\"\xff\"}"),
		[]byte(`{} {}`),
	}
	for index, raw := range invalid {
		if got, err := DecodeDocument(raw); err == nil || got != nil {
			t.Errorf("invalid document %d = (%#v, %v)", index, got, err)
		}
	}
	deep := strings.Repeat(`{"x":`, maximumJSONDepth+2) + `null` + strings.Repeat(`}`, maximumJSONDepth+2)
	if _, err := DecodeDocument([]byte(deep)); err == nil {
		t.Fatal("excessive JSON depth passed")
	}
}

func TestCloneDocumentIsIndependent(t *testing.T) {
	original := Document{"value": []any{map[string]any{"nested": "before"}}}
	clone, err := cloneDocument(original)
	if err != nil {
		t.Fatal(err)
	}
	clone["value"].([]any)[0].(map[string]any)["nested"] = "after"
	if original["value"].([]any)[0].(map[string]any)["nested"] != "before" {
		t.Fatal("clone mutated original")
	}
}
