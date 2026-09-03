# gotth-scim

> **Distribution:** GitHub is the public clone, Go import, and future release endpoint.
> Forgejo remains canonical development and the issue/contribution location.
> See [the distribution contract](docs/distribution.md).


`gotth-scim` is a reusable, storage-neutral SCIM 2.0 server boundary for Go.

Canonical Go package: `github.com/gotthboard/gotth-scim/pkg/scim`. The
module root contains repository governance only; new consumers use the
canonical package.

Repository layout:

- `pkg/scim/` — public protocol/server/store implementation and hostile tests;
- module root — module metadata and repository governance;
- `docs/` — RFC, runtime, architecture, and admission contracts;
- `workflow/` — canonical feature state, review, and verification evidence.

It provides:

- strict, bounded, collision-safe JSON and protocol decoding;
- standard User, Group, and Enterprise User validation plus registered schema extensions;
- ServiceProviderConfig, ResourceTypes, Schemas, Users, Groups, and Bulk HTTP
  endpoints, including GET and POST `.search`;
- configurable strong or weak ETags and transactional HTTP preconditions;
- bounded full-expression filtering, sorting, projection, pagination, PATCH,
  and dependency-aware Bulk execution;
- optional atomic password delegation that never stores or returns plaintext;
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

See `docs/conformance.md` for the exact supported RFC surface and the
consumer-owned deployment requirements.

## Installation, compatibility, and support

Unreleased. The Go API is pre-1.0; the exact supported standards surface is
recorded in `docs/conformance.md`.

No post-migration version has been tagged. To inspect the current source
before the first admitted release:

```sh
go get github.com/gotthboard/gotth-scim@main
```

The repository has no selected license and no long-term support promise.
Versioning, release admission, security reporting, and contribution details are
in [the release policy](docs/RELEASING.md), [security policy](SECURITY.md), and
[contribution guide](CONTRIBUTING.md).
