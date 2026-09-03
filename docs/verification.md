# Verification

`make verify` checks the pinned Go toolchain, formatting, vet, race behavior
across every package, and a hard 90% statement-coverage floor for the canonical
`pkg/scim` implementation. `make verify-repeat` runs the entire
race-enabled suite 50 times.

The suite covers strict/collision-safe JSON, complete typed filter parsing,
schema and Enterprise User validation, mutability and returnability, canonical
errors, strong and weak entity tags, conditional requests, GET/POST search,
sorting, projection, PATCH, dependency-aware Bulk, transactional password
delegation, resource lifecycle, provisioning-scope isolation, persistent IDs,
uniqueness, rollback, defensive copies, tombstones, reconciliation ownership,
storage conformance, and external-package API compilation.

Fuzz targets exercise resource, PATCH, Bulk, filter, and SearchRequest
decoding. The admission run executes each target independently in addition to
its normal seed corpus.

Admission evidence for RFC-completion implementation commit
`1e6f2dfc007204e795326566f4e025d4b198cb04`:

- `make verify`: pass; statement coverage 90.1%;
- `make verify-repeat`: 50 race-enabled repetitions pass;
- clean clone `make verify`: pass;
- five fixed-count fuzz admissions: 50,028 generated resource, PATCH, Bulk,
  filter, and SearchRequest inputs total, all pass; and
- Graphify 0.9.32 code-only graph: 421 nodes, 1,340 edges, 12 communities,
  zero self-loops, zero duplicate relations, and zero same-endpoint collapse
  groups. Graph SHA-256:
  `5ec20107d9b268ad8c903ea72444afa2286cae9b70c7e1e7c7f63b78a3b51212`.

Durable database adapters remain consumer-owned and must run `CheckStore` plus
their own restart, migration, and database-concurrency integration tests. The
reference `MemoryStore` deliberately makes no durability claim.
