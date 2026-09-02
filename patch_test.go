package scim

import (
	"strings"
	"testing"
)

func TestDecodePatch(t *testing.T) {
	raw := []byte(`{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"Replace","path":"active","value":false},{"op":"remove","path":"emails[type eq \"work\"]"}]}`)
	got, err := DecodePatch(raw, 10)
	if err != nil || len(got.Operations) != 2 || got.Operations[0].Op != "replace" {
		t.Fatalf("DecodePatch() = (%+v, %v)", got, err)
	}
	invalid := [][]byte{
		nil,
		[]byte(`{}`),
		[]byte(`{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[]}`),
		[]byte(`{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"remove"}]}`),
		[]byte(`{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"add","path":"name"}]}`),
		[]byte(`{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"merge","value":{}}]}`),
		[]byte(`{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"replace","path":"bad\\path","value":true}]}`),
	}
	for index, value := range invalid {
		if got, err := DecodePatch(value, 10); err == nil || len(got.Operations) != 0 {
			t.Errorf("invalid patch %d = (%+v, %v)", index, got, err)
		}
	}
	if _, err := DecodePatch([]byte(strings.Repeat(" ", MaximumPatchBytes+1)), 10); err == nil {
		t.Fatal("oversized patch passed")
	}
}
