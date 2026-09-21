# Changelog

This repository records user-visible and compatibility-relevant changes here.
Released sections use Semantic Versioning; unreleased work remains under
`Unreleased` and does not imply a tag.

## Unreleased

## v0.1.0 — 2026-09-21

Release source: `255629e27f7df301d263a116fb37fd315ff54693`

First immutable public release of the storage-neutral SCIM 2.0 protocol,
server, and adapter-conformance library. The release preserves the admitted
pre-1.0 API and RFC surface; it does not add runtime behavior over the release
source commit.

Compatibility effect:

- establishes `v0.1.0` as the first supported module pin
- keeps the API explicitly unstable until `v1.0.0`
- leaves authentication, TLS, authorization, durable storage, provisioning
  scope, and product identity policy with the consumer

Verification:

- format and read-only module checks
- vet, race tests, and 90.1% statement coverage
- clean external-consumer import
- live Authentik to GOTTH Mail lifecycle: provision, bind, disable, revoke,
  restore, restart, backup, and isolated restore
- exact Forgejo/GitHub commit and annotated-tag parity

Known limitations:

- the library is storage-neutral and does not ship a production identity
  provider or product database adapter
- no long-term compatibility promise exists before `v1.0.0`

### 2026-09-13 — Select the MIT license

Affected files:

- `LICENSE`
- `README.md`
- `CONTRIBUTING.md`
- `docs/distribution.md`
- `docs/CHANGELOG.md`
- `workflow.toml`
- `workflow/features/mit-license/`

Explanation:

Close the explicit legal decision gate by licensing the repository under MIT.
This changes distribution rights, not the Go API or runtime behavior.

Verification:

- canonical MIT text and copyright notice audit
- repository verification suite
- exact Forgejo/GitHub commit parity after admission

Risks / non-goals:

- no API, runtime behavior, dependency, tag, release, or deployment changes
- the first immutable release remains a separate admission step

### 2026-09-06 00:54 CDT — Route vulnerability reports through GitHub

Commit: current commit; hash assigned by Git after commit

Affected files:

- `README.md`
- `CONTRIBUTING.md`
- `SECURITY.md`
- `docs/distribution.md`
- `docs/CHANGELOG.md`
- `workflow/features/github-distribution/evidence/verification.md`

Explanation:

Keep Forgejo as the canonical development source while routing public bugs to
GitHub Issues and confidential vulnerability reports to GitHub private
security advisories. Remove the obsolete private Forgejo reporting path and
the retired `agents/` namespace.

Verification:

- documentation whitespace and stale-policy scans
- direct GitHub API confirmation that private vulnerability reporting is enabled

Risks / non-goals:

- no source code, public API, tag, release, deployment, or compatibility promise changed

### 2026-09-03 01:04 CDT — Structure and formally admit the alpha.3 library

Commit: `08854d0f6d23a062ad190637a6e387c6f70eebb3`

Affected files:

- `pkg/scim/`
- canonical outside-consumer API test
- `README.md`, `docs/`, `workflow.toml`, and admission evidence

Explanation:

Move the RFC-complete protocol/server/store implementation and hostile tests
out of the repository root, leave the root for governance, and add formal
coding-setup traceability, runtime, performance, review, and workflow records.

Verification:

- preliminary `go test ./...` passed after the move
- final race, fuzz, clean-clone, graph, and Judge evidence is recorded in the
  admission workflow evidence

Risks / non-goals:

- no authentication, TLS, durable store, identity system, tag, or deployment changed

### 2026-09-03 00:42 CDT — Establish GitHub public distribution

Commit: `5ba8966`

Affected files:

- `README.md`
- `CONTRIBUTING.md`
- `SECURITY.md`
- `docs/distribution.md`
- `docs/RELEASING.md`
- `go.mod` and repository-owned Go import references
- repository-owned Go source, tests, fixtures, and package documentation
- `workflow.toml` and `workflow/features/github-distribution/`

Explanation:

Declare GitHub as the public distribution endpoint while retaining Forgejo as
canonical development, define maturity and support honestly, and document the
independent release process. The Go module identity and exact self-imports move to the public GitHub path.

Verification:

- exact old-import search
- documentation contract audit
- `go mod tidy` drift check
- `go vet -mod=readonly ./...`
- `go test -mod=readonly ./...`

Risks / non-goals:

- No license is selected.
- No existing tag is changed and no new release is created.
- Mirror direction, repository ownership, and account type are unchanged.
