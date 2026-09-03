package scim

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// MemoryStore is a concurrency-safe reference Store. It is intended for
// conformance tests and ephemeral deployments, not durable production state.
type MemoryStore struct {
	mu    sync.Mutex
	state memoryState
}

type memoryState struct {
	records    map[string]Record
	tombstones map[string]Tombstone
}

type memoryTransaction struct{ state *memoryState }

// NewMemoryStore returns an empty reference store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{state: newMemoryState()}
}

func newMemoryState() memoryState {
	return memoryState{records: make(map[string]Record), tombstones: make(map[string]Tombstone)}
}

// Transact implements Store with a private copy and atomic state replacement.
func (store *MemoryStore) Transact(ctx context.Context, fn func(Transaction) error) error {
	if store == nil || fn == nil {
		return fmt.Errorf("memory transaction callback is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	working := cloneMemoryState(store.state)
	transaction := &memoryTransaction{state: &working}
	if err := fn(transaction); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	store.state = working
	return nil
}

func cloneMemoryState(source memoryState) memoryState {
	result := newMemoryState()
	for key, record := range source.records {
		result.records[key] = cloneRecord(record)
	}
	for key, tombstone := range source.tombstones {
		result.tombstones[key] = cloneTombstone(tombstone)
	}
	return result
}

func recordKey(resourceType, id string) string { return resourceType + "\x00" + id }

func (transaction *memoryTransaction) Get(scope, resourceType, id string) (Record, error) {
	record, exists := transaction.state.records[recordKey(resourceType, id)]
	if !exists || record.Scope != scope {
		return Record{}, ErrNotFound
	}
	return cloneRecord(record), nil
}

func (transaction *memoryTransaction) List(query Query) ([]Record, error) {
	if query.Scope == "" || !validName(query.ResourceType) || query.Attribute != "" && !validAttributeName(query.Attribute) || query.Limit < 1 {
		return nil, fmt.Errorf("store query is invalid")
	}
	result := make([]Record, 0)
	for _, record := range transaction.state.records {
		if record.Scope != query.Scope || record.ResourceType != query.ResourceType {
			continue
		}
		if query.Attribute != "" && !recordMatches(record, query.Attribute, query.Value) {
			continue
		}
		result = append(result, cloneRecord(record))
		if len(result) > query.Limit {
			return nil, ErrTooMany
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func recordMatches(record Record, attribute, value string) bool {
	if strings.EqualFold(attribute, "externalId") {
		return record.ExternalID == value
	}
	for _, index := range record.Indexes {
		if strings.EqualFold(index.Name, attribute) {
			if index.CaseExact {
				return index.Value == value
			}
			return strings.EqualFold(index.Value, value)
		}
	}
	return false
}

func (transaction *memoryTransaction) Create(record Record) error {
	if err := validateRecord(record); err != nil {
		return err
	}
	key := recordKey(record.ResourceType, record.ID)
	for existingKey, existing := range transaction.state.records {
		if existing.ID == record.ID || existingKey == key {
			return ErrConflict
		}
	}
	for _, tombstone := range transaction.state.tombstones {
		if tombstone.ID == record.ID || record.ExternalID != "" && tombstone.Scope == record.Scope && tombstone.ResourceType == record.ResourceType && tombstone.ExternalID == record.ExternalID {
			return ErrTombstoned
		}
	}
	if transaction.hasIndexConflict(record, "") {
		return ErrConflict
	}
	transaction.state.records[key] = cloneRecord(record)
	return nil
}

func (transaction *memoryTransaction) Replace(record Record, expectedVersion string) error {
	if err := validateRecord(record); err != nil {
		return err
	}
	key := recordKey(record.ResourceType, record.ID)
	existing, exists := transaction.state.records[key]
	if !exists || existing.Scope != record.Scope {
		return ErrNotFound
	}
	if expectedVersion == "" || existing.Version != expectedVersion {
		return ErrPrecondition
	}
	if record.Created != existing.Created || record.Manager != existing.Manager {
		return fmt.Errorf("record creation time and manager are immutable")
	}
	if transaction.hasIndexConflict(record, key) {
		return ErrConflict
	}
	transaction.state.records[key] = cloneRecord(record)
	return nil
}

func (transaction *memoryTransaction) hasIndexConflict(record Record, excludedKey string) bool {
	for key, existing := range transaction.state.records {
		if key == excludedKey || existing.Scope != record.Scope || existing.ResourceType != record.ResourceType {
			continue
		}
		for _, candidate := range record.Indexes {
			for _, current := range existing.Indexes {
				if !strings.EqualFold(candidate.Name, current.Name) {
					continue
				}
				if candidate.Unique != current.Unique || candidate.CaseExact != current.CaseExact {
					return true
				}
				if !candidate.Unique {
					continue
				}
				if candidate.CaseExact && candidate.Value == current.Value || !candidate.CaseExact && strings.EqualFold(candidate.Value, current.Value) {
					return true
				}
			}
		}
	}
	return false
}

func (transaction *memoryTransaction) Delete(scope, resourceType, id, expectedVersion string, tombstone Tombstone) error {
	key := recordKey(resourceType, id)
	record, exists := transaction.state.records[key]
	if !exists || record.Scope != scope {
		return ErrNotFound
	}
	if expectedVersion == "" || record.Version != expectedVersion {
		return ErrPrecondition
	}
	if tombstone.Scope != scope || tombstone.ResourceType != resourceType || tombstone.ID != id || tombstone.DeletedAt.IsZero() || tombstone.Version != record.Version || tombstone.ExternalID != record.ExternalID || tombstone.Manager != record.Manager {
		return fmt.Errorf("tombstone does not match deleted resource")
	}
	if _, exists := transaction.state.tombstones[key]; exists {
		return ErrTombstoned
	}
	delete(transaction.state.records, key)
	transaction.state.tombstones[key] = cloneTombstone(tombstone)
	return nil
}

func (transaction *memoryTransaction) Tombstones(scope, resourceType string) ([]Tombstone, error) {
	if scope == "" || !validName(resourceType) {
		return nil, fmt.Errorf("tombstone query is invalid")
	}
	result := make([]Tombstone, 0)
	for _, tombstone := range transaction.state.tombstones {
		if tombstone.Scope == scope && tombstone.ResourceType == resourceType {
			result = append(result, cloneTombstone(tombstone))
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}
