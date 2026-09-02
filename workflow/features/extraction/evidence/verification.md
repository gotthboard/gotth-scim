# Protocol-kernel verification

Verified on 2026-09-02 with Go `go1.26.6-X:nodwarf5`:

- RFC 7643 and RFC 7644 checked at the RFC Editor
- Mailu source commit `d9c4a122` inspected on the development host
- `go vet -mod=readonly ./...`
- `go test -mod=readonly -race -cover ./...`
- statement coverage: 91.5%
- schema collision, ETag, conditional, PATCH, Bulk, path, bulkId order,
  object-shape, filter, and entropy failure paths exercised
- no Mailu Flask, SQLAlchemy, email-domain, alias, or database code copied

The extraction is a verified protocol kernel, not a complete SCIM server.
HTTP, resource adapters, persistence, tombstones, reconciliation, and an RFC
conformance matrix remain explicit future features and must not be represented
as completed work.
