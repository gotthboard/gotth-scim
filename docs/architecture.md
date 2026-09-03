# Architecture

The package is split into five direct layers:

1. Protocol primitives validate schemas, collision-safe JSON, entity tags, the
   full RFC filter grammar, sorting, projection, GET/POST search, PATCH, Bulk,
   pagination, and SCIM error envelopes.
2. The resource registry describes standard User and Group schemas plus
   caller-supplied extension schemas. It validates wire shape and mutability
   before storage sees data.
3. `Store` and `Transaction` define the persistence boundary. Transactions are
   callback-scoped, never retried by the package, and cover precondition check
   plus mutation. `MemoryStore` is a deterministic reference implementation,
   not a durable production database.
4. `Server` supplies the SCIM HTTP endpoints and `Reconciler` supplies bounded,
   manager-scoped desired-state application. Both depend only on the registry
   and store contract.
5. Password input is write-only and can only cross the optional transactional
   password-writer interface. It is removed from resource JSON before storage
   and is never rendered or logged.

Stored records contain normalized canonical resource JSON, persistent identity,
timestamps, versions, provisioning scope, and an optional manager marker.
Metadata is generated on output; clients cannot overwrite it. Tombstones are
separate immutable records and are never returned by ordinary reads.

Authorization and TLS termination are deliberately outside the package. Required authentication
scheme metadata keeps discovery honest, while a required scope resolver
maps an authenticated request to an opaque provisioning scope. This prevents
the server from inventing bearer-token or tenant policy while ensuring every
storage call is isolated to one explicit scope.

The conformance harness is exported so a PostgreSQL, SQLite, or product-native
adapter must prove the same identity, uniqueness, transaction, visibility, and
tombstone behavior before use.
