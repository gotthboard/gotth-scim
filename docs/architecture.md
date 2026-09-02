# Architecture

The package stops at the protocol/storage boundary:

- schemas and error envelopes describe SCIM wire contracts;
- key normalization rejects case-fold collisions recursively;
- entity-tag parsing implements strong mutation and weak cache comparison;
- PATCH and Bulk decoders bound and validate generic envelopes;
- path parsing prevents external authorities, queries, fragments, traversal,
  encoded separators, and unresolved empty bulk references;
- ID generation supplies an opaque value that a resource adapter persists.

Resource adapters will later own attribute-specific PATCH behavior and one
atomic transaction. A server layer will map errors to `application/scim+json`.
Keeping those layers separate prevents protocol code from pretending a Flask
model or SQLAlchemy session is universal.
