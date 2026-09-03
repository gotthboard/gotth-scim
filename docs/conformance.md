# SCIM conformance matrix

This matrix describes the implemented RFC 7643/7644 server boundary. TLS,
authentication, authorization, durable persistence, and vendor extensions are
deliberately supplied by consumers and must pass their own admission gates.

| Surface | Status | Boundary |
|---|---|---|
| Core User, Group, and Enterprise User resources | Supported | Standard required fields, mutability, returnability, primary normalization, and registered extensions |
| ServiceProviderConfig, ResourceTypes, Schemas | Supported | Generated from the configured registry |
| Create, retrieve, replace, delete | Supported | Transactional, scoped, versioned, tombstoned |
| PATCH | Supported | add/remove/replace, pathless operations, direct and complex value paths, complete filters, mutability enforcement |
| Bulk | Supported | POST/PUT/PATCH/DELETE, forward/path/data bulkId references, dependency cycles and failures, failOnErrors; operations are independently transactional |
| Filtering | Supported | Complete bounded RFC expression grammar, schema-aware operators, complex value paths, Unicode-aware comparison |
| Pagination | Supported | `startIndex` and `count`, bounded by configured maximum |
| GET and POST search | Supported | Resource and service-root `.search`, strict SearchRequest decoding, bounded candidate scans |
| ETags and HTTP preconditions | Supported | Strong by default; optional weak SCIM validators; correct conditional read/write handling |
| Sorting | Supported | Singular and multi-valued sub-attributes, primary selection, stable missing-value ordering |
| Attribute projection | Supported | `attributes` and `excludedAttributes`; `always`, `never`, and `request` returnability |
| Password change | Optional, supported | Explicit PasswordStore/PasswordTransaction capability, PRECIS preparation, atomic revision, no resource persistence or return |
| Public discovery | Optional, supported | Explicit configuration; resource endpoints remain authenticated |
| Authentication | Consumer-owned | Required scope resolver runs after consumer authentication |
| Durable database | Adapter-owned | Exported transaction contract and conformance suite; memory store is reference/test only |
| Reconciliation | Supported | Bounded, externalId-based, exact-manager ownership, ID preserving, tombstone aware |

The source contracts are RFC 7643 and RFC 7644. A deployment is not conformant
until its consumer supplies TLS, authentication, authorization scopes, and a
durable adapter whose restart, migration, concurrency, backup, and restore
evidence supplements `CheckStore`.
