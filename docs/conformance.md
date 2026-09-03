# SCIM conformance matrix

This matrix describes implemented behavior. It is not a claim of complete RFC
7643/7644 coverage.

| Surface | Status | Boundary |
|---|---|---|
| Core User and Group resources | Supported | Standard required fields and admitted attribute shapes; registered extensions preserved |
| ServiceProviderConfig, ResourceTypes, Schemas | Supported | Generated from the configured registry |
| Create, retrieve, replace, delete | Supported | Transactional, scoped, versioned, tombstoned |
| PATCH | Supported subset | add/remove/replace; direct paths and one equality value-path filter |
| Bulk | Supported | POST/PUT/PATCH/DELETE, prior bulkId path/data references, failOnErrors; operations are independently transactional |
| Filtering | Supported subset | One `eq` expression over configured lookup attributes |
| Pagination | Supported | `startIndex` and `count`, bounded by configured maximum |
| ETags and HTTP preconditions | Supported | Strong If-Match, weak If-None-Match comparison |
| Sorting | Not supported | Requests are rejected; capability is false |
| Attribute projection | Not supported | `attributes` and `excludedAttributes` are rejected |
| Password change | Not supported | Password input is rejected; capability is false |
| Authentication | Consumer-owned | Required scope resolver runs after consumer authentication |
| Durable database | Adapter-owned | Exported transaction contract and conformance suite; memory store is reference/test only |
| Reconciliation | Supported | Bounded, externalId-based, exact-manager ownership, ID preserving, tombstone aware |

The source contracts are RFC 7643 and RFC 7644. Unsupported optional behavior
is rejected rather than silently ignored.
