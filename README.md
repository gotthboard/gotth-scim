# gotth-scim

`gotth-scim` is a reusable, storage-neutral SCIM 2.0 server boundary for Go.
It provides:

- strict, bounded, collision-safe JSON and protocol decoding;
- standard User and Group validation plus registered schema extensions;
- ServiceProviderConfig, ResourceTypes, Schemas, Users, Groups, and Bulk HTTP
  endpoints;
- strong ETags and transactional HTTP preconditions;
- bounded equality filtering, pagination, PATCH, and ordered Bulk execution;
- opaque persistent IDs, immediate deletion visibility, and irreversible
  ownership-scoped tombstones;
- manager-scoped atomic reconciliation that preserves provider IDs; and
- a transaction-bearing storage interface, reference memory store, and
  exported adapter conformance check.

Applications supply authentication, an opaque provisioning-scope resolver, a
durable `Store` implementation, schema-extension validation, and product-side
identity/session behavior. `MemoryStore` is for tests and ephemeral use; it is
not fake durability.

```go
registry, _ := scim.NewRegistry(scim.DefaultDefinitions())
store := scim.NewMemoryStore()
handler, _ := scim.NewServer(scim.ServerConfig{
    Store:       store,
    Registry:    registry,
    ExternalURL: "https://example.org/scim/v2",
    ResolveScope: func(r *http.Request) (string, error) {
        return authenticatedProvisioningScope(r)
    },
    AuthenticationSchemes: []scim.AuthenticationScheme{{
        Type: "oauthbearertoken", Name: "Bearer", Description: "OAuth bearer token",
    }},
})
```

See `docs/conformance.md` for the exact supported RFC surface. Unsupported
optional behavior is rejected rather than silently ignored.
