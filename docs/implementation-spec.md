# Implementation specification

- Implements protocol constants from RFC 7643 and RFC 7644.
- Schema sets compare case-insensitively but reject case-equivalent duplicates.
- Resource versions use SHA-256 over canonical state, bounded to one MiB.
  Strong validators are the default; configured weak validators retain RFC
  comparison semantics.
- If-Match uses strong comparison by default. Weak-validator mode admits the
  weak SCIM If-Match form used by RFC 7644 examples; If-None-Match always uses
  weak comparison.
- PATCH payloads are at most one MiB; operation limit is caller-selected within
  1..1000; generic operations are add/remove/replace.
- Bulk payloads are at most one MiB and at most 100 operations.
- Bulk methods are POST, PUT, PATCH, and DELETE; paths are relative registered
  collection/resource paths.
- `bulkId:` references are separated for dependency resolution against
  declared successful operations, including forward references.
- Filtering implements RFC 7644 Figure 1 with bounded tokens/depth and typed,
  schema-aware comparison. Complex value-path filters are included.
- Search accepts GET query parameters at the service root and resource
  collections and SearchRequest POST bodies at matching `/.search` endpoints.
- Sorting supports `sortBy` and `sortOrder`; projection supports `attributes`
  and `excludedAttributes` while preserving attributes marked `always` and
  suppressing attributes marked `never`.
- Resource IDs contain 128 bits of caller-supplied entropy and must be stored.
- Dynamic JSON decoding rejects exact and case-equivalent duplicate object
  names, trailing data, invalid UTF-8, excessive depth, and excessive nodes.
- Standard User and Group definitions validate required identifiers, supported
  attribute shapes, read-only fields, extensions, and bounded string/array
  values. Password input is write-only and transactionally delegated.
- `Store.Transact` executes its callback exactly once and atomically commits
  only a nil callback result. Records and tombstones are isolated by an opaque
  provisioning scope; IDs remain globally non-reassignable.
- The reference memory store enforces User `userName` uniqueness and Group
  `displayName` uniqueness within a scope, copies all byte slices across the
  interface, and exposes no mutable internal maps.
- Every Store list request carries an enforced candidate limit. The generic
  server evaluates a compiled search plan over at most 10,000 canonical
  candidates; adapters may optimize without changing semantics.
- The HTTP layer provides ServiceProviderConfig, ResourceTypes, Schemas, Users,
  Groups, and Bulk under one configured base path. It emits
  `application/scim+json`, ETag, Location, and Content-Location consistently.
- Collection and root queries support bounded 1-based pagination, complete
  filters, sorting, and projection through GET and `/.search` POST.
- POST, PUT, PATCH, and DELETE evaluate preconditions inside the same storage
  transaction as their mutation. DELETE creates a tombstone and makes the
  resource immediately invisible.
- Bulk operations preserve response order and use dependency-aware execution.
  Each operation has its own transaction; successful `bulkId` values may be
  resolved in paths and data, including forward references. Circular
  dependencies fail explicitly without partially executing the cycle.
  `failOnErrors` stops further execution once the configured failure count is
  reached. Bulk is not falsely advertised as one atomic transaction.
- Reconciliation is bounded to 10,000 desired resources, requires non-empty
  external IDs and a manager marker, preserves IDs on update, ignores no-op
  state, and deletes only resources with the exact same manager marker.
- Adapter conformance checks callback rollback, identity persistence,
  uniqueness, isolation, defensive copies, deletion visibility, immutable
  tombstones, and non-reassignment.
- Password support is disabled unless the consumer supplies a `PasswordStore`
  whose transactions implement `PasswordTransaction`. The transaction returns an opaque
  credential revision incorporated into the resource version; cleartext is
  never placed in `Record.Data`.
