# v1.0.0 release admission

Prepare one stable release candidate for the existing `pkg/scim` library.
The feature is intentionally narrow: correct RFC 7644 negative-count paging,
freeze the public/support/error contract, and produce reproducible source-only
release evidence.

## In scope

1. Interpret negative GET and POST search `count` as zero while retaining
   accurate totals and every configured bound.
2. Freeze the exported v1 API and document support and error compatibility.
3. Add changelog, release notes, deterministic source archive, exact source
   manifest, checksums, provenance, SPDX SBOM, and license evidence.
4. Run focused checks locally; run race/repeat/coverage/clean-consumer and
   reproducibility gates on the isolated development host.
5. Obtain and remediate two fresh independent reviews.
6. Commit the exact local candidate without pushing, tagging, publishing, or
   deploying it.

## Stop boundary

This workflow stops at a locally committed, verified candidate. Pushing the
branch, creating `v1.0.0`, creating a hosted Release, uploading assets, or
updating a consumer requires separate authorization.
