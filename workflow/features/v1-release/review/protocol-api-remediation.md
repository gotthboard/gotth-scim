# Independent protocol/API review — remediation record

Fresh review of preliminary commit
`980803ebd62bb08cd0e96e947693a7e5a5360b58` returned three blockers:

1. native-`int` parsing rejected otherwise valid integer `count` values outside
   machine range;
2. the compatibility table did not distinguish GET query `invalidValue`
   from malformed POST SearchRequest `invalidSyntax`; and
3. the search path compared `ErrTooMany` directly rather than with
   `errors.Is`.

Remediation parses wire counts as arbitrary-precision decimal integers before
normalizing negatives or clamping positives, adds extreme GET/POST regressions,
pins both malformed-input mappings, and accepts wrapped `ErrTooMany`. The
final exact commit requires a fresh independent re-review; this record does not
claim that the preliminary review admitted later changes.

## Second fresh pass

A fresh pass on `7b8728fe8c5ac0a2e7551791531c61648956237a` found that
mixed-case POST SearchRequest `Count` was not canonicalized and that the GET
error table was broader than the count-only guarantee. Remediation adds
`count`, `sortBy`, and `sortOrder` to the protocol key canonicalization set,
adds mixed-case negative/positive/null regressions, and narrows the error row to
GET `count`. A later exact commit still requires a fresh pass.

## Fourth fresh pass

A fresh pass on `db32f3d9c911f17ca1b883c5b89da83ea062c954` found that
putting SearchRequest-only keys in recursive `CoreKeyCases` rewrote valid custom
extension attributes. Remediation restores the global map, canonicalizes only
the top-level SearchRequest envelope, and adds a registered-extension regression
for `Count`, `SortBy`, and `SortOrder`. A later exact commit still requires
a fresh pass.

## Sixth fresh pass

A final sealed pass on `a8737b1fd7ec386b699c07c42d56fbc1236e3043`
identified superlinear `math/big` work for near-limit decimal counts. Remediation
replaces it with one linear sign/digit scan, magnitude length/lexical comparison,
and bounded-width conversion only after clamping. Near-1 MiB positive, negative,
leading-zero, and malformed regressions are included. A later exact commit still
requires a fresh sealed pass.
