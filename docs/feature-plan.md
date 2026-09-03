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
