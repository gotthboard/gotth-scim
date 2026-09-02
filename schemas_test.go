package scim

import (
	"reflect"
	"testing"
)

func TestValidateSchemasAndError(t *testing.T) {
	if err := ValidateSchemas([]string{UserSchema, "urn:example:ext"}, []string{"URN:IETF:PARAMS:SCIM:SCHEMAS:CORE:2.0:USER", "urn:example:ext"}); err != nil {
		t.Fatalf("ValidateSchemas() returned error: %v", err)
	}
	for _, actual := range [][]string{nil, {UserSchema, UserSchema}, {GroupSchema}} {
		if err := ValidateSchemas(actual, []string{UserSchema}); err == nil {
			t.Errorf("schemas %v passed", actual)
		}
	}
	got, err := NewError(409, "uniqueness", "resource already exists")
	if err != nil || !reflect.DeepEqual(got.Schemas, []string{ErrorSchema}) || got.Status != "409" {
		t.Fatalf("NewError() = (%+v, %v)", got, err)
	}
	if got, err := NewError(200, "", "bad"); err == nil || got.Status != "" || got.Schemas != nil {
		t.Fatalf("invalid NewError() = (%+v, %v)", got, err)
	}
}
