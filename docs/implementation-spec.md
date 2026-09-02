# Implementation specification

- Implements protocol constants from RFC 7643 and RFC 7644.
- Schema sets compare case-insensitively but reject case-equivalent duplicates.
- Resource versions are quoted strong SHA-256 entity-tags over caller-provided
  canonical state, bounded to one MiB.
- If-Match uses strong comparison; If-None-Match uses weak comparison.
- PATCH payloads are at most one MiB; operation limit is caller-selected within
  1..1000; generic operations are add/remove/replace.
- Bulk payloads are at most one MiB and at most 100 operations.
- Bulk methods are POST, PUT, PATCH, and DELETE; paths are relative Users or
  Groups collection/resource paths.
- `bulkId:` references are separated for resolution against prior successful
  operations by the future execution layer.
- Equality filtering is intentionally one caller-selected `eq` attribute.
- Resource IDs contain 128 bits of caller-supplied entropy and must be stored.
