# gotth-scim

`gotth-scim` is a storage-neutral SCIM 2.0 protocol kernel for Go. It provides
exact schema-set validation, collision-safe case normalization, canonical SCIM
errors, strong resource versions, If-Match/If-None-Match evaluation, bounded
PATCH and Bulk request decoding, safe Bulk paths, a bounded equality-filter
subset, and persistent opaque resource ID generation.

It is not yet a complete SCIM HTTP server. Applications must supply resource
models, attribute registries, authorization, transactions, persistence,
reconciliation, tombstones, and endpoint routing. Those pieces cannot be copied
from Mailu without importing Mailu's domain and database behavior.

This is a clean extraction of protocol invariants and failure cases learned
from the Mailu/Gophermailforge implementation, not a copy of its Flask and
SQLAlchemy storage layer.
