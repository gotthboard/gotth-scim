package scim

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestRFCFilterEdgeTypes(t *testing.T) {
	definition := ResourceDefinition{Schema: "urn:test", Attributes: []SchemaAttribute{
		{Name: "integer", Type: "integer", Mutability: "readWrite", Returned: "default", Uniqueness: "none"},
		{Name: "decimal", Type: "decimal", Mutability: "readWrite", Returned: "default", Uniqueness: "none"},
		{Name: "binary", Type: "binary", Mutability: "readWrite", Returned: "default", Uniqueness: "none"},
		{Name: "flag", Type: "boolean", Mutability: "readWrite", Returned: "default", Uniqueness: "none"},
		{Name: "tags", Type: "string", MultiValued: true, Mutability: "readWrite", Returned: "default", Uniqueness: "none"},
	}}
	document := Document{"integer": json.Number("9007199254740993"), "decimal": json.Number("1.50"), "binary": "YQ==", "flag": false, "tags": []any{"one", "two"}}
	for _, raw := range []string{`integer gt 9007199254740992`, `decimal eq 1.5`, `binary eq "YQ=="`, `flag eq false`, `tags eq "two"`, `integer ne null`} {
		expression, err := ParseFilter(raw, definition)
		if err != nil {
			t.Errorf("%s: %v", raw, err)
			continue
		}
		if matched, err := MatchFilter(expression, document); err != nil || !matched {
			t.Errorf("%s = %v %v", raw, matched, err)
		}
	}
	for _, raw := range []string{`integer eq 1.5`, `binary gt "a"`, `flag lt false`, `decimal eq nope`, `unknown pr`, `tags[value eq "x"]`, `integer eq 1e999999`} {
		if _, err := ParseFilter(raw, definition); err == nil {
			t.Errorf("invalid filter passed: %s", raw)
		}
	}
	for _, raw := range []string{"", `userName eq "unterminated`, "userName eq \"bad\\q\"", strings.Repeat("userName pr or ", 300) + "userName pr"} {
		if _, err := lexFilter(raw); err == nil {
			t.Errorf("invalid lexical filter passed: %q", raw)
		}
	}
}

