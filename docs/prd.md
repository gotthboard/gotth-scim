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
complete RFC 7644 filter grammar, sorting, GET and POST search, attribute
projection, PATCH, Bulk dependency handling, ETags, password delegation, and
the RFC 7643 Enterprise User extension. Every capability reported as supported
must implement its normative conditional requirements rather than a convenient
subset.

Non-goals: choosing bearer-token or TLS policy, mapping SCIM users to a
product's authorization model, storing plaintext passwords, sending
notifications, invalidating product sessions, implementing a product database
adapter, inventing vendor filter operators, or importing Mailu's email/alias
semantics.

## Alpha.3 admission requirements

- `SCIM-A3-01`: New consumers import the documented `pkg/scim` package.
- `SCIM-A3-02`: Exactly one public Go package exists and exact store-error
  sentinel identities remain stable within it.
- `SCIM-A3-03`: Package reorganization does not widen authentication, scope,
  storage, password, transaction, or HTTP authority.
- `SCIM-A3-04`: RFC limits and store/runtime contracts remain explicit and
  boundary-tested.
- `SCIM-A3-05`: Clean-clone, race, fuzz, canonical-consumer, graph, and two
  clean Judge passes gate alpha.3 admission.
