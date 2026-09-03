# Verification

`make verify` checks the pinned Go toolchain, formatting, vet, race behavior,
and a hard 90% statement-coverage floor. `make verify-repeat` runs the entire
race-enabled suite 50 times.

The suite covers strict/collision-safe JSON, schema and extension validation,
canonical errors, entity tags and preconditions, PATCH paths and execution,
Bulk methods/references/failOnErrors, pagination and equality filters, HTTP
discovery and resource lifecycle, provisioning-scope isolation, persistent
IDs, uniqueness, transaction rollback, defensive copies, deletion visibility,
immutable tombstones, reconciliation ownership, manager isolation, no-op ID
preservation, storage conformance, and external-package API compilation.

Fuzz targets exercise resource, PATCH, and Bulk decoding. The admission run
executes each target independently in addition to its normal seed corpus.

Admission evidence for implementation commit
`afe8cd79e6e05761ce651afec75d7f6299062c1f`:

- `make verify`: pass; statement coverage 90.8%;
- `make verify-repeat`: 50 race-enabled repetitions pass;
- clean clone `make verify`: pass;
- three three-second fuzz admissions: 30,955 resource executions, 38,120
  PATCH executions, and 64,695 Bulk executions, all pass; and
- Graphify 0.9.32 code-only graph: 312 nodes, 931 edges, 12 communities,
  zero self-loops, zero duplicate relations, and zero same-endpoint collapse
  groups. Graph SHA-256:
  `00eaa3a636b2dbda303cf1c2533e046b7d1c972c17fd2c02ca40018cb7055a09`.

Durable database adapters remain consumer-owned and must run `CheckStore` plus
their own restart, migration, and database-concurrency integration tests. The
reference `MemoryStore` deliberately makes no durability claim.
