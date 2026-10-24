# Release Policy

`gotth-scim` versions independently under Semantic Versioning. A version number
in another GOTTH repository creates no release obligation here.

If this repository contains a Go module, release tags use the Go-compatible
`vMAJOR.MINOR.PATCH` form, including any SemVer prerelease suffix. Historical
tags are immutable even when they predate this convention.

No release may be tagged until the license decision gate is explicitly closed
and required verification is complete. Reserved placeholders are not tagged at
all.

For an admitted release:

1. Select a version from this repository's actual compatibility change.
2. Verify an exact clean Forgejo commit with the repository's required gates.
3. Update `docs/CHANGELOG.md` with the exact version, commit, compatibility
   effect, verification, and known limitations.
4. Create and push a new annotated tag on canonical Forgejo. Never move or
   replace an existing tag.
5. Verify exact head and tag parity at
   `github.com/gotthboard/gotth-scim` through the existing one-way push
   mirror.
6. Build any artifacts from the exact public GitHub clone, publish checksums,
   and create the GitHub release for that already-mirrored tag.

Forgejo remains the development source of truth. GitHub is the public clone,
Go import, and release-distribution endpoint. Failure of mirror or release
publication blocks the release; it does not authorize reversing mirror
direction or publishing a different commit.

## v1 stable release admission

A v1 candidate additionally requires:

1. `make verify` and `make verify-repeat` on the exact clean commit with
   Go 1.26.6;
2. a clean external consumer that compiles against the canonical
   `pkg/scim` package and exercises the exported sentinel-error contract;
3. two fresh independent reviews of implementation, compatibility promises,
   and release assets;
4. two independent runs of `scripts/build-release-candidate.sh` with the
   authorized full commit and tree IDs whose
   `SHA256SUMS` and every emitted byte match; and
5. validation that the source archive contains `LICENSE`, that license
   evidence matches the pinned module set under disabled workspace/user Go
   configuration and a verified offline module cache, that the SPDX document
   is valid JSON and names every linked module, and that every provenance
   subject has the same digest in the checksum manifest. The provenance file
   itself is checked by `SHA256SUMS` and cannot truthfully be its own subject.

The v1.0.0 asset set is the source archive, exact Git source manifest, combined
project/dependency license evidence, SPDX 2.3 SBOM, an unsigned in-toto
Statement using the project-defined `gotth-scim/provenance/v1` predicate, and
`SHA256SUMS`. The predicate records an explicitly untrusted local builder, the
exact recipe blob, normalized Go 1.26.6 Linux/amd64 environment, source commit
and tree, and verified module material. It deliberately makes no SLSA level,
hosted-build, operator-identity, or signature claim. Any publication signature
is an additional asset and must not change the candidate bytes.

Until the immutable tag and hosted Release are explicitly authorized, the
workflow remains a local candidate. Never describe candidate checks as
publication, and never rebuild different bytes under an existing version.

## Read-only release closeout

The tag fallback URL exists, but the authoritative GitHub release-by-tag API
currently reports no published Release; draft state is unknown without authentication:

```sh
EXPECT_STATE=incomplete sh scripts/release-closeout-check.sh
```

After creating or publishing the normal Release on that immutable tag and uploading the
source archive plus `SHA256SUMS`, require exact metadata and asset bytes:

```sh
EXPECT_STATE=complete sh scripts/release-closeout-check.sh
```

The check requires `git`, `curl`, `python3`, and `sha256sum`. A mismatch
never authorizes moving the tag or replacing tagged source.
