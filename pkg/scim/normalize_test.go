package scim

import (
	"reflect"
	"testing"
)

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
