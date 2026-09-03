# Judge pass 1 — rejected and repaired

The first cold review rejected a module-root compatibility facade. No released
tag or consumer pin established that import path, so it duplicated a large
security-sensitive API without preserving userspace.

Repair: remove the facade, retain exactly one public server package at
`pkg/scim`, preserve exact sentinel identities there, and keep the module root
for governance. Protocol, transaction, and scope mechanisms remain unchanged.
