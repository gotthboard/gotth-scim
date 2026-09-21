# v0.1.0 Release Admission

Publish the first immutable `gotth-scim` module version for the live GOTTH Mail
consumer. The release contains no API or runtime change over source commit
`255629e27f7df301d263a116fb37fd315ff54693`.

## Ordered work

1. Record the exact compatibility effect and limitations.
2. Run the complete repository gates on the clean candidate.
3. Admit the release metadata to canonical `main`.
4. Create one immutable annotated `v0.1.0` tag.
5. Verify exact Forgejo/GitHub tag parity and a clean public consumer import.

The tag is not movable. A defect after publication requires a new SemVer
version.
