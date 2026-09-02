# Product requirements

Provide common SCIM 2.0 protocol mechanics for multiple Go products without
requiring one database, resource model, tenant system, or web framework.
Malformed or ambiguous input must fail closed. Request sizes and operation
counts must be bounded. Conditional requests must distinguish strong If-Match
from weak If-None-Match comparison. Case-equivalent attributes must never be
silently selected. Resource IDs must be opaque and persistent.

Non-goals for the extraction baseline: complete User/Group schemas, an HTTP
router, bearer-token policy, storage adapters, reconciliation, deletion
tombstones, transaction policy, or Mailu email/alias behavior.
