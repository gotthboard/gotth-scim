# v0.1.0 Release Verification

Status: published GitHub Release absent; draft state unknown without GitHub authentication

Fresh closeout date: 2026-10-21

## Exact release identity

- Admitted runtime source: `255629e27f7df301d263a116fb37fd315ff54693`
- Annotated `v0.1.0` tag object:
  `c37ca6b934beebff73d389cd35fceb4bdf1cc1df`
- Peeled release commit: `11989aefa11d0fa11226313b6dffc3a4a0077fa5`
- Local, canonical Forgejo, and public GitHub refs advertised that exact tag
  object and peeled commit on 2026-10-21. The peeled commit was also `main` at
  both endpoints.
- The release commit changes release documentation and workflow records only;
  `pkg/`, `go.mod`, `go.sum`, `.go-version`, and `Makefile` have no delta
  from the admitted runtime source.

## Historical evidence reused

The dated admission evidence remains historical and was not rerun or relabeled
as fresh:

- the runtime source passed format, vet, race, and 90.1% statement-coverage
  gates;
- 2026-09-21 live evidence proved Authentik-to-GOTTH-Mail provisioning, exact
  external-ID projection, role binding, disable, session revocation, restore,
  restart persistence, backup, and isolated restore; and
- GOTTH Mail pinned `github.com/gotthboard/gotth-scim v0.1.0` without a local
  replacement.

## Fresh exact-tag evidence

On 2026-10-21, a clean public GitHub clone detached at the peeled commit passed
`make verify` on the development host with Go 1.26.6: formatting, read-only
module checks, vet, race-enabled tests, and the 90% coverage floor all passed
with 90.1% statement coverage.

An initial remote wrapper failed before any test because its shell expansion
targeted nonexistent local paths; that setup log is retained separately. A
clean external module then passed using the exact package import
`github.com/gotthboard/gotth-scim/pkg/scim@v0.1.0`. Two further consumer-fixture
setup attempts failed before the successful consumer run: one generated
malformed Go import syntax, and one omitted transitive `go.sum` entries.
Those were harness failures, not product or tag failures; their logs were kept
with the final passing log.

The deterministic source archive was reproduced successfully from the exact
tag. Release-asset checksums are:

- `gotth-scim-v0.1.0-source.tar.gz`:
  `90c6a496ba533b9936d2f14a7b3c3b126966f5e636dec27b00cf702b748dc608`
- `SHA256SUMS`:
  `c204438bf00913dffca219447c6c6632b8e028c258335a340e6a12c9c59db727`

## Publication boundary

The public tag fallback URL exists, but GitHub's authoritative release-by-tag
API returns 404: no published Release is visible. Create or publish a normal Release on
the existing immutable tag using
`docs/release-notes-v0.1.0.md` and the recorded source archive and checksum
manifest.

The release workflow remains active until the normal Release object and two
exact custom assets exist and the closeout check verifies metadata, names,
bytes, internal manifest, and tag identity. Never move or replace
the existing tag; any defect requires a new SemVer version.
