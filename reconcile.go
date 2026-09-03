package scim

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"
)

const MaximumReconcileResources = 10000

// DesiredResource is one manager-owned desired SCIM resource.
type DesiredResource struct {
	ResourceType string
	ExternalID   string
	Data         []byte
}

// ReconcileRequest is one atomic desired-state operation.
type ReconcileRequest struct {
	Scope         string
	Manager       string
	Resources     []DesiredResource
	DeleteMissing bool
}

// ReconcileResult records deterministic counts for one committed operation.
type ReconcileResult struct {
	Created   int
	Updated   int
	Deleted   int
	Unchanged int
}

// Reconciler applies bounded manager-scoped desired state.
type Reconciler struct {
	store    Store
	registry *Registry
	clock    func() time.Time
	entropy  io.Reader
	mu       sync.Mutex
}

// NewReconciler constructs a reconciler. Nil clock and entropy use UTC system
// time and crypto/rand.Reader.
func NewReconciler(store Store, registry *Registry, clock func() time.Time, entropy io.Reader) (*Reconciler, error) {
	if store == nil || registry == nil {
		return nil, fmt.Errorf("reconciler store and registry are required")
	}
	if clock == nil {
		clock = time.Now
	}
	if entropy == nil {
		entropy = rand.Reader
	}
	return &Reconciler{store: store, registry: registry, clock: clock, entropy: entropy}, nil
}

// Reconcile validates all desired state before one atomic transaction. It
// never adopts or deletes resources carrying another manager marker.
func (reconciler *Reconciler) Reconcile(ctx context.Context, request ReconcileRequest) (ReconcileResult, error) {
	if reconciler == nil || request.Scope == "" || !validString(request.Scope, 1024) || request.Manager == "" || !validString(request.Manager, 1024) || len(request.Resources) > MaximumReconcileResources {
		return ReconcileResult{}, fmt.Errorf("reconciliation boundary is invalid")
	}
	type preparedDesired struct {
		definition ResourceDefinition
		externalID string
		document   Document
		indexes    []IndexKey
	}
	prepared := make([]preparedDesired, 0, len(request.Resources))
	seen := make(map[string]struct{}, len(request.Resources))
	for index, desired := range request.Resources {
		definition, exists := reconciler.registry.definitionByName(desired.ResourceType)
		if !exists || desired.ExternalID == "" || !validString(desired.ExternalID, maximumStringBytes) {
			return ReconcileResult{}, fmt.Errorf("desired resource %d identity is invalid", index)
		}
		key := desired.ResourceType + "\x00" + desired.ExternalID
		if _, duplicate := seen[key]; duplicate {
			return ReconcileResult{}, fmt.Errorf("desired external identity is duplicated")
		}
		seen[key] = struct{}{}
		document, err := DecodeDocument(desired.Data)
		if err != nil {
			return ReconcileResult{}, fmt.Errorf("desired resource %d: %w", index, err)
		}
		document["externalId"] = desired.ExternalID
		normalized, indexes, externalID, err := prepareResource(definition, document, CreateMode, "")
		if err != nil {
			return ReconcileResult{}, fmt.Errorf("desired resource %d: %w", index, err)
		}
		prepared = append(prepared, preparedDesired{definition: definition, externalID: externalID, document: normalized, indexes: indexes})
	}
	sort.Slice(prepared, func(i, j int) bool {
		if prepared[i].definition.Name == prepared[j].definition.Name {
			return prepared[i].externalID < prepared[j].externalID
		}
		return prepared[i].definition.Name < prepared[j].definition.Name
	})
	reconciler.mu.Lock()
	defer reconciler.mu.Unlock()
	now := reconciler.clock().UTC()
	if now.IsZero() {
		return ReconcileResult{}, fmt.Errorf("reconciliation clock returned zero time")
	}
	result := ReconcileResult{}
	err := reconciler.store.Transact(ctx, func(transaction Transaction) error {
		liveByKey := make(map[string]Record)
		managed := make(map[string]Record)
		tombstoned := make(map[string]struct{})
		for _, definition := range reconciler.registry.definitions() {
			records, err := transaction.List(Query{Scope: request.Scope, ResourceType: definition.Name, Limit: MaximumReconcileResources})
			if err != nil {
				return err
			}
			for _, record := range records {
				if record.ExternalID != "" {
					key := record.ResourceType + "\x00" + record.ExternalID
					if _, exists := liveByKey[key]; exists {
						return fmt.Errorf("store returned duplicate external identity")
					}
					liveByKey[key] = record
					if record.Manager == request.Manager {
						managed[key] = record
					}
				}
			}
			tombstones, err := transaction.Tombstones(request.Scope, definition.Name)
			if err != nil {
				return err
			}
			for _, tombstone := range tombstones {
				if tombstone.ExternalID != "" {
					tombstoned[tombstone.ResourceType+"\x00"+tombstone.ExternalID] = struct{}{}
				}
			}
		}
		for _, desired := range prepared {
			key := desired.definition.Name + "\x00" + desired.externalID
			if _, blocked := tombstoned[key]; blocked {
				return fmt.Errorf("%w: %s", ErrTombstoned, desired.externalID)
			}
			existing, exists := liveByKey[key]
			if exists && existing.Manager != request.Manager {
				return fmt.Errorf("%w: desired identity belongs to another manager", ErrConflict)
			}
			if !exists {
				id, err := NewResourceID(reconciler.entropy)
				if err != nil {
					return err
				}
				record, err := newRecord(request.Scope, request.Manager, desired.definition.Name, id, desired.externalID, desired.document, desired.indexes, now)
				if err != nil {
					return err
				}
				if err := transaction.Create(record); err != nil {
					return err
				}
				result.Created++
				continue
			}
			updated, changed, err := replacementRecord(existing, desired.externalID, desired.document, desired.indexes, now)
			if err != nil {
				return err
			}
			if !changed {
				result.Unchanged++
				delete(managed, key)
				continue
			}
			if err := transaction.Replace(updated, existing.Version); err != nil {
				return err
			}
			result.Updated++
			delete(managed, key)
		}
		if request.DeleteMissing {
			for _, record := range managed {
				tombstone := Tombstone{Scope: record.Scope, ResourceType: record.ResourceType, ID: record.ID, ExternalID: record.ExternalID, Manager: record.Manager, Version: record.Version, DeletedAt: now}
				if err := transaction.Delete(record.Scope, record.ResourceType, record.ID, record.Version, tombstone); err != nil {
					return err
				}
				result.Deleted++
			}
		}
		return nil
	})
	if err != nil {
		return ReconcileResult{}, err
	}
	return result, nil
}
