package scim

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestCompleteFilterGrammar(t *testing.T) {
	definition := DefaultDefinitions()[0]
	document := Document{
		"schemas": []any{UserSchema}, "id": "one", "userName": "Straße", "active": true,
		"name":   map[string]any{"familyName": "Zulu"},
		"emails": []any{map[string]any{"value": "member@example.test", "type": "work", "primary": true}},
		"meta":   map[string]any{"created": "2026-01-02T03:04:05Z"},
	}
	valid := []struct {
		filter string
		match  bool
	}{
		{`userName eq "STRASSE"`, true}, {`userName ne "other"`, true}, {`userName co "ra"`, true},
		{`userName sw "str"`, true}, {`userName ew "SSE"`, true}, {`active pr`, true},
		{`meta.created gt "2025-01-01T00:00:00Z"`, true}, {`name.familyName le "Zulu"`, true},
		{`emails[type eq "work" and value ew "example.test"]`, true},
		{`not (active eq false) and (userName eq "straße" or userName eq "none")`, true},
		{`displayName pr`, false}, {`displayName eq null`, false},
	}
	for _, test := range valid {
		expression, err := ParseFilter(test.filter, definition)
		if err != nil {
			t.Errorf("ParseFilter(%q): %v", test.filter, err)
			continue
		}
		matched, err := MatchFilter(expression, document)
		if err != nil || matched != test.match {
			t.Errorf("MatchFilter(%q) = (%v, %v)", test.filter, matched, err)
		}
	}
	for _, invalid := range []string{`active gt true`, `meta.created gt "bad"`, `emails eq "x"`, `userName regex "x"`, `not userName pr`, strings.Repeat("(", 40) + `userName pr` + strings.Repeat(")", 40)} {
		if _, err := ParseFilter(invalid, definition); err == nil {
			t.Errorf("invalid filter passed: %q", invalid)
		}
	}
}

func TestSearchSortProjectionAndPost(t *testing.T) {
	server := newTestServer(t)
	for _, raw := range []string{
		`{"schemas":["` + UserSchema + `"],"userName":"zulu","displayName":"Zulu","emails":[{"value":"z@example.test","type":"home"},{"value":"a@example.test","type":"work","primary":true}]}`,
		`{"schemas":["` + UserSchema + `"],"userName":"alpha","displayName":"Alpha","emails":[{"value":"b@example.test","type":"work"}]}`,
	} {
		if response := requestServer(t, server, http.MethodPost, "/scim/v2/Users", raw, "tenant", nil); response.Code != 201 {
			t.Fatal(response.Body.String())
		}
	}
	response := requestServer(t, server, http.MethodGet, `/scim/v2/Users?filter=emails%5Btype%20eq%20%22work%22%5D&sortBy=emails.value&sortOrder=descending&attributes=userName,emails.value`, "", "tenant", nil)
	decoded := decodeResponse(t, response)
	resources := decoded["Resources"].([]any)
	if response.Code != 200 || len(resources) != 2 || resources[0].(map[string]any)["userName"] != "alpha" {
		t.Fatalf("search = %d %#v", response.Code, decoded)
	}
	if _, exists := resources[0].(map[string]any)["displayName"]; exists {
		t.Fatal("projection returned unrequested attribute")
	}
	email := resources[0].(map[string]any)["emails"].([]any)[0].(map[string]any)
	if email["value"] == nil || len(email) != 1 {
		t.Fatalf("complex projection = %#v", email)
	}
	post := `{"schemas":["` + SearchRequestSchema + `"],"filter":"userName sw \"a\"","attributes":["userName"],"sortBy":"userName","count":1}`
	response = requestServer(t, server, http.MethodPost, "/scim/v2/Users/.search", post, "tenant", nil)
	decoded = decodeResponse(t, response)
	if response.Code != 200 || decoded["totalResults"] != json.Number("1") {
		t.Fatalf("POST search = %d %#v", response.Code, decoded)
	}
	root := `{"schemas":["` + SearchRequestSchema + `"],"filter":"urn:ietf:params:scim:schemas:core:2.0:User:userName pr"}`
	response = requestServer(t, server, http.MethodPost, "/scim/v2/.search", root, "tenant", nil)
	if response.Code != 200 || decodeResponse(t, response)["totalResults"] != json.Number("2") {
		t.Fatalf("root search = %d %s", response.Code, response.Body.String())
	}
	response = requestServer(t, server, http.MethodGet, `/scim/v2?filter=userName%20pr`, "", "tenant", nil)
	if response.Code != 200 || decodeResponse(t, response)["totalResults"] != json.Number("2") {
		t.Fatalf("root GET search = %d %s", response.Code, response.Body.String())
	}
}

