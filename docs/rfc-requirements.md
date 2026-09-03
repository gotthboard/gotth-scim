# RFC 7643/7644 requirement matrix

This matrix is normative for project admission. “Optional” means the RFC lets a
provider advertise or omit the capability. Once advertised, every conditional
MUST or SHALL attached to that capability is mandatory here.

| RFC surface | Requirement | Admission target |
|---|---|---|
| Core User and Group | mandatory | complete schema, mutability, returnability, uniqueness, references, and primary-value behavior |
| Enterprise User | optional | built-in optional extension and discovery schema |
| Discovery | mandatory | exact ServiceProviderConfig, ResourceTypes, and Schemas behavior; optional metadata supported |
| Attribute projection | mandatory provider behavior | `attributes` and `excludedAttributes` on retrieval, query, and mutation responses |
| Filtering | optional capability | Figure 1 grammar, operators, precedence, typing, case, and complex value paths |
| Sorting | optional capability | `sortBy` and `sortOrder` with schema-aware ordering |
| Pagination | standard query behavior | bounded 1-based paging and accurate totals |
| POST search | optional query form | root and resource `/.search` SearchRequest endpoints |
| PUT | mandatory | replacement plus readOnly, immutable, and writeOnly rules |
| PATCH | optional capability | atomic full path semantics, primary normalization, mutability, projection, and no-op rules |
| DELETE | mandatory | immediate invisibility and permanent provider-ID tombstone |
| Bulk | optional capability | dependency resolution including forward and circular references, failOnErrors, and complete per-operation responses |
| Password change | optional capability | transactional consumer delegation; never persisted in resource JSON or returned |
| ETags | optional capability | strong default plus configurable weak validators and correct HTTP conditions |
| TLS and authentication | mandatory deployment boundary | consumer-owned and required by contract; never falsely implemented in the library |

Vendor-specific schemas, non-standard filter operators, alternative transport
mechanisms, and implementation optimizations are extension points, not RFC
features this project can honestly claim to implement generically.
