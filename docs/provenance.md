# Provenance

Protocol behavior was checked against:

- RFC 7643, SCIM Core Schema: <https://www.rfc-editor.org/rfc/rfc7643>
- RFC 7644, SCIM Protocol: <https://www.rfc-editor.org/rfc/rfc7644>

Failure cases were adapted from the Mailu SCIM work at source commit
`d9c4a122` in `/home/linus/development/mailu-scim-performance-hardening` on the
development host. Relevant lessons include case-equivalent key rejection,
exact schema sets, strong/weak conditional requests, bounded 100-operation
Bulk handling, safe `bulkId` references, immutable persistent IDs, tombstones,
transactional reconciliation, and cross-database behavior.

No Flask route, SQLAlchemy model/session, email-domain rule, alias semantics,
database query, or Mailu extension schema was copied into this repository.

The server contract was checked directly against RFC 7643 sections 2 through 4
and RFC 7644 sections 3.3 through 3.7. In particular, the implementation keeps
provider-issued IDs non-reassignable, treats `meta` as server-owned, scopes
`externalId` to the provisioning domain, uses 1-based pagination, and executes
Bulk operations in request order without falsely claiming request-wide
atomicity.