func TestDiscoveryEnterprisePublicAndWeakETag(t *testing.T) {
	server, err := NewServer(ServerConfig{
		Store: NewMemoryStore(), ExternalURL: "https://scim.example.test/scim/v2", ResolveScope: func(*http.Request) (string, error) { return "", errors.New("denied") },
		AuthenticationSchemes: []AuthenticationScheme{{Type: "oauthbearertoken", Name: "Bearer", Description: "Bearer"}},
		PublicDiscovery:       true, WeakETags: true, DocumentationURI: "https://docs.example.test/scim", Entropy: bytes.NewReader(make([]byte, 64)), Clock: func() time.Time { return testTime },
	})
	if err != nil {
		t.Fatal(err)
	}
	config := requestServer(t, server, http.MethodGet, "/scim/v2/ServiceProviderConfig", "", "", nil)
	if config.Code != 200 || decodeResponse(t, config)["documentationUri"] == nil {
		t.Fatalf("public discovery = %d %s", config.Code, config.Body.String())
	}
	forbidden := requestServer(t, server, http.MethodGet, "/scim/v2/Schemas?filter=id%20pr", "", "", nil)
	if forbidden.Code != 403 {
		t.Fatalf("discovery filter = %d", forbidden.Code)
	}
	schema := requestServer(t, server, http.MethodGet, "/scim/v2/Schemas/"+EnterpriseUserSchema, "", "", nil)
	if schema.Code != 200 {
		t.Fatalf("enterprise schema = %d %s", schema.Code, schema.Body.String())
	}
	// Resources remain protected even when discovery is public.
	if protected := requestServer(t, server, http.MethodGet, "/scim/v2/Users", "", "", nil); protected.Code != 401 {
		t.Fatalf("protected collection = %d", protected.Code)
	}

	server.resolveScope = func(*http.Request) (string, error) { return "tenant", nil }
	created := requestServer(t, server, http.MethodPost, "/scim/v2/Users", `{"schemas":["`+UserSchema+`","`+EnterpriseUserSchema+`"],"id":123,"userName":"member","`+EnterpriseUserSchema+`":{"employeeNumber":"42","manager":{"value":"boss","displayName":"ignored"}}}`, "tenant", nil)
	resource := decodeResponse(t, created)
	if created.Code != 201 || !strings.HasPrefix(created.Header().Get("ETag"), "W/") || resource["id"] == nil {
		t.Fatalf("weak create = %d %#v", created.Code, resource)
	}
	manager := resource[EnterpriseUserSchema].(map[string]any)["manager"].(map[string]any)
	if _, exists := manager["displayName"]; exists {
		t.Fatal("readOnly manager displayName was retained")
	}
	id := resource["id"].(string)
	updated := requestServer(t, server, http.MethodPatch, "/scim/v2/Users/"+id, `{"schemas":["`+PatchSchema+`"],"Operations":[{"op":"add","path":"displayName","value":"Updated"}]}`, "tenant", map[string]string{"If-Match": created.Header().Get("ETag")})
	if updated.Code != 200 {
		t.Fatalf("weak If-Match = %d %s", updated.Code, updated.Body.String())
	}
}

type testPasswordStore struct {
	base       *MemoryStore
	lastDigest [32]byte
	writes     int
}

func (store *testPasswordStore) SupportsPasswordTransactions() {}
func (store *testPasswordStore) Transact(ctx context.Context, fn func(Transaction) error) error {
	var staged [32]byte
	changed := false
	err := store.base.Transact(ctx, func(transaction Transaction) error {
		return fn(&testPasswordTransaction{Transaction: transaction, stage: func(secret []byte) string {
			staged = sha256.Sum256(secret)
			changed = true
			return "credential-v" + string(rune('0'+store.writes+1))
		}})
	})
	if err == nil && changed {
		store.lastDigest = staged
		store.writes++
	}
	return err
}

type testPasswordTransaction struct {
	Transaction
	stage func([]byte) string
}

func (transaction *testPasswordTransaction) SetPassword(_, _, _ string, password []byte) (string, error) {
	return transaction.stage(password), nil
}

