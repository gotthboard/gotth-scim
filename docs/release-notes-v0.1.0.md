# gotth-scim v0.1.0

First immutable pre-1.0 release of the storage-neutral SCIM 2.0 protocol,
server, and adapter-conformance library used by GOTTH Mail.

## Install

```sh
go get github.com/gotthboard/gotth-scim/pkg/scim@v0.1.0
```

```go
import scim "github.com/gotthboard/gotth-scim/pkg/scim"
```

## Highlights

- RFC 7643/7644 resource validation, schemas, filters, PATCH, Bulk, discovery,
  sorting, projection, conditional requests, and canonical protocol errors.
- Transactional store boundary, persistent IDs, uniqueness, tombstones,
  reconciliation ownership, provisioning-scope isolation, and adapter
  conformance checks.
- Bounded request work, defensive copies, strict JSON handling, and hostile
  edge/fuzz coverage.
- Authentication, TLS, authorization, durable storage, product identity
  policy, and production database adapters remain consumer-owned.

This is an unstable pre-1.0 API. Verification used Go 1.26.6 and passed format,
vet, race-enabled tests, and 90.1% statement coverage plus an external
exact-package consumer. Historical GOTTH Mail evidence covers live Authentik
provisioning, binding, disable/revoke, restore, restart, backup, and isolated
restore.

## Source and checksum

- Annotated tag object: `c37ca6b934beebff73d389cd35fceb4bdf1cc1df`
- Peeled commit: `11989aefa11d0fa11226313b6dffc3a4a0077fa5`
- `gotth-scim-v0.1.0-source.tar.gz` SHA-256:
  `90c6a496ba533b9936d2f14a7b3c3b126966f5e636dec27b00cf702b748dc608`
