# Alpha.3 library admission

## Scope and authority

This pass admits `pkg/scim` as the canonical package for GOTTH Board alpha.3.
It does not choose authentication, TLS, authorization, provisioning scopes,
durable storage, product identity policy, or session invalidation.

## Requirement traceability

| Requirement | Design/specification | Code | Verification |
|---|---|---|---|
| `SCIM-A3-01` | architecture and README layout | `pkg/scim/` | canonical outside-package test |
| `SCIM-A3-02` | implementation specification | `pkg/scim/scim.go` | sentinel/API contract tests |
| `SCIM-A3-03` | architecture authority split | server/store/password interfaces | negative and conformance tests |
| `SCIM-A3-04` | RFC matrix and specification | protocol/server/store code | RFC, boundary, fuzz, CheckStore tests |
| `SCIM-A3-05` | verification contract | tests/workflow evidence | clean clone, graph, two Judge passes |

## Runtime boundary

- Go 1.26.6 and RFC 7643/7644 are the reusable implementation authorities.
- JSON, resource, PATCH, Bulk, query, filter-depth, candidate, page, and
  reconciliation limits are pinned in the implementation specification.
- Completeness oracles include exact schema sets, canonical document/version,
  bounded candidate counts, transactional callback semantics, immediate delete
  visibility, immutable tombstones, and the exported `CheckStore` suite.
- Durable adapters require their own backend/version, restart, migration,
  concurrency, backup, and restore boundary evidence.

## Performance admission

No protocol algorithm, scan bound, transaction, HTTP, filter, sort, JSON, or
allocation mechanism changes. The canonical package is the original
implementation; the root owns governance only. No speedup is claimed, so
benchmark/Amdahl evidence is N/A for this structural admission.

## Failure and rollback

Rollback is a revert before the first consumer pin. The exact exported error
values retain their identity in the canonical package. The memory store remains
explicitly ephemeral, and no live identity provider or durable database is
touched.
