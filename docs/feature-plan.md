# Feature plan

1. Extract protocol invariants and hostile-input cases into a clean Go kernel.
2. Add an attribute registry and complete core User/Group validation.
3. Define a transaction-bearing resource adapter interface.
4. Implement PATCH/Bulk execution, persistent IDs, ETags, tombstones, and
   reconciliation against disposable stores.
5. Add an explicit HTTP layer and RFC conformance matrix.
6. Only then ship a provider adapter such as Authentik or a consuming product.

Steps 2 through 6 are not falsely marked complete by this extraction commit.