func TestRFCInternalBoundaryBranches(t *testing.T) {
	if documentID(Document{"id": "x"}) != "x" || documentID(Document{}) != "" {
		t.Fatal("document ID")
	}
	record := Record{ExternalID: "external", Indexes: []IndexKey{{Name: "userName", Value: "Member"}}}
	if !recordMatches(record, "externalId", "external") || !recordMatches(record, "userName", "member") || recordMatches(record, "userName", "other") {
		t.Fatal("record match")
	}
	for value, expected := range map[any]bool{nil: false, "": false, "x": true, false: true} {
		if filterPresent(value) != expected {
			t.Errorf("present %#v", value)
		}
	}
	if filterPresent([]any{}) || !filterPresent([]any{"x"}) || filterPresent(map[string]any{}) || !filterPresent(map[string]any{"x": 1}) {
		t.Fatal("collection presence")
	}
	if comparison, ok := orderFilterValues(false, true, SchemaAttribute{Type: "boolean"}); !ok || comparison >= 0 {
		t.Fatal("boolean order")
	}
	if comparison, ok := orderFilterValues("2026-01-01T00:00:00Z", "2025-01-01T00:00:00Z", SchemaAttribute{Type: "dateTime"}); !ok || comparison <= 0 {
		t.Fatal("date order")
	}
	if compareSortValues(Document{}, Document{}, []string{"x"}, SchemaAttribute{Type: "string"}) != 0 || compareSortValues(Document{}, Document{"x": "a"}, []string{"x"}, SchemaAttribute{Type: "string"}) <= 0 || compareSortValues(Document{"x": "a"}, Document{}, []string{"x"}, SchemaAttribute{Type: "string"}) >= 0 {
		t.Fatal("missing sort order")
	}
	if _, ok := orderFilterValues("bad", "2025-01-01T00:00:00Z", SchemaAttribute{Type: "dateTime"}); ok {
		t.Fatal("bad date compared")
	}

	target := map[string]any{}
	if err := applyDirectPatch(target, "name", "givenName", "add", "A"); err != nil {
		t.Fatal(err)
	}
	if err := applyDirectPatch(target, "name", "givenName", "replace", "B"); err != nil {
		t.Fatal(err)
	}
	if err := applyDirectPatch(target, "name", "givenName", "remove", nil); err != nil {
		t.Fatal(err)
	}
	if err := applyDirectPatch(target, "missing", "", "remove", nil); err == nil {
		t.Fatal("missing removal passed")
	}
	target["scalar"] = "x"
	if err := applyDirectPatch(target, "scalar", "sub", "add", "x"); err == nil {
		t.Fatal("scalar parent passed")
	}

	plan := searchPlan{sortPath: []string{"userName"}, sortAttr: SchemaAttribute{Type: "string"}, start: 1, count: 2}
	resources := []Document{{"id": "b", "userName": "same"}, {"id": "a", "userName": "same"}}
	sorted, _, err := applySearch(plan, resources)
	if err != nil || sorted[0]["id"] != "a" {
		t.Fatalf("stable tie = %#v %v", sorted, err)
	}
	plan.descending = true
	sorted, _, _ = applySearch(plan, resources)
	if sorted[0]["id"] != "b" {
		t.Fatalf("descending tie = %#v", sorted)
	}

	protocol := bulkFailure(BulkOperation{Method: "POST", BulkID: "x"}, clientError(400, "invalidValue", "bad"))
	if protocol.Status != "400" || protocol.Response == nil {
		t.Fatal("bulk protocol failure")
	}
	internal := bulkFailure(BulkOperation{Method: "POST"}, ErrNotFound)
	if internal.Status != "404" {
		t.Fatal("bulk store failure")
	}

	definition := ResourceDefinition{Attributes: []SchemaAttribute{{Name: "immutable", Type: "string", Mutability: "immutable", Returned: "default", Uniqueness: "none"}, {Name: "readonly", Type: "string", Mutability: "readOnly", Returned: "default", Uniqueness: "none"}}}
	current := Document{"immutable": "old", "readonly": "server"}
	incoming := Document{"readonly": "client"}
	if err := enforceReplacementMutability(definition, current, incoming); err != nil || incoming["immutable"] != "old" || incoming["readonly"] != "server" {
		t.Fatalf("mutability preserve = %#v %v", incoming, err)
	}
	incoming["immutable"] = "new"
	if err := enforceReplacementMutability(definition, current, incoming); err == nil {
		t.Fatal("immutable mutation passed")
	}

	if _, ok := exactFilterNumber(struct{}{}); ok {
		t.Fatal("non-number converted")
	}
}

