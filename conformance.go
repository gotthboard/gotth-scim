package scim

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// CheckStore verifies the transaction, identity, copy, isolation, uniqueness,
// deletion, and tombstone requirements of a fresh Store implementation.
func CheckStore(ctx context.Context, factory func() Store) error {
	if factory == nil {
		return fmt.Errorf("store factory is required")
	}
	store := factory()
	if store == nil {
		return fmt.Errorf("store factory returned nil")
	}
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	first := conformanceRecord("scope-a", "id-one", "external-one", "Member", now)
	if err := store.Transact(ctx, func(transaction Transaction) error { return transaction.Create(first) }); err != nil {
		return fmt.Errorf("create: %w", err)
	}
	first.Data[0] = 'x'
	first.Indexes[0].Value = "mutated"
	if err := store.Transact(ctx, func(transaction Transaction) error {
		stored, err := transaction.Get("scope-a", "User", "id-one")
		if err != nil {
			return err
		}
		if string(stored.Data) != `{"externalId":"external-one","schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"Member"}` || stored.Indexes[0].Value != "Member" {
			return fmt.Errorf("store did not defensively copy create input")
		}
		stored.Data[0] = 'x'
		return errors.New("rollback marker")
	}); err == nil || err.Error() != "rollback marker" {
		return fmt.Errorf("callback error was not returned exactly")
	}
	if err := store.Transact(ctx, func(transaction Transaction) error {
		stored, err := transaction.Get("scope-a", "User", "id-one")
		if err != nil || stored.Data[0] != '{' {
			return fmt.Errorf("rollback or output copy failed: %w", err)
		}
		if _, err := transaction.Get("scope-b", "User", "id-one"); !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("scope isolation failed")
		}
		return nil
	}); err != nil {
		return err
	}
	conflict := conformanceRecord("scope-a", "id-two", "external-two", "member", now)
	if err := store.Transact(ctx, func(transaction Transaction) error { return transaction.Create(conflict) }); !errors.Is(err, ErrConflict) {
		return fmt.Errorf("case-insensitive uniqueness failed: %v", err)
	}
	if err := store.Transact(ctx, func(transaction Transaction) error {
		stored, err := transaction.Get("scope-a", "User", "id-one")
		if err != nil {
			return err
		}
		updated := cloneRecord(stored)
		updated.Data = []byte(`{"externalId":"external-one","schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"Updated"}`)
		updated.Indexes[0].Value = "Updated"
		updated.LastModified = now.Add(time.Second)
		updated.Version, err = calculateRecordVersion(updated)
		if err != nil {
			return err
		}
		if err := transaction.Replace(updated, `"wrong"`); !errors.Is(err, ErrPrecondition) {
			return fmt.Errorf("replace precondition failed: %v", err)
		}
		return transaction.Replace(updated, stored.Version)
	}); err != nil {
		return err
	}
	if err := store.Transact(ctx, func(transaction Transaction) error {
		stored, err := transaction.Get("scope-a", "User", "id-one")
		if err != nil {
			return err
		}
		tombstone := Tombstone{Scope: stored.Scope, ResourceType: stored.ResourceType, ID: stored.ID, ExternalID: stored.ExternalID, Manager: stored.Manager, Version: stored.Version, DeletedAt: now.Add(2 * time.Second)}
		return transaction.Delete(stored.Scope, stored.ResourceType, stored.ID, stored.Version, tombstone)
	}); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	if err := store.Transact(ctx, func(transaction Transaction) error {
		if _, err := transaction.Get("scope-a", "User", "id-one"); !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("deleted resource remained visible")
		}
		tombstones, err := transaction.Tombstones("scope-a", "User")
		if err != nil || len(tombstones) != 1 || tombstones[0].ID != "id-one" {
			return fmt.Errorf("tombstone persistence failed: %v", err)
		}
		reused := conformanceRecord("scope-a", "id-one", "external-new", "Other", now)
		if err := transaction.Create(reused); !errors.Is(err, ErrTombstoned) {
			return fmt.Errorf("deleted ID was reassigned: %v", err)
		}
		recreated := conformanceRecord("scope-a", "id-three", "external-one", "Other", now)
		if err := transaction.Create(recreated); !errors.Is(err, ErrTombstoned) {
			return fmt.Errorf("tombstoned external identity was recreated: %v", err)
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func conformanceRecord(scope, id, externalID, userName string, now time.Time) Record {
	record := Record{
		Scope: scope, ResourceType: "User", ID: id, ExternalID: externalID, Manager: "manager",
		Created: now, LastModified: now,
		Data:    []byte(fmt.Sprintf(`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":%q}`, userName)),
		Indexes: []IndexKey{{Name: "userName", Value: userName, Unique: true}},
	}
	if externalID != "" {
		record.Data = []byte(fmt.Sprintf(`{"externalId":%q,"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":%q}`, externalID, userName))
	}
	record.Version, _ = calculateRecordVersion(record)
	return record
}
