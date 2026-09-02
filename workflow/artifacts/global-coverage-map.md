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

HTTP, resource adapters, persistence, tombstones, and reconciliation are
explicit future features, not uncovered code in this extraction feature.
