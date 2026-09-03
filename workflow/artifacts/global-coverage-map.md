# Coverage map

| Surface | Evidence |
|---|---|
| Schema sets and SCIM errors | `schemas_test.go` |
| Entity tags and conditional requests | `etag_test.go` |
| Case-collision-safe normalization | `normalize_test.go` |
| PATCH envelope | `patch_test.go` |
| Bulk envelope and paths | `bulk_test.go` |
| Complete filter grammar, typing, logical and complex value paths | `filter_expression.go`, `rfc_complete_test.go`, `rfc_edge_test.go`, `fuzz_test.go` |
| Persistent opaque IDs | `id_test.go` |
| Strict JSON and duplicate-key rejection | `json_test.go`, `fuzz_test.go` |
| Resource registry and User/Group/Enterprise validation | `resource_test.go`, `edge_test.go`, `rfc_edge_test.go` |
| PATCH execution, complete value paths, primary normalization, and mutability | `patch_apply_test.go`, `edge_test.go`, `rfc_complete_test.go`, `rfc_edge_test.go` |
| Store transactions, copies, uniqueness, and tombstones | `memory_test.go`, `conformance.go` |
| HTTP discovery, CRUD, full search, sorting, projection, pagination, and strong/weak preconditions | `server_test.go`, `server_edge_test.go`, `rfc_complete_test.go`, `rfc_edge_test.go` |
| Password transaction capability and non-returnability | `rfc_complete_test.go`, `rfc_edge_test.go` |
| Dependency-aware Bulk execution, forward references, cycles, and locations | `server_test.go`, `server_edge_test.go`, `rfc_complete_test.go`, `rfc_edge_test.go` |
| Manager-scoped reconciliation | `reconcile_test.go` |
| External consumer API | `external_test.go` |

The remaining uncovered statements are defensive failures requiring a broken
Store/password adapter or failures from deterministic standard-library encoders.
Durable adapter restart/migration behavior is outside this repository and must
be covered by each adapter.
