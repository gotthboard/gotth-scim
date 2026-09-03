package scim

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProtocolErrorAndSafeFailure(t *testing.T) {
	failure := &ProtocolError{Status: 400, SCIMType: "invalidValue", Detail: "bad input"}
	if failure.Error() != "SCIM 400 invalidValue: bad input" || (*ProtocolError)(nil).Error() != "<nil>" {
		t.Fatal("ProtocolError formatting changed")
	}
	for _, test := range []struct {
		err    error
		status int
	}{
		{failure, 400}, {ErrNotFound, 404}, {ErrConflict, 409}, {ErrTombstoned, 409}, {ErrPrecondition, 412}, {errors.New("private"), 500},
	} {
		status, _, _ := safeFailure(test.err)
		if status != test.status {
			t.Errorf("safeFailure(%v) = %d", test.err, status)
		}
	}
}

func TestRegistryRejectsInvalidDefinitions(t *testing.T) {
	validAttribute := SchemaAttribute{Name: "employeeNumber", Type: "string", Mutability: "readWrite", Returned: "default", Uniqueness: "none"}
	invalid := [][]ResourceDefinition{
		nil,
		{{Name: "1bad", Endpoint: "Things", Schema: "urn:things"}},
		{{Name: "Thing", Endpoint: "Things", Schema: ""}},
		{{Name: "Thing", Endpoint: "Things", Schema: "urn:things"}, {Name: "Thing", Endpoint: "Others", Schema: "urn:other"}},
		{{Name: "Thing", Endpoint: "Things", Schema: "urn:things", FilterAttributes: []string{"name", "NAME"}}},
		{{Name: "Thing", Endpoint: "Things", Schema: "urn:things", FilterAttributes: []string{"name"}, UniqueAttribute: "other"}},
		{{Name: "Thing", Endpoint: "Things", Schema: "urn:things", Extensions: []Extension{{Schema: "urn:things"}}}},
		{{Name: "Thing", Endpoint: "Things", Schema: "urn:things", Extensions: []Extension{{Schema: "urn:ext", Attributes: []SchemaAttribute{{Name: "bad", Type: "magic", Mutability: "readWrite", Returned: "default", Uniqueness: "none"}}}}}},
		{{Name: "Thing", Endpoint: "Things", Schema: "urn:things", Extensions: []Extension{{Schema: "urn:ext", Attributes: []SchemaAttribute{validAttribute, validAttribute}}}}},
	}
	tooMany := make([]ResourceDefinition, 33)
	for index := range tooMany {
		tooMany[index] = ResourceDefinition{Name: fmt.Sprintf("Thing%d", index), Endpoint: fmt.Sprintf("Things%d", index), Schema: fmt.Sprintf("urn:thing:%d", index)}
	}
	invalid = append(invalid, tooMany)
	for index, definitions := range invalid {
		if _, err := NewRegistry(definitions); err == nil {
			t.Errorf("invalid registry %d passed", index)
		}
	}
	valid := ResourceDefinition{Name: "Thing", Endpoint: "Things", Schema: "urn:things", Validate: func(Document, WriteMode) error { return nil }, Extensions: []Extension{{Schema: "urn:ext", Name: "Extension", Attributes: []SchemaAttribute{validAttribute}}}}
	registry, err := NewRegistry([]ResourceDefinition{valid})
	if err != nil || len(registry.definitions()) != 1 {
		t.Fatalf("valid custom registry = (%v, %v)", registry, err)
	}
	returned := registry.definitions()
	returned[0].Extensions[0].Attributes[0].Name = "mutated"
	again := registry.definitions()
	if again[0].Extensions[0].Attributes[0].Name == "mutated" {
		t.Fatal("registry exposed mutable discovery attributes")
	}
}

