# GitHub Distribution Verification

Status: complete

## Identity and scope

- Pinned base: `efc58063f79dda57f5eadbcb695f3783903a506b`
- Publicly verified candidate: `f927c35f5b940987eb996a7ef3e44dde8781082a`
- Declared module: `github.com/gotthboard/gotth-scim`
- Runtime/API behavior: unchanged; this is a module-identity and distribution
  contract migration.

Exact stale-prefix searches found no old module or import identity in Go source,
`go.mod`, examples, or fixtures. Canonical Forgejo URLs remain only where the
development, issue, contribution, and security-reporting endpoints are stated.

## Verification

- Local `go mod tidy` produced no dependency drift.
- Local `go vet -mod=readonly ./...` passed.
- Local `go test -mod=readonly ./...` passed.
- On `development`, `make verify` passed with race coverage 90.1%.
- On `development`, `go test -mod=readonly -race -count=50 ./...` passed.
- The repository coverage threshold passed.
- A fresh public GitHub clone of `feature/github-distribution` resolved exact
  commit `f927c35f5b940987eb996a7ef3e44dde8781082a` and passed `go test -mod=readonly ./...`.
- A fresh external consumer compiled the public package through both direct VCS
  resolution and `https://proxy.golang.org,direct` at
  `v0.0.0-20260903060720-f927c35f5b94`.
- Complete Forgejo and GitHub advertised head/tag ref sets matched after the
  candidate push.
- A fresh public GitHub `main` clone resolved
  `dbf3a5240f8ebfe444f863f211184c4a22994faf`, produced no `go mod tidy` drift, and passed
  `go test -mod=readonly ./...`.
- Fresh external consumers resolved `@main` through direct VCS and
  `https://proxy.golang.org,direct`, then compiled at
  `v0.0.0-20260903062630-dbf3a5240f8e`.

The slash-containing feature ref is accepted by direct VCS resolution but is
not a legal version query for the module proxy. The pre-promotion proxy lane
therefore used the exact candidate pseudo-version above; both final `@main`
lanes passed after promotion.

## Impact graph

Graphify recorded 422 nodes / 1,541 edges at implementation commit. Graph SHA-256:
`921b79c2124427c67678f7f6f93ac93d2b47b6e92789e1ff14ff4809b0f73646`.
Subsequent commits before this record changed documentation only. The merged
suite graph had 4,116 nodes and 11,415 edges, with no
cross-repository module dependency edge.

## Admission and residual gates

Two cold Judge passes reviewed the completed candidate before promotion. This
completion update changes evidence and workflow state only and receives two
fresh cold passes before commit. No performance benchmark applies because
executable paths and data flow are unchanged.

No license was selected. Release tags remain blocked until Danny closes that
decision gate. GitHub metadata mutation lacks authentication. Forgejo is still
private, so unauthenticated public contribution and private vulnerability
reporting remain unresolved. Account conversion and ownership changes were not
performed.
