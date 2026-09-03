# Coverage

The canonical behavior map is `workflow/artifacts/global-coverage-map.md`.
`make verify` runs RFC, server, store, concurrency, hostile, fuzz-seed, and
outside-package tests under `pkg/scim`, enforcing a 90% floor. Current statement
coverage is 90.1%; recorded gaps are defensive broken-adapter and deterministic
standard-library encoding failures.
