package scim

import (
	"encoding/json"
	"fmt"
	"time"
)

func newRecord(scope, manager, resourceType, id, externalID string, document Document, indexes []IndexKey, now time.Time) (Record, error) {
	data, err := canonicalDocument(document)
	if err != nil {
		return Record{}, err
	}
	record := Record{Scope: scope, Manager: manager, ResourceType: resourceType, ID: id, ExternalID: externalID, Created: now.UTC(), LastModified: now.UTC(), Data: data, Indexes: append([]IndexKey(nil), indexes...)}
	record.Version, err = calculateRecordVersion(record)
	if err != nil {
		return Record{}, err
	}
	return record, validateRecord(record)
}

func replacementRecord(current Record, externalID string, document Document, indexes []IndexKey, now time.Time) (Record, bool, error) {
	data, err := canonicalDocument(document)
	if err != nil {
		return Record{}, false, err
	}
	if string(data) == string(current.Data) && externalID == current.ExternalID {
		return cloneRecord(current), false, nil
	}
	updated := cloneRecord(current)
	updated.Data = data
	updated.ExternalID = externalID
	updated.Indexes = append([]IndexKey(nil), indexes...)
	updated.LastModified = now.UTC()
	if !updated.LastModified.After(current.LastModified) {
		updated.LastModified = current.LastModified.Add(time.Nanosecond)
	}
	updated.Version, err = calculateRecordVersion(updated)
	if err != nil {
		return Record{}, false, err
	}
	return updated, true, validateRecord(updated)
}

func calculateRecordVersion(record Record) (string, error) {
	input := struct {
		ResourceType string          `json:"resourceType"`
		ID           string          `json:"id"`
		Data         json.RawMessage `json:"data"`
	}{ResourceType: record.ResourceType, ID: record.ID, Data: record.Data}
	canonical, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("encode resource version state: %w", err)
	}
	return Version(canonical)
}
