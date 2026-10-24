# Independent release-engineering review — remediation record

Fresh review of preliminary commit
`980803ebd62bb08cd0e96e947693a7e5a5360b58` returned five blockers:

1. SPDX timestamps used an offset rather than the required UTC `Z` form;
2. provenance mislabeled Git and Go directory digests;
3. repeated mutable `HEAD` reads could mix source identities;
4. destination publication used a check-then-ordinary-rename sequence; and
5. the workflow's focused command was descriptive text, not executable.

Remediation emits deterministic UTC timestamps, uses `gitCommit`,
`gitTree`, and hex `dirHash1` digests, derives all Git data from one
captured commit and rechecks `HEAD`, uses GNU `mv -T -n` plus an
unmoved-staging failure check for no-replace admission, and records the exact
focused command. The builder/documentation now also use resolvable source URLs,
omit a fake globally unique invocation identifier, and explain provenance's
non-self-referential checksum boundary.

The final exact commit requires a fresh independent re-review; this record does
not claim that the preliminary review admitted later changes.

## Second fresh pass

A fresh pass on `7b8728fe8c5ac0a2e7551791531c61648956237a` found that the
builder derived its repository from the caller's current directory rather than
the executing script. Remediation resolves `BASH_SOURCE[0]`, requires the
canonical repository script path, hashes the executing script against the
captured commit, and emits an explicit diagnostic when no-replace publication
loses a race. A later exact commit still requires a fresh pass.

## Third fresh pass

A fresh pass on `cbab6ee28eb29762450a1d5331a820b68c5f8749` found that
ambient Go workspace/user settings could redirect dependency inspection and
that the SLSA v1 predicate omitted its required `runDetails.metadata` object.
Remediation executes `go mod verify` and module inspection in an empty,
offline environment with `GOWORK=off`, `GOENV=off`, empty `GOFLAGS`, and
`GOTOOLCHAIN=local`; rejects module replacements or version drift; and emits
an honest empty metadata object. A later exact commit still requires a fresh
pass.

## Fourth fresh pass

A fresh pass on `db32f3d9c911f17ca1b883c5b89da83ea062c954` found
ambient Git archive/replacement state, unenforced Go version, an overclaimed SLSA
builder identity, and not-yet-sealed post-commit evidence. Remediation disables
replacement objects and external Git configuration, pins archive umask and
attributes, requires Go 1.26.6 Linux/amd64, replaces SLSA claims with an honest
project-defined in-toto predicate identifying an untrusted local builder, and
defines the source-bound external evidence directory that is sealed only after
the final commit exists. A later exact commit still requires a fresh pass.

## Fifth fresh pass

A fresh evidence-bound pass on
`2d732a8b201e398fa6b6525ab075572069f11315` found that the builder
accepted whichever clean `HEAD` invoked it and that the external gate records
were not yet wholly sealed. Remediation requires authorized full commit/tree
inputs and rechecks them before and after generation. The final harness records
exact commands, environments, timestamps, statuses, source cleanliness, and log
digests; uses an archive-derived external-consumer source copy; and seals every
evidence leaf under a top-level checksum/root. A later exact commit still
requires a fresh pass over the sealed evidence.
