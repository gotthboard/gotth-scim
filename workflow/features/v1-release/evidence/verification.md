# v1.0.0 candidate verification

Status: local candidate; no push, tag, hosted Release, or deployment authorized

Base: canonical Forgejo `main` at
`9ee85cecde608fa2c0116eed6773f3f00de71317` (fresh fetch on 2026-10-24
showed no newer main commit).

## Fresh pre-commit evidence

- focused negative-count regression: RED against the base behavior, then PASS
  after the correction;
- GET and POST negative counts return empty pages with accurate totals;
- direct compiled-plan and configured-page-size regressions pass;
- malformed count and candidate-bound regressions remain enforced.

Exact clean-commit gates, asset digests, reproducibility, consumer proof, and
independent final review outcomes are sealed after the final commit identity
exists under:

`/home/linus/.openclaw/workspace/.artifacts/gotth-scim-v1-release-final-20261024`

That external directory is source-bound and checksummed. It avoids pretending
that a commit can contain evidence naming its own immutable hash.

## Historical evidence reused honestly

The v0.1.0 workflow records retain the prior RFC, race, coverage, exact-package
consumer, live Authentik/GOTTH Mail lifecycle, tag-parity, and source-archive
evidence. Those records are historical and are not relabeled as fresh v1
verification. The v1 implementation and release artifacts receive fresh gates.

## Publication boundary

The candidate remains local. `v1.0.0` does not exist until separately
authorized immutable publication completes.