func TestSchemaAttributeValidationBranches(t *testing.T) {
	valid := SchemaAttribute{Name: "value", Type: "string", Mutability: "readWrite", Returned: "default", Uniqueness: "none"}
	tooMany := make([]SchemaAttribute, 257)
	for index := range tooMany {
		tooMany[index] = valid
		tooMany[index].Name = fmt.Sprintf("value%d", index)
	}
	invalid := [][]SchemaAttribute{
		tooMany,
		{{Name: "1bad", Type: "string", Mutability: "readWrite", Returned: "default", Uniqueness: "none"}},
		{valid, valid},
		{{Name: "value", Type: "magic", Mutability: "readWrite", Returned: "default", Uniqueness: "none"}},
		{{Name: "value", Type: "string", Mutability: "magic", Returned: "default", Uniqueness: "none"}},
		{{Name: "value", Type: "string", Mutability: "readWrite", Returned: "magic", Uniqueness: "none"}},
		{{Name: "value", Type: "string", Mutability: "readWrite", Returned: "default", Uniqueness: "magic"}},
		{{Name: "value", Type: "string", Mutability: "readWrite", Returned: "default", Uniqueness: "none", Description: "bad\ntext"}},
		{{Name: "value", Type: "string", Mutability: "readWrite", Returned: "default", Uniqueness: "none", SubAttributes: []SchemaAttribute{valid}}},
	}
	for index, attributes := range invalid {
		if err := validateSchemaAttributes(attributes, 0); err == nil {
			t.Errorf("invalid schema attributes %d passed", index)
		}
	}
	complex := SchemaAttribute{Name: "name", Type: "complex", Mutability: "readWrite", Returned: "default", Uniqueness: "none", SubAttributes: []SchemaAttribute{valid}}
	if err := validateSchemaAttributes([]SchemaAttribute{complex}, 0); err != nil {
		t.Fatalf("valid complex attribute: %v", err)
	}
	if err := validateSchemaAttributes([]SchemaAttribute{complex}, 5); err == nil {
		t.Fatal("excessive attribute depth passed")
	}
}

func TestResourceValidationFailurePaths(t *testing.T) {
	user := DefaultDefinitions()[0]
	invalid := []string{
		`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":true}`,
		`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":" member "}`,
		`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"member","active":"yes"}`,
		`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"member","name":"wrong"}`,
		`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"member","name":{"unknown":"x"}}`,
		`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"member","emails":"wrong"}`,
		`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"member","emails":["wrong"]}`,
		`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"member","emails":[{"primary":"wrong"}]}`,
		`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"member","emails":[{"unknown":"x"}]}`,
	}
	for index, raw := range invalid {
		document, err := DecodeDocument([]byte(raw))
		if err != nil {
			continue
		}
		if _, _, _, err := prepareResource(user, document, CreateMode, ""); err == nil {
			t.Errorf("invalid user %d passed", index)
		}
	}
	document, _ := DecodeDocument([]byte(`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"id":"right","userName":"member","displayName":null,"emails":[]}`))
	prepared, _, _, err := prepareResource(user, document, ReplaceMode, "right")
	if err != nil || prepared["displayName"] != nil || prepared["emails"] != nil {
		t.Fatalf("unassigned normalization = (%#v, %v)", prepared, err)
	}
	if _, _, _, err := prepareResource(user, document, WriteMode(99), "right"); err == nil {
		t.Fatal("invalid write mode passed")
	}
}

