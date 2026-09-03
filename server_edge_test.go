package scim

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type failingStore struct{ err error }

func (store failingStore) Transact(context.Context, func(Transaction) error) error { return store.err }

func TestServerMethodAndProtocolFailures(t *testing.T) {
	server := newTestServer(t)
	for _, test := range []struct {
		method string
		path   string
		body   string
		status int
	}{
		{http.MethodPut, "/scim/v2/Users", `{}`, 405},
		{http.MethodPost, "/scim/v2/Users/missing", `{}`, 405},
		{http.MethodPost, "/scim/v2/Bulk?x=1", `{}`, 400},
		{http.MethodGet, "/scim/v2/Bulk", "", 405},
		{http.MethodPost, "/scim/v2/Bulk", `{}`, 400},
		{http.MethodGet, "/scim/v2/ResourceTypes/Missing", "", 404},
		{http.MethodGet, "/scim/v2/Schemas/missing", "", 404},
		{http.MethodGet, "/scim/v2//Users", "", 404},
		{http.MethodGet, "/scim/v2/Users/bulkId", "", 404},
		{http.MethodGet, "/scim/v2/Users/id/extra", "", 404},
		{http.MethodGet, "/scim/v2/Devices", "", 404},
	} {
		response := requestServer(t, server, test.method, test.path, test.body, "tenant", nil)
		if response.Code != test.status {
			t.Errorf("%s %s = %d, want %d: %s", test.method, test.path, response.Code, test.status, response.Body.String())
		}
	}
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, nil)
	if recorder.Code != 500 {
		t.Fatalf("nil request = %d", recorder.Code)
	}
}

func TestDiscoveryRejectsMethodsAndQueries(t *testing.T) {
	server := newTestServer(t)
	for _, path := range []string{"/scim/v2/ServiceProviderConfig", "/scim/v2/ResourceTypes", "/scim/v2/Schemas"} {
		if response := requestServer(t, server, http.MethodPost, path, `{}`, "tenant", nil); response.Code != 405 {
			t.Errorf("POST %s = %d", path, response.Code)
		}
		if response := requestServer(t, server, http.MethodGet, path+"?x=1", "", "tenant", nil); response.Code != 400 {
			t.Errorf("GET %s?x=1 = %d", path, response.Code)
		}
	}
}

