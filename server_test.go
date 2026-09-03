package scim

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	entropy := make([]byte, 16*32)
	for index := range entropy {
		entropy[index] = byte(index)
	}
	server, err := NewServer(ServerConfig{
		Store: NewMemoryStore(), ExternalURL: "https://scim.example.test/scim/v2",
		ResolveScope: func(request *http.Request) (string, error) {
			scope := request.Header.Get("X-Scope")
			if scope == "" {
				return "", errors.New("denied")
			}
			return scope, nil
		},
		Clock: func() time.Time { return testTime }, Entropy: bytes.NewReader(entropy), MaximumPageSize: 2,
		AuthenticationSchemes: []AuthenticationScheme{{Type: "oauthbearertoken", Name: "Bearer", Description: "Bearer token supplied by the consumer"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return server
}

func requestServer(t *testing.T, server *Server, method, path, body, scope string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, "https://internal.test"+path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/scim+json")
	}
	if scope != "" {
		request.Header.Set("X-Scope", scope)
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	return response
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var decoded map[string]any
	decoder := json.NewDecoder(response.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		t.Fatalf("decode response %d %q: %v", response.Code, response.Body.String(), err)
	}
	return decoded
}

func TestServerResourceLifecycle(t *testing.T) {
	server := newTestServer(t)
	created := requestServer(t, server, http.MethodPost, "/scim/v2/Users", `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"externalId":"upstream-1","userName":"member","active":true}`, "tenant-a", nil)
	if created.Code != http.StatusCreated || created.Header().Get("ETag") == "" || created.Header().Get("Location") == "" || created.Header().Get("Content-Type") != "application/scim+json" {
		t.Fatalf("create = %d, headers=%v, body=%s", created.Code, created.Header(), created.Body.String())
	}
	resource := decodeResponse(t, created)
	id, _ := resource["id"].(string)
	version := created.Header().Get("ETag")
	if id == "" || resource["meta"].(map[string]any)["version"] != version {
		t.Fatalf("created resource = %#v", resource)
	}

	notModified := requestServer(t, server, http.MethodGet, "/scim/v2/Users/"+id, "", "tenant-a", map[string]string{"If-None-Match": "W/" + version})
	if notModified.Code != http.StatusNotModified || notModified.Body.Len() != 0 {
		t.Fatalf("conditional GET = %d %q", notModified.Code, notModified.Body.String())
	}
	isolated := requestServer(t, server, http.MethodGet, "/scim/v2/Users/"+id, "", "tenant-b", nil)
	if isolated.Code != http.StatusNotFound {
		t.Fatalf("cross-scope GET = %d", isolated.Code)
	}

	wrong := requestServer(t, server, http.MethodPut, "/scim/v2/Users/"+id, `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"id":"`+id+`","externalId":"upstream-1","userName":"renamed"}`, "tenant-a", map[string]string{"If-Match": `"wrong"`})
	if wrong.Code != http.StatusPreconditionFailed {
		t.Fatalf("wrong precondition = %d %s", wrong.Code, wrong.Body.String())
	}

	patched := requestServer(t, server, http.MethodPatch, "/scim/v2/Users/"+id, `{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"replace","path":"active","value":false},{"op":"add","path":"displayName","value":"Member"}]}`, "tenant-a", map[string]string{"If-Match": version})
	if patched.Code != http.StatusOK || patched.Header().Get("ETag") == version {
		t.Fatalf("PATCH = %d headers=%v body=%s", patched.Code, patched.Header(), patched.Body.String())
	}
	patchedResource := decodeResponse(t, patched)
	if patchedResource["active"] != false || patchedResource["displayName"] != "Member" {
		t.Fatalf("PATCH resource = %#v", patchedResource)
	}

	listed := requestServer(t, server, http.MethodGet, `/scim/v2/Users?filter=userName%20eq%20%22member%22&startIndex=1&count=1`, "", "tenant-a", nil)
	list := decodeResponse(t, listed)
	if listed.Code != http.StatusOK || list["totalResults"] != json.Number("1") || len(list["Resources"].([]any)) != 1 {
		t.Fatalf("list = %d %#v", listed.Code, list)
	}

	deleted := requestServer(t, server, http.MethodDelete, "/scim/v2/Users/"+id, "", "tenant-a", map[string]string{"If-Match": patched.Header().Get("ETag")})
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("DELETE = %d %s", deleted.Code, deleted.Body.String())
	}
	missing := requestServer(t, server, http.MethodGet, "/scim/v2/Users/"+id, "", "tenant-a", nil)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("deleted GET = %d", missing.Code)
	}
	recreate := requestServer(t, server, http.MethodPost, "/scim/v2/Users", `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"externalId":"upstream-1","userName":"other"}`, "tenant-a", nil)
	if recreate.Code != http.StatusConflict {
		t.Fatalf("tombstoned recreation = %d %s", recreate.Code, recreate.Body.String())
	}
}

func TestServerDiscoveryAndBoundaries(t *testing.T) {
	server := newTestServer(t)
	for _, path := range []string{"/scim/v2/ServiceProviderConfig", "/scim/v2/ResourceTypes", "/scim/v2/ResourceTypes/User", "/scim/v2/Schemas", "/scim/v2/Schemas/" + UserSchema} {
		response := requestServer(t, server, http.MethodGet, path, "", "tenant", nil)
		if response.Code != http.StatusOK {
			t.Errorf("GET %s = %d %s", path, response.Code, response.Body.String())
		}
	}
	unauthorized := requestServer(t, server, http.MethodGet, "/scim/v2/Users", "", "", nil)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized = %d", unauthorized.Code)
	}
	outside := requestServer(t, server, http.MethodGet, "/other/Users", "", "tenant", nil)
	if outside.Code != http.StatusNotFound {
		t.Fatalf("outside route = %d", outside.Code)
	}
	sorted := requestServer(t, server, http.MethodGet, "/scim/v2/Users?sortBy=userName", "", "tenant", nil)
	if sorted.Code != http.StatusOK {
		t.Fatalf("sorted query = %d", sorted.Code)
	}
	method := requestServer(t, server, http.MethodPost, "/scim/v2/ServiceProviderConfig", `{}`, "tenant", nil)
	if method.Code != http.StatusMethodNotAllowed || method.Header().Get("Allow") != "GET" {
		t.Fatalf("unsupported method = %d %v", method.Code, method.Header())
	}
	media := httptest.NewRequest(http.MethodPost, "https://internal.test/scim/v2/Users", strings.NewReader(`{}`))
	media.Header.Set("X-Scope", "tenant")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, media)
	if recorder.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("missing media type = %d", recorder.Code)
	}
}

