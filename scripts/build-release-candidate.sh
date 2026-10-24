#!/usr/bin/env bash
set -euo pipefail

version="v1.0.0"
project="gotth-scim"
xtext_version="v0.41.0"
xtext_license_sha256="911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad"
project_license_sha256="6e9b37c7c3e16d84193b2cf9b5c17fe51d56ce9dd3a70551e2615f8d794a6760"

if [[ $# -ne 3 ]]; then
	printf 'usage: %s OUTPUT_DIRECTORY EXPECTED_COMMIT EXPECTED_TREE\n' "$0" >&2
	exit 2
fi
for tool in git go gzip sha256sum tar awk cmp date env mv realpath; do
	command -v "$tool" >/dev/null || { printf 'required tool not found: %s\n' "$tool" >&2; exit 1; }
done

unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES
export GIT_NO_REPLACE_OBJECTS=1 GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null

script_path="$(realpath "${BASH_SOURCE[0]}")"
repo="$(git -C "$(dirname "$script_path")" rev-parse --show-toplevel)"
[[ "$script_path" == "$repo/scripts/build-release-candidate.sh" ]] || { printf 'release builder path is not canonical: %s\n' "$script_path" >&2; exit 1; }
cd "$repo"
if [[ -n "$(git status --porcelain=v1 --untracked-files=all)" ]]; then
	printf 'release source must be a clean checkout\n' >&2
	exit 1
fi
output_parent="$(realpath -m "$(dirname "$1")")"
output="$output_parent/$(basename "$1")"
expected_commit="$2"
expected_tree="$3"
[[ "$expected_commit" =~ ^[0-9a-f]{40}$ && "$expected_tree" =~ ^[0-9a-f]{40}$ ]] || { printf 'expected commit and tree must be full lowercase object IDs\n' >&2; exit 1; }
case "$output/" in
	"$repo"/*) printf 'output directory must be outside the source checkout\n' >&2; exit 1 ;;
esac
[[ ! -e "$output" ]] || { printf 'output path already exists: %s\n' "$output" >&2; exit 1; }
mkdir -p "$output_parent"
staging="$(mktemp -d "$output_parent/.${project}-${version}.XXXXXX")"
trap 'rm -rf "$staging"' EXIT

commit="$(git rev-parse HEAD)"
tree="$(git rev-parse "$commit^{tree}")"
[[ "$commit" == "$expected_commit" && "$tree" == "$expected_tree" ]] || { printf 'clean checkout does not match expected commit/tree\n' >&2; exit 1; }
created="$(date -u -d "@$(git show -s --format=%ct "$commit")" +%Y-%m-%dT%H:%M:%SZ)"
script_blob="$(git rev-parse "$commit:scripts/build-release-candidate.sh")"
[[ "$(git hash-object "$script_path")" == "$script_blob" ]] || { printf 'executing release builder does not match the exact commit\n' >&2; exit 1; }
[[ "$(git cat-file -t "$commit")" == commit ]]
[[ "$(git show "$commit:go.mod" | awk '$1 == "go" { print $2 }')" == 1.26.6 ]]
[[ "$(git show "$commit:go.mod" | awk '$1 == "require" && $2 == "golang.org/x/text" { print $3 }')" == "$xtext_version" ]]

prefix="${project}-${version}"
archive="${prefix}-source.tar.gz"
manifest="${prefix}-source-manifest.txt"
licenses="${prefix}-LICENSES.txt"
sbom="${prefix}.spdx.json"
provenance="${prefix}.intoto.jsonl"

git ls-tree -r --full-tree "$commit" >"$staging/$manifest"
git -c core.attributesFile=/dev/null -c tar.umask=0022 archive --format=tar --prefix="$prefix/" "$commit" | gzip -n -9 >"$staging/$archive"
tar -tzf "$staging/$archive" | awk -v prefix="$prefix/" 'index($0, prefix) != 1 { exit 1 }'
archive_license="$(mktemp "$staging/.license.XXXXXX")"
tar -xOf "$staging/$archive" "$prefix/LICENSE" >"$archive_license"
printf '%s  %s\n' "$project_license_sha256" "$archive_license" | sha256sum -c - >/dev/null
cmp "$archive_license" <(git show "$commit:LICENSE")
rm -f "$archive_license"

go_bin="$(realpath "$(command -v go)")"
go_clean=(env -i HOME="${HOME:?}" PATH="$(dirname "$go_bin"):/usr/bin:/bin" GOTOOLCHAIN=local GOWORK=off GOENV=off GOFLAGS= GOPROXY=off GOSUMDB=sum.golang.org "$go_bin")
[[ "$("${go_clean[@]}" env GOVERSION)" == go1.26.6 ]]
[[ "$("${go_clean[@]}" env GOOS)" == linux && "$("${go_clean[@]}" env GOARCH)" == amd64 ]]
"${go_clean[@]}" mod verify >/dev/null
IFS='|' read -r xtext_path resolved_xtext_version xtext_replacement xtext_dir < <("${go_clean[@]}" list -mod=readonly -m -f '{{.Path}}|{{.Version}}|{{if .Replace}}{{.Replace.Path}}{{end}}|{{.Dir}}' golang.org/x/text)
[[ "$xtext_path" == golang.org/x/text && "$resolved_xtext_version" == "$xtext_version" && -z "$xtext_replacement" ]]
[[ -n "$xtext_dir" && -f "$xtext_dir/LICENSE" ]]
printf '%s  %s\n' "$xtext_license_sha256" "$xtext_dir/LICENSE" | sha256sum -c - >/dev/null
{
	printf 'gotth-scim %s — MIT\n\n' "$version"
	git show "$commit:LICENSE"
	printf '\n---\n\ngolang.org/x/text %s — BSD-3-Clause\n\n' "$xtext_version"
	cat "$xtext_dir/LICENSE"
} >"$staging/$licenses"

archive_sha="$(sha256sum "$staging/$archive" | awk '{print $1}')"
manifest_sha="$(sha256sum "$staging/$manifest" | awk '{print $1}')"
licenses_sha="$(sha256sum "$staging/$licenses" | awk '{print $1}')"
cat >"$staging/$sbom" <<EOF
{
  "SPDXID": "SPDXRef-DOCUMENT",
  "spdxVersion": "SPDX-2.3",
  "dataLicense": "CC0-1.0",
  "name": "gotth-scim-v1.0.0-source",
  "documentNamespace": "https://github.com/gotthboard/gotth-scim/releases/tag/v1.0.0/sbom/$commit",
  "creationInfo": {
    "created": "$created",
    "creators": ["Tool: scripts/build-release-candidate.sh@$script_blob"]
  },
  "packages": [
    {
      "SPDXID": "SPDXRef-Package-gotth-scim",
      "name": "github.com/gotthboard/gotth-scim",
      "versionInfo": "v1.0.0",
      "downloadLocation": "git+https://github.com/gotthboard/gotth-scim.git@$commit",
      "filesAnalyzed": false,
      "checksums": [{"algorithm": "SHA256", "checksumValue": "$archive_sha"}],
      "licenseConcluded": "MIT",
      "licenseDeclared": "MIT",
      "copyrightText": "Copyright (c) 2026 GOTTH SCIM contributors",
      "externalRefs": [{"referenceCategory": "PACKAGE-MANAGER", "referenceType": "purl", "referenceLocator": "pkg:golang/github.com/gotthboard/gotth-scim@v1.0.0"}]
    },
    {
      "SPDXID": "SPDXRef-Package-golang-org-x-text",
      "name": "golang.org/x/text",
      "versionInfo": "$xtext_version",
      "downloadLocation": "https://proxy.golang.org/golang.org/x/text/@v/$xtext_version.zip",
      "filesAnalyzed": false,
      "licenseConcluded": "BSD-3-Clause",
      "licenseDeclared": "BSD-3-Clause",
      "copyrightText": "Copyright 2009 The Go Authors",
      "externalRefs": [{"referenceCategory": "PACKAGE-MANAGER", "referenceType": "purl", "referenceLocator": "pkg:golang/golang.org/x/text@$xtext_version"}]
    }
  ],
  "relationships": [
    {"spdxElementId": "SPDXRef-DOCUMENT", "relationshipType": "DESCRIBES", "relatedSpdxElement": "SPDXRef-Package-gotth-scim"},
    {"spdxElementId": "SPDXRef-Package-gotth-scim", "relationshipType": "DEPENDS_ON", "relatedSpdxElement": "SPDXRef-Package-golang-org-x-text"}
  ],
  "annotations": [{"annotationDate": "$created", "annotationType": "OTHER", "annotator": "Tool: scripts/build-release-candidate.sh@$script_blob", "comment": "License evidence asset SHA256: $licenses_sha"}]
}
EOF
sbom_sha="$(sha256sum "$staging/$sbom" | awk '{print $1}')"
cat >"$staging/$provenance" <<EOF
{"_type":"https://in-toto.io/Statement/v1","subject":[{"name":"$archive","digest":{"sha256":"$archive_sha"}},{"name":"$manifest","digest":{"sha256":"$manifest_sha"}},{"name":"$licenses","digest":{"sha256":"$licenses_sha"}},{"name":"$sbom","digest":{"sha256":"$sbom_sha"}}],"predicateType":"https://github.com/gotthboard/gotth-scim/provenance/v1","predicate":{"builder":{"id":"urn:gotth:builder:local-untrusted"},"recipe":{"uri":"https://github.com/gotthboard/gotth-scim/blob/$commit/scripts/build-release-candidate.sh","gitBlob":"$script_blob"},"environment":{"goVersion":"go1.26.6","goos":"linux","goarch":"amd64","moduleNetwork":"offline"},"materials":[{"uri":"git+https://git.dannyhunn.com/gotthboard/gotth-scim.git","digest":{"gitCommit":"$commit","gitTree":"$tree"}},{"uri":"pkg:golang/golang.org/x/text@$xtext_version","digest":{"dirHash1":"bf3fec780d259d7f3b3ad86ed9fff42f6e11720ad70fdfd81534ae1a38f7ac7f"}}]}}
EOF

(
	cd "$staging"
	sha256sum "$archive" "$manifest" "$licenses" "$sbom" "$provenance" >SHA256SUMS
	sha256sum -c SHA256SUMS
)
[[ "$(git rev-parse HEAD)" == "$expected_commit" ]]
[[ "$(git rev-parse "$expected_commit^{tree}")" == "$expected_tree" ]]
[[ -z "$(git status --porcelain=v1 --untracked-files=all)" ]]
mv -T -n "$staging" "$output"
if [[ -e "$staging" ]]; then
	printf 'output path appeared during publication: %s\n' "$output" >&2
	exit 1
fi
[[ -d "$output" ]]
trap - EXIT
printf 'release candidate assets: %s\n' "$output"