func TestSchemaValueTypeValidation(t *testing.T) {
	attributes := []SchemaAttribute{
		{Name: "text", Type: "string", Required: true, Mutability: "readWrite", Returned: "default", Uniqueness: "none"},
		{Name: "reference", Type: "reference", ReferenceTypes: []string{"external"}, Mutability: "readWrite", Returned: "default", Uniqueness: "none"},
		{Name: "flag", Type: "boolean", Mutability: "readWrite", Returned: "default", Uniqueness: "none"},
		{Name: "integer", Type: "integer", Mutability: "readWrite", Returned: "default", Uniqueness: "none"},
		{Name: "decimal", Type: "decimal", Mutability: "readWrite", Returned: "default", Uniqueness: "none"},
		{Name: "when", Type: "dateTime", Mutability: "readWrite", Returned: "default", Uniqueness: "none"},
		{Name: "binary", Type: "binary", Mutability: "readWrite", Returned: "default", Uniqueness: "none"},
		{Name: "object", Type: "complex", Mutability: "readWrite", Returned: "default", Uniqueness: "none", SubAttributes: []SchemaAttribute{{Name: "value", Type: "string", Required: true, Mutability: "readWrite", Returned: "default", Uniqueness: "none"}}},
		{Name: "many", Type: "string", MultiValued: true, Mutability: "readWrite", Returned: "default", Uniqueness: "none"},
	}
	valid := map[string]any{"text": "x", "reference": "https://example.test/x", "flag": true, "integer": json.Number("2"), "decimal": json.Number("1.5"), "when": "2026-01-02T03:04:05Z", "binary": "eA==", "object": map[string]any{"value": "x"}, "many": []any{"a", "b"}}
	if err := validateAttributeValuesExact(valid, attributes); err != nil {
		t.Fatal(err)
	}
	invalid := []map[string]any{
		{}, {"text": true}, {"text": "x", "reference": "relative"}, {"text": "x", "flag": "yes"},
		{"text": "x", "integer": json.Number("1.5")}, {"text": "x", "decimal": "one"}, {"text": "x", "when": "bad"},
		{"text": "x", "binary": "bad"}, {"text": "x", "object": "bad"}, {"text": "x", "object": map[string]any{}},
		{"text": "x", "object": map[string]any{"value": "x", "unknown": true}}, {"text": "x", "many": "one"}, {"text": []any{"x"}}, {"text": "x", "unknown": true},
	}
	for index, object := range invalid {
		if err := validateAttributeValuesExact(object, attributes); err == nil {
			t.Errorf("invalid schema value %d passed", index)
		}
	}
	object := map[string]any{"empty": "", "nil": nil, "array": []any{"", nil, "x"}, "nested": map[string]any{"empty": ""}}
	removeUnassigned(object, false)
	if object["empty"] != nil || object["nil"] != nil || len(object["array"].([]any)) != 1 {
		t.Fatalf("unassigned normalization = %#v", object)
	}
	for _, enterprise := range []Document{{"unknown": "x"}, {"manager": "bad"}, {"manager": map[string]any{"value": true}}, {"employeeNumber": true}} {
		if err := validateEnterpriseUser(enterprise); err == nil {
			t.Errorf("invalid enterprise value passed: %#v", enterprise)
		}
	}
	badWriteOnly := ResourceDefinition{Name: "Thing", Endpoint: "Things", Schema: "urn:thing", Attributes: []SchemaAttribute{{Name: "secret", Type: "string", Mutability: "writeOnly", Returned: "never", Uniqueness: "none"}}, Validate: func(Document, WriteMode) error { return nil }}
	if _, err := NewRegistry([]ResourceDefinition{badWriteOnly}); err == nil {
		t.Fatal("unadapted write-only schema passed")
	}
	badMetadata := ResourceDefinition{Name: "Thing", Endpoint: "Things", Schema: "urn:thing", Attributes: []SchemaAttribute{{Name: "text", Type: "string", Mutability: "readWrite", Returned: "default", Uniqueness: "none", ReferenceTypes: []string{"external"}}}, Validate: func(Document, WriteMode) error { return nil }}
	if _, err := NewRegistry([]ResourceDefinition{badMetadata}); err == nil {
		t.Fatal("invalid reference metadata passed")
	}
}

func TestSearchValidationAndReturnability(t *testing.T) {
	definition := ResourceDefinition{Schema: "urn:test", Attributes: []SchemaAttribute{
		{Name: "always", Type: "string", Mutability: "readOnly", Returned: "always", Uniqueness: "none"},
		{Name: "requested", Type: "string", Mutability: "readWrite", Returned: "request", Uniqueness: "none"},
		{Name: "secret", Type: "string", Mutability: "writeOnly", Returned: "never", Uniqueness: "none"},
		{Name: "object", Type: "complex", Mutability: "readWrite", Returned: "default", Uniqueness: "none", SubAttributes: []SchemaAttribute{{Name: "one", Type: "string", Mutability: "readWrite", Returned: "default", Uniqueness: "none"}, {Name: "two", Type: "string", Mutability: "readWrite", Returned: "default", Uniqueness: "none"}}},
	}}
	document := Document{"schemas": []any{"urn:test"}, "id": "id", "always": "yes", "requested": "ask", "secret": "no", "object": map[string]any{"one": "1", "two": "2"}}
	projected, err := projectDocument(document, [][]string{{"requested"}, {"object", "one"}}, nil, definition)
	if err != nil || projected["requested"] != "ask" || projected["always"] != "yes" || projected["secret"] != nil || len(projected["object"].(map[string]any)) != 1 {
		t.Fatalf("include projection = %#v %v", projected, err)
	}
	projected, err = projectDocument(document, nil, [][]string{{"always"}, {"object", "two"}}, definition)
	if err != nil || projected["always"] != "yes" || projected["requested"] != nil || projected["object"].(map[string]any)["two"] != nil {
		t.Fatalf("exclude projection = %#v %v", projected, err)
	}

	zero := 0
	if _, err := compileSearch(SearchRequest{Attributes: []string{"id"}, ExcludedAttributes: []string{"id"}}, definition, 10); err == nil {
		t.Fatal("mutually exclusive projection passed")
	}
	if _, err := compileSearch(SearchRequest{SortOrder: "sideways"}, definition, 10); err == nil {
		t.Fatal("bad sort order passed")
	}
	if _, err := compileSearch(SearchRequest{SortBy: "object"}, definition, 10); err == nil {
		t.Fatal("complex sort passed")
	}
	if plan, err := compileSearch(SearchRequest{StartIndex: -1, Count: &zero}, definition, 10); err != nil || plan.start != 1 || plan.count != 0 {
		t.Fatalf("zero search plan = %+v %v", plan, err)
	}
	if _, err := compileProjectionPaths([]string{"missing"}, definition); err == nil {
		t.Fatal("unknown projection passed")
	}
	tooMany := make([]string, 101)
	if _, err := compileProjectionPaths(tooMany, definition); err == nil {
		t.Fatal("large projection passed")
	}
	for _, raw := range []string{"", `{}`, `{"schemas":["wrong"]}`, `{"schemas":["` + SearchRequestSchema + `"],"unknown":true}`, `{"schemas":["` + SearchRequestSchema + `"]} trailing`} {
		if _, err := decodeSearchRequest([]byte(raw)); err == nil {
			t.Errorf("invalid search passed: %q", raw)
		}
	}
	values := url.Values{"count": {"1", "2"}}
	if _, err := searchRequestFromQuery(values); err == nil {
		t.Fatal("repeated query passed")
	}
}