func TestServerBulkReferencesAndFailOnErrors(t *testing.T) {
	server := newTestServer(t)
	request := `{"schemas":["urn:ietf:params:scim:api:messages:2.0:BulkRequest"],"failOnErrors":1,"Operations":[` +
		`{"method":"POST","bulkId":"new-user","path":"/Users","data":{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"member"}},` +
		`{"method":"POST","bulkId":"new-group","path":"/Groups","data":{"schemas":["urn:ietf:params:scim:schemas:core:2.0:Group"],"displayName":"Operators","members":[{"value":"bulkId:new-user","$ref":"bulkId:new-user"}]}},` +
		`{"method":"POST","bulkId":"duplicate","path":"/Users","data":{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"member"}},` +
		`{"method":"POST","bulkId":"not-run","path":"/Users","data":{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"later"}}]}`
	response := requestServer(t, server, http.MethodPost, "/scim/v2/Bulk", request, "tenant", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("Bulk = %d %s", response.Code, response.Body.String())
	}
	decoded := decodeResponse(t, response)
	operations := decoded["Operations"].([]any)
	if len(operations) != 3 || operations[0].(map[string]any)["status"] != "201" || operations[2].(map[string]any)["status"] != "409" {
		t.Fatalf("Bulk operations = %#v", operations)
	}
	groupList := requestServer(t, server, http.MethodGet, `/scim/v2/Groups?filter=displayName%20eq%20%22Operators%22`, "", "tenant", nil)
	group := decodeResponse(t, groupList)["Resources"].([]any)[0].(map[string]any)
	member := group["members"].([]any)[0].(map[string]any)
	if member["value"] == "bulkId:new-user" || !strings.HasPrefix(member["$ref"].(string), "https://scim.example.test/") {
		t.Fatalf("Bulk references were not resolved: %#v", member)
	}
}

func TestNewServerRejectsInvalidConfiguration(t *testing.T) {
	_, err := NewServer(ServerConfig{})
	if err == nil {
		t.Fatal("empty config passed")
	}
	_, err = NewServer(ServerConfig{Store: NewMemoryStore(), ResolveScope: func(*http.Request) (string, error) { return "x", nil }, ExternalURL: "http://example.test/scim", AuthenticationSchemes: []AuthenticationScheme{{Type: "bearer", Name: "Bearer", Description: "Bearer"}}})
	if err == nil {
		t.Fatal("plaintext external URL passed")
	}
}
