package scim

import "testing"

func TestParseBulkPath(t *testing.T) {
	collection, id, reference, err := ParseBulkPath("/Users/member%40example.com")
	if err != nil || collection != "Users" || id != "member@example.com" || reference != "" {
		t.Fatalf("ParseBulkPath(resource) = (%q, %q, %q, %v)", collection, id, reference, err)
	}
	collection, id, reference, err = ParseBulkPath("Groups/bulkId:created-group")
	if err != nil || collection != "Groups" || id != "" || reference != "created-group" {
		t.Fatalf("ParseBulkPath(reference) = (%q, %q, %q, %v)", collection, id, reference, err)
	}
	collection, id, reference, err = ParseBulkPath("Devices/device-1")
	if err != nil || collection != "Devices" || id != "device-1" || reference != "" {
		t.Fatalf("ParseBulkPath(custom) = (%q, %q, %q, %v)", collection, id, reference, err)
	}
	custom := []byte(`{"schemas":["` + BulkRequestSchema + `"],"Operations":[{"method":"POST","bulkId":"device","path":"/Devices","data":{}}]}`)
	if decoded, err := DecodeBulk(custom); err != nil || decoded.Operations[0].Path != "/Devices" {
		t.Fatalf("DecodeBulk(custom) = (%+v, %v)", decoded, err)
	}
	for _, raw := range []string{"", "https://example/Users", "1Devices", "Users/", "Users/a/b", "Users/a%2Fb", "Groups/bulkId:"} {
		if _, _, _, err := ParseBulkPath(raw); err == nil {
			t.Errorf("invalid path %q passed", raw)
		}
	}
}

func TestDecodeBulk(t *testing.T) {
	raw := []byte(`{"schemas":["urn:ietf:params:scim:api:messages:2.0:BulkRequest"],"failOnErrors":1,"Operations":[{"method":"post","bulkId":"new-user","path":"/Users","data":{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"member@example.com"}},{"method":"PATCH","path":"/Users/bulkId:new-user","data":{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"replace","path":"active","value":true}]}},{"method":"DELETE","path":"/Groups/group-one"}]}`)
	got, err := DecodeBulk(raw)
	if err != nil || len(got.Operations) != 3 || got.Operations[0].Method != "POST" {
		t.Fatalf("DecodeBulk() = (%+v, %v)", got, err)
	}
	invalid := [][]byte{
		nil,
		[]byte(`{}`),
		[]byte(`{"schemas":["urn:ietf:params:scim:api:messages:2.0:BulkRequest"],"Operations":[]}`),
		[]byte(`{"schemas":["urn:ietf:params:scim:api:messages:2.0:BulkRequest"],"Operations":[{"method":"GET","path":"/Users"}]}`),
		[]byte(`{"schemas":["urn:ietf:params:scim:api:messages:2.0:BulkRequest"],"Operations":[{"method":"POST","bulkId":"x","path":"/Users","data":"not-an-object"}]}`),
		[]byte(`{"schemas":["urn:ietf:params:scim:api:messages:2.0:BulkRequest"],"Operations":[{"method":"POST","bulkId":"x","path":"/Users","data":{}},{"method":"POST","bulkId":"x","path":"/Groups","data":{}}]}`),
		[]byte(`{"schemas":["urn:ietf:params:scim:api:messages:2.0:BulkRequest"],"Operations":[{"method":"DELETE","path":"/Users/member","data":{}}]}`),
		[]byte(`{"schemas":["urn:ietf:params:scim:api:messages:2.0:BulkRequest"],"Operations":[{"method":"PUT","path":"/Users/member"}]}`),
		[]byte(`{"schemas":["urn:ietf:params:scim:api:messages:2.0:BulkRequest"],"Operations":[]} trailing`),
	}
	for index, value := range invalid {
		if got, err := DecodeBulk(value); err == nil || len(got.Operations) != 0 {
			t.Errorf("invalid bulk %d = (%+v, %v)", index, got, err)
		}
	}
}
