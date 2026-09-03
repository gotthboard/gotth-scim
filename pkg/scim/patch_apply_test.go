package scim

import "testing"

func TestApplyPatch(t *testing.T) {
	definition := DefaultDefinitions()[0]
	current, err := DecodeDocument([]byte(`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"member","active":true,"name":{"givenName":"Before"},"emails":[{"type":"work","value":"old@example.com"},{"type":"home","value":"home@example.com"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	request, err := DecodePatch([]byte(`{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"replace","path":"name.givenName","value":"After"},{"op":"replace","path":"emails[type eq \"work\"].value","value":"new@example.com"},{"op":"remove","path":"emails[type eq \"home\"]"},{"op":"add","path":"displayName","value":"Member"}]}`), 10)
	if err != nil {
		t.Fatal(err)
	}
	patched, indexes, _, err := ApplyPatch(definition, current, request, "id")
	if err != nil {
		t.Fatal(err)
	}
	if patched["name"].(map[string]any)["givenName"] != "After" || patched["emails"].([]any)[0].(map[string]any)["value"] != "new@example.com" || len(patched["emails"].([]any)) != 1 || patched["displayName"] != "Member" || len(indexes) != 1 {
		t.Fatalf("patched = %#v", patched)
	}

	missing, _ := DecodePatch([]byte(`{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"remove","path":"title"}]}`), 10)
	if _, _, _, err := ApplyPatch(definition, current, missing, "id"); err == nil {
		t.Fatal("missing PATCH target passed")
	}
}