func TestRFCMutabilityAndCredentialFailures(t *testing.T) {
	group := DefaultDefinitions()[1]
	current := Document{"schemas": []any{GroupSchema}, "displayName": "group", "members": []any{map[string]any{"value": "u", "$ref": "https://x/Users/u", "type": "User"}}}
	changed, _ := cloneDocument(current)
	changed["members"].([]any)[0].(map[string]any)["$ref"] = "https://x/Users/other"
	if err := enforceReplacementMutability(group, current, changed); err == nil {
		t.Fatal("immutable member changed")
	}
	omitted := Document{"schemas": []any{GroupSchema}, "displayName": "group", "members": []any{map[string]any{"value": "u"}}}
	if err := enforceReplacementMutability(group, current, omitted); err != nil || omitted["members"].([]any)[0].(map[string]any)["type"] != "User" {
		t.Fatalf("immutable member omission = %#v %v", omitted, err)
	}
	for _, document := range []Document{{"password": true}, {"password": ""}, {"password": strings.Repeat("x", 1025)}} {
		if _, err := extractPassword(document, true, "User"); err == nil {
			t.Errorf("invalid password passed: %#v", document)
		}
	}
	if _, err := extractPassword(Document{"password": "secret"}, false, "User"); err == nil {
		t.Fatal("disabled password passed")
	}
	if _, err := writePassword(&memoryTransaction{state: &memoryState{}}, "s", "User", "id", []byte("x")); err == nil {
		t.Fatal("missing password adapter passed")
	}
	user := DefaultDefinitions()[0]
	for _, raw := range []string{
		`{"schemas":["` + UserSchema + `"],"userName":"member","profileUrl":"relative"}`,
		`{"schemas":["` + UserSchema + `"],"userName":"member","photos":[{"value":"javascript:bad"}]}`,
		`{"schemas":["` + UserSchema + `"],"userName":"member","x509Certificates":[{"value":"not base64"}]}`,
		`{"schemas":["` + UserSchema + `"],"userName":"member","emails":[{"$ref":"https://smuggled.test"}]}`,
	} {
		document, err := DecodeDocument([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := prepareResource(user, document, CreateMode, ""); err == nil {
			t.Errorf("invalid typed resource passed: %s", raw)
		}
	}
}

func TestSearchHTTPFailuresAndBounds(t *testing.T) {
	server := newTestServer(t)
	server.maximumSearchCandidates = 1
	for _, name := range []string{"one", "two"} {
		response := requestServer(t, server, http.MethodPost, "/scim/v2/Users", `{"schemas":["`+UserSchema+`"],"userName":"`+name+`"}`, "tenant", nil)
		if response.Code != 201 {
			t.Fatal(response.Body.String())
		}
	}
	if response := requestServer(t, server, http.MethodGet, "/scim/v2/Users", "", "tenant", nil); response.Code != 413 {
		t.Fatalf("candidate bound = %d %s", response.Code, response.Body.String())
	}
	if response := requestServer(t, server, http.MethodGet, "/scim/v2/Users/.search", "", "tenant", nil); response.Code != 405 {
		t.Fatalf("search method = %d", response.Code)
	}
	if response := requestServer(t, server, http.MethodPost, "/scim/v2/Users/.search?x=1", `{"schemas":["`+SearchRequestSchema+`"]}`, "tenant", nil); response.Code != 400 {
		t.Fatalf("search query = %d", response.Code)
	}
	if response := requestServer(t, server, http.MethodPost, "/scim/v2/Users/.search", `{}`, "tenant", nil); response.Code != 400 {
		t.Fatalf("search body = %d", response.Code)
	}
	if _, err := NewServer(ServerConfig{Store: NewMemoryStore(), ExternalURL: "https://x.test/scim", ResolveScope: func(*http.Request) (string, error) { return "x", nil }, DocumentationURI: "http://bad.test", AuthenticationSchemes: []AuthenticationScheme{{Type: "x", Name: "x", Description: "x"}}}); err == nil {
		t.Fatal("insecure documentation URI passed")
	}
}

func TestProjectionAndSearchHTTPValidation(t *testing.T) {
	server := newTestServer(t)
	created := requestServer(t, server, http.MethodPost, "/scim/v2/Users", `{"schemas":["`+UserSchema+`"],"userName":"member"}`, "tenant", nil)
	id := decodeResponse(t, created)["id"].(string)
	for _, path := range []string{
		"/scim/v2/Users/" + id + "?attributes=missing",
		"/scim/v2/Users/" + id + "?attributes=userName&excludedAttributes=displayName",
		"/scim/v2/Users?filter=active%20gt%20true",
		"/scim/v2/Users?sortOrder=sideways",
		"/scim/v2/Users?sortBy=name",
	} {
		if response := requestServer(t, server, http.MethodGet, path, "", "tenant", nil); response.Code != 400 {
			t.Errorf("invalid query %s = %d %s", path, response.Code, response.Body.String())
		}
	}
	projected := requestServer(t, server, http.MethodGet, "/scim/v2/Users/"+id+"?excludedAttributes=userName", "", "tenant", nil)
	if projected.Code != 200 || decodeResponse(t, projected)["userName"] != nil {
		t.Fatalf("GET projection = %d %s", projected.Code, projected.Body.String())
	}
	withEmail := requestServer(t, server, http.MethodPatch, "/scim/v2/Users/"+id, `{"schemas":["`+PatchSchema+`"],"Operations":[{"op":"add","path":"emails","value":[{"value":"a@example.test","type":"work"}]}]}`, "tenant", nil)
	if withEmail.Code != 200 {
		t.Fatal(withEmail.Body.String())
	}
	projected = requestServer(t, server, http.MethodGet, "/scim/v2/Users/"+id+"?excludedAttributes=emails.value", "", "tenant", nil)
	email := decodeResponse(t, projected)["emails"].([]any)[0].(map[string]any)
	if email["value"] != nil || email["type"] != "work" {
		t.Fatalf("excluded complex subattribute = %#v", email)
	}
}

func TestBulkUnknownReferencesAndLocations(t *testing.T) {
	server := newTestServer(t)
	bulk := `{"schemas":["` + BulkRequestSchema + `"],"Operations":[{"method":"DELETE","path":"/Users/bulkId:missing"}]}`
	response := requestServer(t, server, http.MethodPost, "/scim/v2/Bulk", bulk, "tenant", nil)
	operation := decodeResponse(t, response)["Operations"].([]any)[0].(map[string]any)
	if operation["status"] != "400" {
		t.Fatalf("unknown reference = %#v", operation)
	}
	if location := server.bulkOperationLocation(BulkOperation{Path: "/bad/path/extra"}, nil); location != "" {
		t.Fatalf("invalid location = %q", location)
	}
	data := json.RawMessage(`{"members":[{"value":"bulkId:x","$ref":"bulkId:x"}]}`)
	resolved, err := resolveBulkData(data, map[string]bulkReference{"x": {id: "id", location: "https://x/Groups/id"}})
	if err != nil || !bytes.Contains(resolved, []byte(`"value":"id"`)) || !bytes.Contains(resolved, []byte(`"$ref":"https://x/Groups/id"`)) {
		t.Fatalf("resolved data = %s %v", resolved, err)
	}
}
