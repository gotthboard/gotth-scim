package scim_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"

	scim "github.com/gotthboard/gotth-scim"
)

func TestPublicServerAndReconcilerAPI(t *testing.T) {
	registry, err := scim.NewRegistry(scim.DefaultDefinitions())
	if err != nil {
		t.Fatal(err)
	}
	store := scim.NewMemoryStore()
	server, err := scim.NewServer(scim.ServerConfig{
		Store: store, Registry: registry, ExternalURL: "https://example.test/scim/v2",
		ResolveScope:          func(*http.Request) (string, error) { return "tenant", nil },
		AuthenticationSchemes: []scim.AuthenticationScheme{{Type: "oauthbearertoken", Name: "Bearer", Description: "OAuth bearer token"}},
	})
	if err != nil || server == nil {
		t.Fatalf("NewServer() = (%v, %v)", server, err)
	}
	reconciler, err := scim.NewReconciler(store, registry, nil, bytes.NewReader(make([]byte, 16)))
	if err != nil {
		t.Fatal(err)
	}
	result, err := reconciler.Reconcile(context.Background(), scim.ReconcileRequest{Scope: "tenant", Manager: "controller", Resources: []scim.DesiredResource{{ResourceType: "User", ExternalID: "upstream", Data: []byte(`{"schemas":["` + scim.UserSchema + `"],"userName":"member"}`)}}})
	if err != nil || result.Created != 1 {
		t.Fatalf("Reconcile() = (%+v, %v)", result, err)
	}
	if !errors.Is(scim.ErrNotFound, scim.ErrNotFound) {
		t.Fatal("exported store errors are not comparable")
	}
}