func TestMemoryStoreFailurePaths(t *testing.T) {
	var nilStore *MemoryStore
	if err := nilStore.Transact(context.Background(), func(Transaction) error { return nil }); err == nil {
		t.Fatal("nil store passed")
	}
	store := NewMemoryStore()
	if err := store.Transact(context.Background(), nil); err == nil {
		t.Fatal("nil callback passed")
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.Transact(canceled, func(Transaction) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled transaction = %v", err)
	}
	if err := store.Transact(context.Background(), func(transaction Transaction) error {
		if _, err := transaction.List(Query{}); err == nil {
			return errors.New("invalid query passed")
		}
		if _, err := transaction.Tombstones("", "User"); err == nil {
			return errors.New("invalid tombstone query passed")
		}
		if err := transaction.Create(Record{}); err == nil {
			return errors.New("invalid record passed")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	record := conformanceRecord("scope", "one", "", "One", testTime)
	tamperedIndex := cloneRecord(record)
	tamperedIndex.Indexes[0].Value = "Other"
	if err := store.Transact(context.Background(), func(transaction Transaction) error { return transaction.Create(tamperedIndex) }); err == nil {
		t.Fatal("index inconsistent with canonical data passed")
	}
	if err := store.Transact(context.Background(), func(transaction Transaction) error { return transaction.Create(record) }); err != nil {
		t.Fatal(err)
	}
	inconsistent := conformanceRecord("scope", "two", "", "Two", testTime)
	inconsistent.Indexes[0].CaseExact = true
	if err := store.Transact(context.Background(), func(transaction Transaction) error { return transaction.Create(inconsistent) }); !errors.Is(err, ErrConflict) {
		t.Fatalf("inconsistent index contract = %v", err)
	}
	if err := store.Transact(context.Background(), func(transaction Transaction) error {
		if err := transaction.Replace(record, record.Version); err != nil {
			return err
		}
		wrongCreated := cloneRecord(record)
		wrongCreated.Created = wrongCreated.Created.Add(time.Second)
		if err := transaction.Replace(wrongCreated, record.Version); err == nil {
			return errors.New("creation time mutation passed")
		}
		if err := transaction.Delete("scope", "User", "one", `"wrong"`, Tombstone{}); !errors.Is(err, ErrPrecondition) {
			return errors.New("delete precondition passed")
		}
		if err := transaction.Delete("scope", "User", "one", record.Version, Tombstone{}); err == nil {
			return errors.New("invalid tombstone passed")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPatchAdditionalOperations(t *testing.T) {
	definition := DefaultDefinitions()[0]
	base, _ := DecodeDocument([]byte(`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"member","name":{"givenName":"One"},"emails":[{"type":"work","value":"one@example.com"}]}`))
	cases := []string{
		`{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"add","value":{"displayName":"Member","roles":[{"value":"admin"}]}}]}`,
		`{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"add","path":"emails","value":{"type":"home","value":"home@example.com"}}]}`,
		`{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"replace","path":"name","value":{"givenName":"Two"}}]}`,
		`{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"remove","path":"name.givenName"}]}`,
		`{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"replace","path":"emails[type eq \"work\"]","value":{"display":"Work"}}]}`,
	}
	for index, raw := range cases {
		request, err := DecodePatch([]byte(raw), 10)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := ApplyPatch(definition, base, request, "id"); err != nil {
			t.Errorf("PATCH case %d: %v", index, err)
		}
	}
	invalid := []string{
		`{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"add","value":"wrong"}]}`,
		`{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"replace","path":"missing.sub","value":"x"}]}`,
		`{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"replace","path":"emails[type eq \"none\"].value","value":"x"}]}`,
		`{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"replace","path":"emails[type eq \"work\"]","value":"x"}]}`,
		`{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"remove","path":"missing"}]}`,
	}
	for index, raw := range invalid {
		request, err := DecodePatch([]byte(raw), 10)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := ApplyPatch(definition, base, request, "id"); err == nil {
			t.Errorf("invalid PATCH %d passed", index)
		}
	}
}

func TestAddValueShapes(t *testing.T) {
	target := map[string]any{"scalar": "before", "array": []any{"one"}}
	addValue(target, "new", "value")
	addValue(target, "scalar", "after")
	addValue(target, "array", []any{"two", "three"})
	addValue(target, "array", "four")
	if target["new"] != "value" || target["scalar"] != "after" || len(target["array"].([]any)) != 4 {
		t.Fatalf("addValue = %#v", target)
	}
}

func TestServerReplacePaginationAndErrors(t *testing.T) {
	server := newTestServer(t)
	created := requestServer(t, server, http.MethodPost, "/scim/v2/Users", `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"first"}`, "tenant", nil)
	resource := decodeResponse(t, created)
	id := resource["id"].(string)
	version := created.Header().Get("ETag")
	replaced := requestServer(t, server, http.MethodPut, "/scim/v2/Users/"+id, `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"id":"`+id+`","userName":"second"}`, "tenant", map[string]string{"If-Match": version})
	if replaced.Code != http.StatusOK || replaced.Header().Get("ETag") == version {
		t.Fatalf("replace = %d %s", replaced.Code, replaced.Body.String())
	}
	got := requestServer(t, server, http.MethodGet, "/scim/v2/Users/"+id, "", "tenant", nil)
	if got.Code != http.StatusOK || decodeResponse(t, got)["userName"] != "second" {
		t.Fatalf("GET replaced = %d %s", got.Code, got.Body.String())
	}
	for _, name := range []string{"third", "fourth"} {
		response := requestServer(t, server, http.MethodPost, "/scim/v2/Users", `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"`+name+`"}`, "tenant", nil)
		if response.Code != http.StatusCreated {
			t.Fatal(response.Body.String())
		}
	}
	page := requestServer(t, server, http.MethodGet, "/scim/v2/Users?startIndex=2&count=99", "", "tenant", nil)
	decoded := decodeResponse(t, page)
	if len(decoded["Resources"].([]any)) != 2 || decoded["totalResults"] != json.Number("3") {
		t.Fatalf("page = %#v", decoded)
	}
	for _, path := range []string{
		`/scim/v2/Users?count=-1`,
		`/scim/v2/Users?count=bad`,
	} {
		if response := requestServer(t, server, http.MethodGet, path, "", "tenant", nil); response.Code != http.StatusBadRequest {
			t.Errorf("GET %s = %d", path, response.Code)
		}
	}
	if response := requestServer(t, server, http.MethodPost, "/scim/v2/Users?x=1", `{}`, "tenant", nil); response.Code != http.StatusBadRequest {
		t.Fatalf("POST query = %d", response.Code)
	}
	if response := requestServer(t, server, http.MethodDelete, "/scim/v2/Users/"+id+"?x=1", "", "tenant", nil); response.Code != http.StatusBadRequest {
		t.Fatalf("resource query = %d", response.Code)
	}
}

func TestServerBulkMutationMethodsAndFailures(t *testing.T) {
	server := newTestServer(t)
	created := requestServer(t, server, http.MethodPost, "/scim/v2/Users", `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"member"}`, "tenant", nil)
	user := decodeResponse(t, created)
	id := user["id"].(string)
	version := created.Header().Get("ETag")
	bulk := fmt.Sprintf(`{"schemas":["%s"],"Operations":[{"method":"PUT","path":"/Users/%s","version":%q,"data":{"schemas":["%s"],"id":"%s","userName":"renamed"}},{"method":"PATCH","path":"/Users/%s","data":{"schemas":["%s"],"Operations":[{"op":"add","path":"active","value":true}]}},{"method":"DELETE","path":"/Users/%s"}]}`, BulkRequestSchema, id, version, UserSchema, id, id, PatchSchema, id)
	response := requestServer(t, server, http.MethodPost, "/scim/v2/Bulk", bulk, "tenant", nil)
	operations := decodeResponse(t, response)["Operations"].([]any)
	if len(operations) != 3 || operations[0].(map[string]any)["status"] != "200" || operations[2].(map[string]any)["status"] != "204" {
		t.Fatalf("Bulk mutations = %#v", operations)
	}
	bad := `{"schemas":["` + BulkRequestSchema + `"],"Operations":[{"method":"POST","bulkId":"x","path":"/Users","data":{"schemas":["` + UserSchema + `"],"userName":"bulkId:missing"}}]}`
	response = requestServer(t, server, http.MethodPost, "/scim/v2/Bulk", bad, "tenant", nil)
	operations = decodeResponse(t, response)["Operations"].([]any)
	if operations[0].(map[string]any)["status"] != "400" {
		t.Fatalf("unresolved Bulk data = %#v", operations)
	}
}

func TestReadBodyAndErrorWriters(t *testing.T) {
	request, _ := http.NewRequest(http.MethodPost, "https://example.test", strings.NewReader(strings.Repeat("x", MaximumResourceBytes+1)))
	request.Header.Set("Content-Type", "application/json")
	if _, err := readBody(request); err == nil {
		t.Fatal("oversized body passed")
	}
	if response, err := NewError(500, "", "safe"); err != nil || response.Status != "500" {
		t.Fatal("valid error response failed")
	}
	if _, err := canonicalDocument(Document{"bad": make(chan int)}); err == nil {
		t.Fatal("unencodable document passed")
	}
}

func TestRecordVersionAndReplacement(t *testing.T) {
	document, _ := DecodeDocument([]byte(`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"member"}`))
	record, err := newRecord("scope", "manager", "User", "id", "", document, []IndexKey{{Name: "userName", Value: "member", Unique: true}}, testTime)
	if err != nil {
		t.Fatal(err)
	}
	same, changed, err := replacementRecord(record, "", document, record.Indexes, testTime)
	if err != nil || changed || same.Version != record.Version {
		t.Fatalf("same replacement = (%+v, %v, %v)", same, changed, err)
	}
	document["active"] = true
	updated, changed, err := replacementRecord(record, "", document, record.Indexes, testTime.Add(-time.Second))
	if err != nil || !changed || !updated.LastModified.After(record.LastModified) || updated.Version == record.Version {
		t.Fatalf("changed replacement = (%+v, %v, %v)", updated, changed, err)
	}
}

func TestCheckStoreRejectsInvalidFactory(t *testing.T) {
	if err := CheckStore(context.Background(), nil); err == nil {
		t.Fatal("nil factory passed")
	}
	if err := CheckStore(context.Background(), func() Store { return nil }); err == nil {
		t.Fatal("nil store passed")
	}
}

func TestDecodePatchAndBulkRejectDuplicateKeys(t *testing.T) {
	patch := []byte(`{"schemas":["` + PatchSchema + `"],"Operations":[],"Operations":[]}`)
	if _, err := DecodePatch(patch, 10); err == nil {
		t.Fatal("duplicate PATCH key passed")
	}
	bulk := []byte(`{"schemas":["` + BulkRequestSchema + `"],"Operations":[],"operations":[]}`)
	if _, err := DecodeBulk(bulk); err == nil {
		t.Fatal("case-equivalent Bulk key passed")
	}
}

func TestResolveBulkValue(t *testing.T) {
	references := map[string]bulkReference{"one": {id: "id-one", location: "https://example/Users/id-one"}}
	value := map[string]any{"value": "bulkId:one", "$ref": "bulkId:one", "nested": []any{"bulkId:one", true}}
	resolved, err := resolveBulkValue(value, "", references)
	if err != nil {
		t.Fatal(err)
	}
	object := resolved.(map[string]any)
	if object["value"] != "id-one" || object["$ref"] != "https://example/Users/id-one" || object["nested"].([]any)[0] != "id-one" {
		t.Fatalf("resolved = %#v", object)
	}
	if _, err := resolveBulkValue("bulkId:missing", "", references); err == nil {
		t.Fatal("unknown bulk reference passed")
	}
}

func TestWriteJSONEncodingFailure(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeJSON(recorder, 200, map[string]any{"bad": make(chan int)})
	if recorder.Code != 500 || !bytes.Contains(recorder.Body.Bytes(), []byte("encoding failed")) {
		t.Fatalf("writeJSON failure = %d %s", recorder.Code, recorder.Body.String())
	}
}
