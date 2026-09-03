package scim

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

func TestReconcilerLifecycleAndOwnership(t *testing.T) {
	registry, err := NewRegistry(DefaultDefinitions())
	if err != nil {
		t.Fatal(err)
	}
	store := NewMemoryStore()
	entropy := make([]byte, 16*16)
	for index := range entropy {
		entropy[index] = byte(index)
	}
	reconciler, err := NewReconciler(store, registry, func() time.Time { return testTime }, bytes.NewReader(entropy))
	if err != nil {
		t.Fatal(err)
	}
	request := ReconcileRequest{Scope: "tenant", Manager: "controller-a", DeleteMissing: true, Resources: []DesiredResource{
		{ResourceType: "User", ExternalID: "upstream-user", Data: []byte(`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"member","active":true}`)},
		{ResourceType: "Group", ExternalID: "upstream-group", Data: []byte(`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:Group"],"displayName":"Operators"}`)},
	}}
	result, err := reconciler.Reconcile(context.Background(), request)
	if err != nil || result.Created != 2 {
		t.Fatalf("first reconcile = (%+v, %v)", result, err)
	}
	result, err = reconciler.Reconcile(context.Background(), request)
	if err != nil || result.Unchanged != 2 {
		t.Fatalf("no-op reconcile = (%+v, %v)", result, err)
	}

	var originalID string
	if err := store.Transact(context.Background(), func(transaction Transaction) error {
		records, err := transaction.List(Query{Scope: "tenant", ResourceType: "User", Attribute: "externalId", Value: "upstream-user", Limit: 10})
		if err == nil && len(records) == 1 {
			originalID = records[0].ID
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	request.Resources = []DesiredResource{{ResourceType: "User", ExternalID: "upstream-user", Data: []byte(`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"member","active":false}`)}}
	result, err = reconciler.Reconcile(context.Background(), request)
	if err != nil || result.Updated != 1 || result.Deleted != 1 {
		t.Fatalf("update reconcile = (%+v, %v)", result, err)
	}
	if err := store.Transact(context.Background(), func(transaction Transaction) error {
		records, err := transaction.List(Query{Scope: "tenant", ResourceType: "User", Attribute: "externalId", Value: "upstream-user", Limit: 10})
		if err != nil || len(records) != 1 || records[0].ID != originalID {
			return errors.New("reconciliation did not preserve provider ID")
		}
		tombstones, err := transaction.Tombstones("tenant", "Group")
		if err != nil || len(tombstones) != 1 || tombstones[0].Manager != "controller-a" {
			return errors.New("missing managed resource did not become an owned tombstone")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	request.Resources = append(request.Resources, DesiredResource{ResourceType: "Group", ExternalID: "upstream-group", Data: []byte(`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:Group"],"displayName":"Operators"}`)})
	if _, err := reconciler.Reconcile(context.Background(), request); !errors.Is(err, ErrTombstoned) {
		t.Fatalf("tombstoned desired identity = %v", err)
	}
}

func TestReconcilerDoesNotAdoptOtherManager(t *testing.T) {
	registry, _ := NewRegistry(DefaultDefinitions())
	store := NewMemoryStore()
	document, _ := DecodeDocument([]byte(`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"externalId":"shared","userName":"member"}`))
	definition, _ := registry.definitionByName("User")
	document, indexes, externalID, _ := prepareResource(definition, document, CreateMode, "")
	record, _ := newRecord("tenant", "controller-a", "User", "existing-id", externalID, document, indexes, testTime)
	if err := store.Transact(context.Background(), func(transaction Transaction) error { return transaction.Create(record) }); err != nil {
		t.Fatal(err)
	}
	reconciler, _ := NewReconciler(store, registry, func() time.Time { return testTime }, bytes.NewReader(make([]byte, 32)))
	_, err := reconciler.Reconcile(context.Background(), ReconcileRequest{Scope: "tenant", Manager: "controller-b", Resources: []DesiredResource{{ResourceType: "User", ExternalID: "shared", Data: []byte(`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"other"}`)}}})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("cross-manager adoption = %v", err)
	}
}

func TestReconcilerRejectsDuplicateDesiredIdentity(t *testing.T) {
	registry, _ := NewRegistry(DefaultDefinitions())
	reconciler, _ := NewReconciler(NewMemoryStore(), registry, nil, nil)
	resource := DesiredResource{ResourceType: "User", ExternalID: "same", Data: []byte(`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"member"}`)}
	if _, err := reconciler.Reconcile(context.Background(), ReconcileRequest{Scope: "tenant", Manager: "controller", Resources: []DesiredResource{resource, resource}}); err == nil {
		t.Fatal("duplicate desired identity passed")
	}
}

func TestReconcilerFailureBoundaries(t *testing.T) {
	registry, _ := NewRegistry(DefaultDefinitions())
	if _, err := NewReconciler(nil, registry, nil, nil); err == nil {
		t.Fatal("nil store passed")
	}
	var nilReconciler *Reconciler
	if _, err := nilReconciler.Reconcile(context.Background(), ReconcileRequest{}); err == nil {
		t.Fatal("nil reconciler passed")
	}
	reconciler, _ := NewReconciler(NewMemoryStore(), registry, func() time.Time { return time.Time{} }, bytes.NewReader(make([]byte, 16)))
	if _, err := reconciler.Reconcile(context.Background(), ReconcileRequest{Scope: "tenant", Manager: "manager"}); err == nil {
		t.Fatal("zero clock passed")
	}
	reconciler, _ = NewReconciler(NewMemoryStore(), registry, func() time.Time { return testTime }, bytes.NewReader(nil))
	if _, err := reconciler.Reconcile(context.Background(), ReconcileRequest{Scope: "tenant", Manager: "manager", Resources: []DesiredResource{{ResourceType: "User", ExternalID: "one", Data: []byte(`{"schemas":["` + UserSchema + `"],"userName":"member"}`)}}}); err == nil {
		t.Fatal("entropy failure passed")
	}
	for index, desired := range []DesiredResource{
		{ResourceType: "Missing", ExternalID: "one", Data: []byte(`{}`)},
		{ResourceType: "User", ExternalID: "", Data: []byte(`{}`)},
		{ResourceType: "User", ExternalID: "one", Data: []byte(`{`)},
		{ResourceType: "User", ExternalID: "one", Data: []byte(`{"schemas":["` + UserSchema + `"]}`)},
	} {
		reconciler, _ = NewReconciler(NewMemoryStore(), registry, nil, nil)
		if _, err := reconciler.Reconcile(context.Background(), ReconcileRequest{Scope: "tenant", Manager: "manager", Resources: []DesiredResource{desired}}); err == nil {
			t.Errorf("invalid desired resource %d passed", index)
		}
	}
}
