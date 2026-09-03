# Coverage map

| Surface | Evidence |
|---|---|
| Schema sets and SCIM errors | `schemas_test.go` |
| Entity tags and conditional requests | `etag_test.go` |
| Case-collision-safe normalization | `normalize_test.go` |
| PATCH envelope | `patch_test.go` |
| Bulk envelope and paths | `bulk_test.go` |
| Equality filter subset | `filter_test.go` |
| Persistent opaque IDs | `id_test.go` |
| Strict JSON and duplicate-key rejection | `json_test.go`, `fuzz_test.go` |
| Resource registry and User/Group validation | `resource_test.go`, `edge_test.go` |
| PATCH execution and value paths | `patch_apply_test.go`, `edge_test.go` |
| Store transactions, copies, uniqueness, and tombstones | `memory_test.go`, `conformance.go` |
| HTTP discovery, CRUD, filtering, pagination, and preconditions | `server_test.go`, `server_edge_test.go` |
| Ordered Bulk execution and references | `server_test.go`, `server_edge_test.go` |
| Manager-scoped reconciliation | `reconcile_test.go` |
| External consumer API | `external_test.go` |

The remaining uncovered statements are defensive failures requiring a broken
Store implementation or failures from deterministic standard-library encoders.
Durable adapter restart/migration behavior is outside this repository and must
be covered by each adapter.
