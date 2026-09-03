# Implementation specification

- Implements protocol constants from RFC 7643 and RFC 7644.
- Schema sets compare case-insensitively but reject case-equivalent duplicates.
- Resource versions are quoted strong SHA-256 entity-tags over caller-provided
  canonical state, bounded to one MiB.
- If-Match uses strong comparison; If-None-Match uses weak comparison.
- PATCH payloads are at most one MiB; operation limit is caller-selected within
  1..1000; generic operations are add/remove/replace.
- Bulk payloads are at most one MiB and at most 100 operations.
- Bulk methods are POST, PUT, PATCH, and DELETE; paths are relative registered
  collection/resource paths.
- `bulkId:` references are separated for resolution against prior successful
  operations by the server execution layer.
- Equality filtering is intentionally one caller-selected `eq` attribute.
- Resource IDs contain 128 bits of caller-supplied entropy and must be stored.
- Dynamic JSON decoding rejects exact and case-equivalent duplicate object
  names, trailing data, invalid UTF-8, excessive depth, and excessive nodes.
- Standard User and Group definitions validate required identifiers, supported
  attribute shapes, read-only fields, extensions, and bounded string/array
  values. Password storage is not supported.
- `Store.Transact` executes its callback exactly once and atomically commits
  only a nil callback result. Records and tombstones are isolated by an opaque
  provisioning scope; IDs remain globally non-reassignable.
- The reference memory store enforces User `userName` uniqueness and Group
  `displayName` uniqueness within a scope, copies all byte slices across the
  interface, and exposes no mutable internal maps.
- The HTTP layer provides ServiceProviderConfig, ResourceTypes, Schemas, Users,
  Groups, and Bulk under one configured base path. It emits
  `application/scim+json`, ETag, Location, and Content-Location consistently.
- Collection GET supports bounded 1-based pagination and one equality filter
  over the resource definition's configured lookup attributes. Unsupported
  sorting, projection, or filters fail explicitly.
- POST, PUT, PATCH, and DELETE evaluate preconditions inside the same storage
  transaction as their mutation. DELETE creates a tombstone and makes the
  resource immediately invisible.
- Bulk operations run in request order. Each operation has its own transaction;
  successful prior `bulkId` values may be resolved in later paths and data.
  `failOnErrors` stops further execution once the configured failure count is
  reached. Bulk is not falsely advertised as one atomic transaction.
- Reconciliation is bounded to 10,000 desired resources, requires non-empty
  external IDs and a manager marker, preserves IDs on update, ignores no-op
  state, and deletes only resources with the exact same manager marker.
- Adapter conformance checks callback rollback, identity persistence,
  uniqueness, isolation, defensive copies, deletion visibility, immutable
  tombstones, and non-reassignment.
