# Server-boundary admission evidence

Implementation commit:
`afe8cd79e6e05761ce651afec75d7f6299062c1f`

## Completed scope

- strict standard User and Group resources with registered extensions;
- ServiceProviderConfig, ResourceTypes, Schemas, Users, Groups, and Bulk HTTP;
- transactional preconditions, persistent IDs, immediate delete visibility,
  immutable tombstones, and provisioning-scope isolation;
- storage contract, copy-on-write reference store, and exported adapter
  conformance check; and
- bounded exact-manager reconciliation with ID-preserving updates.

## Verification

- format: pass;
- `go vet -mod=readonly ./...`: pass;
- `go test -mod=readonly -race -coverprofile=coverage.out ./...`: pass,
  90.8% statement coverage;
- `go test -mod=readonly -race -count=50 ./...`: pass;
- clean-clone `make verify`: pass;
- resource/PATCH/Bulk fuzz admissions: 133,770 executions total, pass; and
- Graphify 0.9.32: 312 nodes, 931 edges, 12 communities, no self-loops,
  duplicate relations, or same-endpoint collapse groups.

## Explicit limits

Authentication and provisioning-scope derivation remain consumer-owned.
Durable adapters must run `CheckStore` and their own restart, schema-migration,
database-concurrency, backup, and restoration tests. The reference
`MemoryStore` is intentionally ephemeral. Sorting, attribute projection,
password change, and the full SCIM filter grammar are not advertised.

No live identity provider, database, deployment, or GOTTH Board state was
read or changed during this feature.