func TestServerPatchDeleteAndStorageFailures(t *testing.T) {
	server := newTestServer(t)
	created := requestServer(t, server, http.MethodPost, "/scim/v2/Users", `{"schemas":["`+UserSchema+`"],"userName":"member"}`, "tenant", nil)
	id := decodeResponse(t, created)["id"].(string)
	version := created.Header().Get("ETag")
	badPatch := requestServer(t, server, http.MethodPatch, "/scim/v2/Users/"+id, `{}`, "tenant", nil)
	if badPatch.Code != 400 {
		t.Fatalf("bad PATCH = %d", badPatch.Code)
	}
	badMedia := requestServer(t, server, http.MethodPatch, "/scim/v2/Users/"+id, `{}`, "tenant", map[string]string{"Content-Type": "text/plain"})
	if badMedia.Code != 415 {
		t.Fatalf("PATCH media type = %d", badMedia.Code)
	}
	wrongDelete := requestServer(t, server, http.MethodDelete, "/scim/v2/Users/"+id, "", "tenant", map[string]string{"If-Match": `"wrong"`})
	if wrongDelete.Code != 412 {
		t.Fatalf("wrong DELETE = %d", wrongDelete.Code)
	}
	malformed := requestServer(t, server, http.MethodGet, "/scim/v2/Users/"+id, "", "tenant", map[string]string{"If-None-Match": "bad"})
	if malformed.Code != 400 {
		t.Fatalf("malformed conditional GET = %d", malformed.Code)
	}
	noOp := requestServer(t, server, http.MethodPut, "/scim/v2/Users/"+id, `{"schemas":["`+UserSchema+`"],"id":"`+id+`","userName":"member"}`, "tenant", map[string]string{"If-Match": version})
	if noOp.Code != 200 || noOp.Header().Get("ETag") != version {
		t.Fatalf("no-op PUT = %d %v", noOp.Code, noOp.Header())
	}

	broken, err := NewServer(ServerConfig{
		Store: failingStore{err: errors.New("private database failure")}, ExternalURL: "https://scim.example.test/scim/v2",
		ResolveScope:          func(*http.Request) (string, error) { return "tenant", nil },
		AuthenticationSchemes: []AuthenticationScheme{{Type: "bearer", Name: "Bearer", Description: "Bearer"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	failure := requestServer(t, broken, http.MethodGet, "/scim/v2/Users/missing", "", "tenant", nil)
	if failure.Code != 500 || strings.Contains(failure.Body.String(), "database") {
		t.Fatalf("storage failure leaked = %d %s", failure.Code, failure.Body.String())
	}
	failure = requestServer(t, broken, http.MethodGet, "/scim/v2/Users", "", "tenant", nil)
	if failure.Code != 500 {
		t.Fatalf("list storage failure = %d", failure.Code)
	}
}

func TestNewServerAuthenticationAndLimitValidation(t *testing.T) {
	base := ServerConfig{Store: NewMemoryStore(), ExternalURL: "https://example.test/scim/v2", ResolveScope: func(*http.Request) (string, error) { return "scope", nil }}
	invalidSchemes := [][]AuthenticationScheme{
		nil,
		{{Type: "1bad", Name: "Bearer", Description: "Bearer"}},
		{{Type: "bearer", Name: "", Description: "Bearer"}},
		{{Type: "bearer", Name: "Bearer", Description: ""}},
		{{Type: "bearer", Name: "Bearer", Description: "Bearer", SpecURI: "http://example.test/spec"}},
	}
	for index, schemes := range invalidSchemes {
		config := base
		config.AuthenticationSchemes = schemes
		if _, err := NewServer(config); err == nil {
			t.Errorf("invalid authentication schemes %d passed", index)
		}
	}
	base.AuthenticationSchemes = []AuthenticationScheme{{Type: "bearer", Name: "Bearer", Description: "Bearer"}}
	for _, limits := range [][2]int{{-1, 0}, {10001, 0}, {1, -1}, {1, 1001}} {
		config := base
		config.MaximumPageSize, config.MaximumPatchOperations = limits[0], limits[1]
		if _, err := NewServer(config); err == nil {
			t.Errorf("invalid limits %v passed", limits)
		}
	}
}

func TestServerRejectsUnsafeScopeAndEncodedBasePath(t *testing.T) {
	server := newTestServer(t)
	response := requestServer(t, server, http.MethodGet, "/scim/v2/Users", "", "tenant\nother", nil)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unsafe scope = %d", response.Code)
	}

	_, err := NewServer(ServerConfig{
		Store: NewMemoryStore(), ExternalURL: "https://example.test/scim/é",
		ResolveScope:          func(*http.Request) (string, error) { return "tenant", nil },
		AuthenticationSchemes: []AuthenticationScheme{{Type: "bearer", Name: "Bearer", Description: "Bearer"}},
	})
	if err == nil {
		t.Fatal("encoded external base path passed")
	}
}

func TestServerEntropyAndClockFailures(t *testing.T) {
	config := ServerConfig{
		Store: NewMemoryStore(), ExternalURL: "https://example.test/scim/v2", ResolveScope: func(*http.Request) (string, error) { return "scope", nil },
		AuthenticationSchemes: []AuthenticationScheme{{Type: "bearer", Name: "Bearer", Description: "Bearer"}}, Entropy: bytes.NewReader(nil),
	}
	server, err := NewServer(config)
	if err != nil {
		t.Fatal(err)
	}
	response := requestServer(t, server, http.MethodPost, "/scim/v2/Users", `{"schemas":["`+UserSchema+`"],"userName":"member"}`, "scope", nil)
	if response.Code != 500 {
		t.Fatalf("entropy failure = %d", response.Code)
	}
	config.Entropy = bytes.NewReader(make([]byte, 32))
	config.Clock = func() time.Time { return time.Time{} }
	server, _ = NewServer(config)
	response = requestServer(t, server, http.MethodPost, "/scim/v2/Users", `{"schemas":["`+UserSchema+`"],"userName":"member"}`, "scope", nil)
	if response.Code != 500 {
		t.Fatalf("clock failure = %d", response.Code)
	}
}

func TestBulkPathReferenceAndInvalidCollection(t *testing.T) {
	server := newTestServer(t)
	request := `{"schemas":["` + BulkRequestSchema + `"],"Operations":[` +
		`{"method":"POST","bulkId":"u","path":"/Users","data":{"schemas":["` + UserSchema + `"],"userName":"member"}},` +
		`{"method":"PATCH","path":"/Users/bulkId:u","data":{"schemas":["` + PatchSchema + `"],"Operations":[{"op":"add","path":"active","value":true}]}}]}`
	response := requestServer(t, server, http.MethodPost, "/scim/v2/Bulk", request, "tenant", nil)
	operations := decodeResponse(t, response)["Operations"].([]any)
	if len(operations) != 2 || operations[1].(map[string]any)["status"] != "200" {
		t.Fatalf("Bulk path reference = %#v", operations)
	}

	op := BulkOperation{Method: http.MethodPost, BulkID: "x", Path: "/Devices", Data: json.RawMessage(`{}`)}
	if _, _, err := server.executeBulkOperation(httptest.NewRequest(http.MethodPost, "/", nil), "tenant", op, nil); err == nil {
		t.Fatal("unknown Bulk collection passed")
	}
}

func TestServerAdditionalInputFailures(t *testing.T) {
	server := newTestServer(t)
	for _, path := range []string{"/scim/v2/Users?startIndex=bad", "/scim/v2/Users?startIndex=-10", "/scim/v2/Users?startIndex=99", "/scim/v2/Users?count=0"} {
		response := requestServer(t, server, http.MethodGet, path, "", "tenant", nil)
		if strings.Contains(path, "bad") {
			if response.Code != 400 {
				t.Errorf("GET %s = %d", path, response.Code)
			}
		} else if response.Code != 200 {
			t.Errorf("GET %s = %d", path, response.Code)
		}
	}
	for index, test := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/scim/v2/Users", `{`},
		{http.MethodPost, "/scim/v2/Users", `{"schemas":["` + UserSchema + `"]}`},
		{http.MethodPut, "/scim/v2/Users/missing", `{`},
		{http.MethodPut, "/scim/v2/Users/missing", `{"schemas":["` + UserSchema + `"],"id":"wrong","userName":"member"}`},
	} {
		response := requestServer(t, server, test.method, test.path, test.body, "tenant", nil)
		if response.Code != 400 {
			t.Errorf("invalid input %d = %d %s", index, response.Code, response.Body.String())
		}
	}
	if response := requestServer(t, server, http.MethodDelete, "/scim/v2/Users/missing", "", "tenant", nil); response.Code != 404 {
		t.Fatalf("delete missing = %d", response.Code)
	}
}

func TestWritePreconditionFailures(t *testing.T) {
	record := conformanceRecord("scope", "id", "", "member", testTime)
	for _, headers := range [][2]string{{"bad", ""}, {`"wrong"`, ""}, {"", "bad"}, {"", record.Version}} {
		if err := evaluateWritePreconditions(headers[0], headers[1], record); err == nil {
			t.Errorf("preconditions %q %q passed", headers[0], headers[1])
		}
	}
}

func TestExtensionPatchAndDiscovery(t *testing.T) {
	definition := DefaultDefinitions()[0]
	definition.Extensions = []Extension{{
		Schema: "urn:example:employee", Name: "Employee", Description: "Employee fields",
		Attributes: []SchemaAttribute{{Name: "employeeNumber", Type: "string", Mutability: "readWrite", Returned: "default", Uniqueness: "none"}},
		Validate:   func(document Document) error { _, err := requiredString(document, "employeeNumber", 32); return err },
	}}
	registry, err := NewRegistry([]ResourceDefinition{definition})
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(ServerConfig{
		Store: NewMemoryStore(), Registry: registry, ExternalURL: "https://example.test/scim/v2",
		ResolveScope: func(*http.Request) (string, error) { return "tenant", nil }, Entropy: bytes.NewReader(make([]byte, 64)), Clock: func() time.Time { return testTime },
		AuthenticationSchemes: []AuthenticationScheme{{Type: "bearer", Name: "Bearer", Description: "Bearer"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	created := requestServer(t, server, http.MethodPost, "/scim/v2/Users", `{"schemas":["`+UserSchema+`","urn:example:employee"],"userName":"member","urn:example:employee":{"employeeNumber":"1"}}`, "tenant", nil)
	id := decodeResponse(t, created)["id"].(string)
	patched := requestServer(t, server, http.MethodPatch, "/scim/v2/Users/"+id, `{"schemas":["`+PatchSchema+`"],"Operations":[{"op":"replace","path":"urn:example:employee:employeeNumber","value":"2"}]}`, "tenant", nil)
	if patched.Code != 200 {
		t.Fatalf("extension PATCH = %d %s", patched.Code, patched.Body.String())
	}
	schema := requestServer(t, server, http.MethodGet, "/scim/v2/Schemas/urn:example:employee", "", "tenant", nil)
	if schema.Code != 200 || !strings.Contains(schema.Body.String(), "employeeNumber") {
		t.Fatalf("extension schema = %d %s", schema.Code, schema.Body.String())
	}
}

func TestPatchExtensionMissingAndPathFailures(t *testing.T) {
	definition := DefaultDefinitions()[0]
	definition.Extensions = []Extension{{Schema: "urn:optional", Validate: func(Document) error { return nil }}}
	base, _ := DecodeDocument([]byte(`{"schemas":["` + UserSchema + `"],"userName":"member"}`))
	for index, raw := range []string{
		`{"schemas":["` + PatchSchema + `"],"Operations":[{"op":"remove","path":"urn:optional:value"}]}`,
		`{"schemas":["` + PatchSchema + `"],"Operations":[{"op":"replace","path":"urn:optional:value","value":"x"}]}`,
		`{"schemas":["` + PatchSchema + `"],"Operations":[{"op":"add","path":"bad.deep.path","value":"x"}]}`,
		`{"schemas":["` + PatchSchema + `"],"Operations":[{"op":"add","path":"emails[type eq \"work\"]oops","value":"x"}]}`,
	} {
		request, err := DecodePatch([]byte(raw), 10)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := ApplyPatch(definition, base, request, "id"); err == nil {
			t.Errorf("invalid extension/path PATCH %d passed", index)
		}
	}
}

func TestWriterAndStoredRecordFailures(t *testing.T) {
	server := newTestServer(t)
	recorder := httptest.NewRecorder()
	server.writeRecord(recorder, 200, DefaultDefinitions()[0], Record{})
	if recorder.Code != 500 {
		t.Fatalf("invalid stored record = %d", recorder.Code)
	}
	recorder = httptest.NewRecorder()
	writeProtocolError(recorder, clientError(200, "", "invalid status"))
	if recorder.Code != 500 {
		t.Fatalf("invalid protocol error = %d", recorder.Code)
	}
	for _, err := range []error{ErrNotFound, ErrConflict, ErrTombstoned, ErrPrecondition, errors.New("private")} {
		recorder = httptest.NewRecorder()
		writeStoreError(recorder, err)
		if recorder.Code < 400 {
			t.Fatalf("writeStoreError(%v) = %d", err, recorder.Code)
		}
	}
}

func TestRecordAndStoreValidationFailures(t *testing.T) {
	valid := conformanceRecord("scope", "id", "", "member", testTime)
	invalid := []Record{
		{},
		func() Record { value := cloneRecord(valid); value.Version = "bad"; return value }(),
		func() Record {
			value := cloneRecord(valid)
			value.LastModified = value.Created.Add(-time.Second)
			return value
		}(),
		func() Record { value := cloneRecord(valid); value.Data = []byte(`[]`); return value }(),
		func() Record {
			value := cloneRecord(valid)
			value.Indexes = []IndexKey{{Name: "bad.path.deep", Value: "x"}}
			return value
		}(),
		func() Record { value := cloneRecord(valid); value.ID = "bad/id"; return value }(),
		func() Record { value := cloneRecord(valid); value.ExternalID = "mismatch"; return value }(),
		func() Record {
			value := cloneRecord(valid)
			value.Indexes = []IndexKey{{Name: "userName", Value: "x"}, {Name: "USERNAME", Value: "y"}}
			return value
		}(),
	}
	for index, record := range invalid {
		if err := validateRecord(record); err == nil {
			t.Errorf("invalid record %d passed", index)
		}
	}
	if _, err := calculateRecordVersion(Record{Data: json.RawMessage(`{`)}); err == nil {
		t.Fatal("invalid version input passed")
	}
	store := NewMemoryStore()
	if err := store.Transact(context.Background(), func(transaction Transaction) error { return transaction.Create(valid) }); err != nil {
		t.Fatal(err)
	}
	if err := store.Transact(context.Background(), func(transaction Transaction) error {
		changedManager := cloneRecord(valid)
		changedManager.Manager = "other"
		if err := transaction.Replace(changedManager, valid.Version); err == nil {
			return errors.New("manager mutation passed")
		}
		if _, err := transaction.Get("other", "User", valid.ID); !errors.Is(err, ErrNotFound) {
			return errors.New("scope lookup leaked")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestResourceLocationEscaping(t *testing.T) {
	server := newTestServer(t)
	location := server.resourceLocation(DefaultDefinitions()[0], "member@example.com")
	if location != "https://scim.example.test/scim/v2/Users/member@example.com" {
		t.Fatalf("resource location = %q", location)
	}
	if validResourceID("bad/id") || !validResourceID("member@example.com") {
		t.Fatal("resource ID path boundary is wrong")
	}
}

func TestRootExternalURLAndEscapedSchemaID(t *testing.T) {
	definition := DefaultDefinitions()[0]
	definition.Extensions = []Extension{{Schema: "https://schemas.example.test/employee/v1", Name: "Employee", Validate: func(Document) error { return nil }}}
	registry, err := NewRegistry([]ResourceDefinition{definition})
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(ServerConfig{
		Store: NewMemoryStore(), Registry: registry, ExternalURL: "https://scim.example.test",
		ResolveScope:          func(*http.Request) (string, error) { return "tenant", nil },
		AuthenticationSchemes: []AuthenticationScheme{{Type: "bearer", Name: "Bearer", Description: "Bearer"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	location := server.discoveryLocation("Schemas", "https://schemas.example.test/employee/v1")
	if !strings.Contains(location, "https:%2F%2Fschemas.example.test%2Femployee%2Fv1") {
		t.Fatalf("escaped schema location = %q", location)
	}
	path := strings.TrimPrefix(location, "https://scim.example.test")
	response := requestServer(t, server, http.MethodGet, path, "", "tenant", nil)
	if response.Code != 200 {
		t.Fatalf("escaped schema route = %d %s", response.Code, response.Body.String())
	}
}
