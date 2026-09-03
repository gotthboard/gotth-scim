package scim

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrNotFound     = errors.New("SCIM resource not found")
	ErrConflict     = errors.New("SCIM resource conflicts with existing state")
	ErrPrecondition = errors.New("SCIM resource precondition failed")
	ErrTombstoned   = errors.New("SCIM resource identity is tombstoned")
)

// IndexKey is one storage lookup and uniqueness key derived from validated
// resource data.
type IndexKey struct {
	Name      string
	Value     string
	CaseExact bool
	Unique    bool
}

// Record is the storage representation of one live SCIM resource. Data is
// canonical JSON without server-assigned id or meta attributes.
type Record struct {
	Scope        string
	ResourceType string
	ID           string
	ExternalID   string
	Manager      string
	Version      string
	Created      time.Time
	LastModified time.Time
	Data         []byte
	Indexes      []IndexKey
}

// Tombstone permanently reserves a deleted provider ID and records the
// provisioning ownership needed to prevent unsafe automatic recreation.
type Tombstone struct {
	Scope        string
	ResourceType string
	ID           string
	ExternalID   string
	Manager      string
	Version      string
	DeletedAt    time.Time
}

// Query is a bounded store query. An empty Attribute returns all live records
// in one scope and resource type.
type Query struct {
	Scope        string
	ResourceType string
	Attribute    string
	Value        string
}

// Store runs one callback in one atomic transaction. Implementations must call
// fn exactly once, commit only when fn returns nil, and never retry it.
type Store interface {
	Transact(context.Context, func(Transaction) error) error
}

// Transaction is the complete persistence contract used by Server and
// Reconciler. Returned values and accepted slices must be defensively copied.
type Transaction interface {
	Get(scope, resourceType, id string) (Record, error)
	List(Query) ([]Record, error)
	Create(Record) error
	Replace(Record, string) error
	Delete(scope, resourceType, id, expectedVersion string, tombstone Tombstone) error
	Tombstones(scope, resourceType string) ([]Tombstone, error)
}

func validateRecord(record Record) error {
	if record.Scope == "" || !validString(record.Scope, 1024) || !validName(record.ResourceType) || !validResourceID(record.ID) || !validString(record.Manager, 1024) || !validString(record.ExternalID, maximumStringBytes) {
		return fmt.Errorf("record identity is invalid")
	}
	if record.Version == "" || record.Created.IsZero() || record.LastModified.Before(record.Created) || len(record.Data) == 0 || len(record.Data) > MaximumResourceBytes {
		return fmt.Errorf("record metadata is invalid")
	}
	if _, err := parseSingleStrongTag(record.Version); err != nil {
		return fmt.Errorf("record version is invalid")
	}
	document, err := DecodeDocument(record.Data)
	if err != nil {
		return fmt.Errorf("record data is invalid: %w", err)
	}
	externalID, err := optionalString(document, "externalId", maximumStringBytes)
	if err != nil || externalID != record.ExternalID {
		return fmt.Errorf("record external identity does not match data")
	}
	expectedVersion, err := calculateRecordVersion(record)
	if err != nil || expectedVersion != record.Version {
		return fmt.Errorf("record version does not match canonical data")
	}
	seen := make(map[string]struct{}, len(record.Indexes))
	for _, index := range record.Indexes {
		if !validAttributeName(index.Name) || index.Value == "" || !validString(index.Value, maximumStringBytes) {
			return fmt.Errorf("record index is invalid")
		}
		folded := strings.ToLower(index.Name)
		if _, duplicate := seen[folded]; duplicate {
			return fmt.Errorf("record index is duplicated")
		}
		value, present, err := stringPath(document, index.Name)
		if err != nil || !present || value != index.Value {
			return fmt.Errorf("record index does not match canonical data")
		}
		seen[folded] = struct{}{}
	}
	return nil
}

func validResourceID(id string) bool {
	return id != "" && id != "." && id != ".." && id != "bulkId" && validString(id, 1024) && !strings.Contains(id, "/")
}

func cloneRecord(record Record) Record {
	record.Data = append([]byte(nil), record.Data...)
	record.Indexes = append([]IndexKey(nil), record.Indexes...)
	return record
}

func cloneTombstone(tombstone Tombstone) Tombstone { return tombstone }
