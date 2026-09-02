# Verification

`make verify` checks formatting, vet, race, and coverage. Current tests cover
exact schema sets, canonical errors, case-collision rejection, recursive key
normalization, resource versions, strong and weak entity-tag behavior,
malformed tags, PATCH operation boundaries, Bulk method/count/path/bulkId
boundaries, encoded-separator rejection, equality-filter boundaries, and
opaque ID entropy failures.

A complete SCIM server remains gated on resource adapters, persistence,
tombstones, reconciliation, HTTP behavior, and a published RFC conformance
matrix. This repository does not claim those unimplemented layers pass.
