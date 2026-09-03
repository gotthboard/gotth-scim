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

Durable database adapters remain consumer-owned and must run `CheckStore` plus
their own restart, migration, and database-concurrency integration tests. The
reference `MemoryStore` deliberately makes no durability claim.