func TestPasswordTransactionAndReturnability(t *testing.T) {
	store := &testPasswordStore{base: NewMemoryStore()}
	server, err := NewServer(ServerConfig{Store: store, ExternalURL: "https://scim.example.test/scim/v2", ResolveScope: func(*http.Request) (string, error) { return "tenant", nil }, ChangePasswordSupported: true, AuthenticationSchemes: []AuthenticationScheme{{Type: "oauthbearertoken", Name: "Bearer", Description: "Bearer"}}, Entropy: bytes.NewReader(make([]byte, 64)), Clock: func() time.Time { return testTime }})
	if err != nil {
		t.Fatal(err)
	}
	created := requestServer(t, server, http.MethodPost, "/scim/v2/Users?attributes=userName,password", `{"schemas":["`+UserSchema+`"],"userName":"member","password":"sëcret phrase"}`, "tenant", nil)
	resource := decodeResponse(t, created)
	if created.Code != 201 || store.writes != 1 || store.lastDigest == ([32]byte{}) {
		t.Fatalf("password create = %d writes=%d", created.Code, store.writes)
	}
	if _, exists := resource["password"]; exists {
		t.Fatal("writeOnly password was returned")
	}
	id := resource["id"].(string)
	old := created.Header().Get("ETag")
	patched := requestServer(t, server, http.MethodPatch, "/scim/v2/Users/"+id, `{"schemas":["`+PatchSchema+`"],"Operations":[{"op":"replace","path":"password","value":"new secret"}]}`, "tenant", nil)
	if patched.Code != 200 || store.writes != 2 || patched.Header().Get("ETag") == old {
		t.Fatalf("password patch = %d writes=%d", patched.Code, store.writes)
	}
	if _, err := NewServer(ServerConfig{Store: NewMemoryStore(), ExternalURL: "https://x.test/scim", ResolveScope: func(*http.Request) (string, error) { return "x", nil }, ChangePasswordSupported: true, AuthenticationSchemes: []AuthenticationScheme{{Type: "x", Name: "x", Description: "x"}}}); err == nil {
		t.Fatal("lying password capability was admitted")
	}
}

func TestPatchSemanticsAndBulkDependencies(t *testing.T) {
	definition := DefaultDefinitions()[0]
	base := Document{"schemas": []any{UserSchema}, "userName": "member", "emails": []any{map[string]any{"value": "a@example.test", "type": "work", "primary": true}, map[string]any{"value": "b@example.test", "type": "home", "primary": false}}}
	raw := `{"schemas":["` + PatchSchema + `"],"Operations":[{"op":"remove","path":"emails[type eq \"missing\"]"},{"op":"replace","path":"emails[type eq \"work\" and value sw \"a\"].value","value":"updated@example.test"},{"op":"add","path":"emails","value":[{"value":"b@example.test","type":"home","primary":false}]},{"op":"add","path":"` + EnterpriseUserSchema + `:department","value":"Engineering"}]}`
	request, err := DecodePatch([]byte(raw), 20)
	if err != nil {
		t.Fatal(err)
	}
	patched, _, _, err := ApplyPatch(definition, base, request, "id")
	if err != nil {
		t.Fatal(err)
	}
	if len(patched["emails"].([]any)) != 2 || patched[EnterpriseUserSchema].(map[string]any)["department"] != "Engineering" {
		t.Fatalf("patched = %#v", patched)
	}

	server := newTestServer(t)
	bulk := `{"schemas":["` + BulkRequestSchema + `"],"Operations":[{"method":"PATCH","path":"/Users/bulkId:later","data":{"schemas":["` + PatchSchema + `"],"Operations":[{"op":"add","path":"displayName","value":"Forward"}]}},{"method":"POST","bulkId":"later","path":"/Users","data":{"schemas":["` + UserSchema + `"],"userName":"forward"}}]}`
	response := requestServer(t, server, http.MethodPost, "/scim/v2/Bulk", bulk, "tenant", nil)
	operations := decodeResponse(t, response)["Operations"].([]any)
	if operations[0].(map[string]any)["status"] != "200" || operations[1].(map[string]any)["status"] != "201" {
		t.Fatalf("forward Bulk = %#v", operations)
	}
	cycle := `{"schemas":["` + BulkRequestSchema + `"],"Operations":[{"method":"POST","bulkId":"a","path":"/Users","data":{"schemas":["` + UserSchema + `"],"userName":"bulkId:b"}},{"method":"POST","bulkId":"b","path":"/Users","data":{"schemas":["` + UserSchema + `"],"userName":"bulkId:a"}}]}`
	response = requestServer(t, server, http.MethodPost, "/scim/v2/Bulk", cycle, "tenant", nil)
	operations = decodeResponse(t, response)["Operations"].([]any)
	if operations[0].(map[string]any)["status"] != "409" || operations[1].(map[string]any)["status"] != "409" {
		t.Fatalf("cyclic Bulk = %#v", operations)
	}
}
