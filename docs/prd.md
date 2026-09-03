# Product requirements

Provide a reusable SCIM 2.0 server boundary for Go products without requiring
one database, tenant model, bearer-token implementation, or product resource
model. The package must own protocol parsing, standard User and Group wire
validation, HTTP behavior, conditional requests, Bulk execution, tombstones,
and bounded reconciliation. Consumers must be able to supply a durable store
whose transaction behavior is verified by the package's conformance suite.

Malformed, ambiguous, oversized, or case-colliding input fails closed. Every
write is transaction-bearing. Resource IDs are opaque, persistent, globally
unique, and never reassigned. Deletion makes a resource immediately invisible
and leaves an immutable ownership-scoped tombstone. Reconciliation only
deletes resources bearing the exact manager marker supplied by its caller.

The server must remain honest about its supported RFC surface. It supports the
interoperable equality-filter and PATCH path subset documented in the
conformance matrix and rejects unsupported operations explicitly.

Non-goals: choosing bearer-token policy, mapping SCIM users to a product's
authorization model, storing passwords, sending notifications, invalidating
product sessions, implementing a product database adapter, or importing
Mailu's email/alias semantics.
