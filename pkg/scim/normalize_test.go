package scim

import (
	"reflect"
	"testing"
)

func TestSearchKeysDoNotRewriteResourceExtensions(t *testing.T) {
	document, err := DecodeDocument([]byte(`{"schemas":["urn:base","urn:example"],"urn:example":{"Count":1,"SortBy":"name","SortOrder":"descending"}}`))
	if err != nil {
		t.Fatal(err)
	}
	extension := document["urn:example"].(map[string]any)
	for _, key := range []string{"Count", "SortBy", "SortOrder"} {
		if _, exists := extension[key]; !exists {
			t.Fatalf("custom extension key %q was rewritten: %#v", key, extension)
		}
	}
	attributes := []SchemaAttribute{
		{Name: "Count", Type: "integer", Mutability: "readWrite", Returned: "default", Uniqueness: "none"},
		{Name: "SortBy", Type: "string", Mutability: "readWrite", Returned: "default", Uniqueness: "none"},
		{Name: "SortOrder", Type: "string", Mutability: "readWrite", Returned: "default", Uniqueness: "none"},
	}
	definition := ResourceDefinition{Name: "Thing", Endpoint: "Things", Schema: "urn:base", Extensions: []Extension{{Schema: "urn:example", Name: "Example", Attributes: attributes}}}
	if _, _, _, err := prepareResource(definition, document, CreateMode, ""); err != nil {
		t.Fatalf("registered extension search-like keys failed: %v", err)
	}
}

func TestNormalizeKeys(t *testing.T) {
	input := map[string]any{"USERNAME": "member", "emails": []any{map[string]any{"VALUE": "m@example.com"}}}
	got, err := NormalizeKeys(input, CoreKeyCases())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"userName": "member", "emails": []any{map[string]any{"value": "m@example.com"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeKeys() = %#v", got)
	}
	if _, err := NormalizeKeys(map[string]any{"userName": "one", "USERNAME": "two"}, CoreKeyCases()); err == nil {
		t.Fatal("case-equivalent collision passed")
	}
}
