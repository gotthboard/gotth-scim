# Feature plan

1. Extract protocol invariants and hostile-input cases into a clean Go kernel.
2. Add a resource registry and bounded core User/Group validation.
3. Define and conformance-test a transaction-bearing storage interface and
   reference memory store.
4. Implement resource and Bulk execution, persistent IDs, versions,
   irreversible tombstones, and manager-scoped reconciliation.
5. Add the HTTP endpoints and publish an exact RFC conformance matrix.
6. Verify race behavior, coverage, clean-clone use, external adapter use, and
   code-graph structure before admission.

Provider-specific adapters such as Authentik and product-specific persistence
remain later consumer work. They do not belong in this repository's core.

RFC completion proceeds in dependency order:

1. Replace the equality-only parser with a bounded RFC filter AST and evaluator.
2. Add bounded multi-resource search, sorting, projection, and `/.search`.
3. Complete schema mutability/returnability, PATCH, Enterprise User, and
   transactional password handling.
4. Resolve forward/circular Bulk dependencies and exact response semantics.
5. Correct discovery behavior and expose optional metadata/ETag configuration.
6. Run the complete requirement matrix, external consumer, race, fuzz,
   clean-clone, and graph admission gates.
