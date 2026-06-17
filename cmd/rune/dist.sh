#!/bin/bash
# Publish a Rune release artifact to the public GCS download bucket.
#
# Required environment:
#   BLUE_RELEASE_TAR    - path to the artifact (.tar.gz or .dmg)
#   BLUE_TARGET_OS      - target OS (e.g. linux, darwin)
#   BLUE_TARGET_ARCH    - target arch (e.g. amd64, arm64)
#
# Optional:
#   DOWNLOADS_BUCKET    - GCS bucket (default: gs://downloads.rune.build)
#   DOWNLOAD_HOST       - public HTTP origin used to print download URLs
#                         (default: https://<bucket-fqdn>). Use
#                         https://storage.googleapis.com/<bucket> for
#                         buckets that are NOT fronted by an HTTPS load
#                         balancer with a custom domain.
set -e

DOWNLOADS_BUCKET="${DOWNLOADS_BUCKET:-gs://downloads.rune.build}"
DOWNLOAD_HOST="${DOWNLOAD_HOST:-https://${DOWNLOADS_BUCKET#gs://}}"

# Reject anything that the in-product upgrader would refuse so we never
# publish a manifest that downgrade/version checks treat as garbage. The
# dist-* make rules also run this as their first prerequisite so the tag
# is validated before any build/notarize work. Sourcing here keeps the
# check authoritative even when dist.sh is invoked directly.
source "$(dirname "${BASH_SOURCE[0]}")/check-release-tag.sh"

# Download host must be https://; the client refuses anything else.
if [[ "$DOWNLOAD_HOST" != https://* ]]; then
    echo "ERROR: DOWNLOAD_HOST must be https://...; got '${DOWNLOAD_HOST}'. The in-product upgrader refuses non-HTTPS URLs."
    exit 1
fi

if [[ -z "${BLUE_RELEASE_TAR}" ]]; then
    echo "BLUE_RELEASE_TAR is not set. Pass the path to the release artifact."
    exit 1
fi

if [[ -z "${BLUE_TARGET_OS}" ]]; then
    echo "BLUE_TARGET_OS is not set. Pass the target OS (e.g. linux)."
    exit 1
fi

if [[ -z "${BLUE_TARGET_ARCH}" ]]; then
    echo "BLUE_TARGET_ARCH is not set. Pass the target arch (e.g. amd64)."
    exit 1
fi

# Final gate before publish: never ship .go source inside the artifact.
"$(dirname "${BASH_SOURCE[0]}")/../verify-no-go-source.sh" "$BLUE_RELEASE_TAR"

# Final gate before publish: never ship a macOS artifact whose minimum
# OS exceeds the floor we advertise. macOS enforces the binary's minos
# at launch, so a too-high floor (e.g. built on a newer SDK without
# pinning -mmacosx-version-min) makes the DMG refuse to open for users
# on supported releases. Fail-closed: a darwin publish without an
# explicit RUNE_MIN_MACOS is a configuration bug, not a reason to skip
# the check.
if [[ "$BLUE_TARGET_OS" == "darwin" ]]; then
    if [[ -z "${RUNE_MIN_MACOS}" ]]; then
        echo "ERROR: RUNE_MIN_MACOS is not set for a darwin publish — refusing to publish unverified." >&2
        echo "       The dist-darwin-* make rules set it from DARWIN_<arch>_MIN_MACOS." >&2
        exit 1
    fi
    "$(dirname "${BASH_SOURCE[0]}")/../verify-min-macos.sh" \
        "$BLUE_RELEASE_TAR" "$RUNE_MIN_MACOS"
fi

# Publish to public GCS bucket.
gcs_arch="${BLUE_TARGET_OS}-${BLUE_TARGET_ARCH}"
gcs_dir="${DOWNLOADS_BUCKET}/${gcs_arch}"

# Determine the public filename based on artifact extension.
case "$BLUE_RELEASE_TAR" in
	*.dmg)  versioned="Rune-${GIT_TAG}.dmg";  latest="Rune-latest.dmg"  ;;
	*)      versioned="rune-${GIT_TAG}.tar.gz"; latest="rune-latest.tar.gz" ;;
esac

echo "Publishing to ${gcs_dir}/${versioned} ..."
gsutil cp "$BLUE_RELEASE_TAR" "${gcs_dir}/${versioned}"
gsutil cp "$BLUE_RELEASE_TAR" "${gcs_dir}/${latest}"
echo "Public download URLs:"
echo "  ${DOWNLOAD_HOST}/${gcs_arch}/${versioned}"
echo "  ${DOWNLOAD_HOST}/${gcs_arch}/${latest}"

# Compute the SHA256 of the artifact using whichever tool is available.
# `shasum -a 256` is shipped on macOS; `sha256sum` is the GNU utility on Linux.
if command -v shasum >/dev/null 2>&1; then
	artifact_sha256=$(shasum -a 256 "$BLUE_RELEASE_TAR" | awk '{print $1}')
elif command -v sha256sum >/dev/null 2>&1; then
	artifact_sha256=$(sha256sum "$BLUE_RELEASE_TAR" | awk '{print $1}')
else
	echo "ERROR: neither shasum nor sha256sum is available — cannot compute checksum."
	exit 1
fi
artifact_size=$(wc -c < "$BLUE_RELEASE_TAR" | tr -d ' ')
published_at=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Source changelog from the top of CHANGELOG.md when present. The "top
# section" is everything from the first H2 heading up to (but not
# including) the next H2 heading, similar to how release notes are
# typically extracted.
changelog_json="\"\""
repo_root=$(git rev-parse --show-toplevel 2>/dev/null || echo ".")
changelog_file="${repo_root}/CHANGELOG.md"
if [[ -f "$changelog_file" ]]; then
	# Extract the first H2 section (## ...) using awk, then JSON-encode it.
	changelog_text=$(awk '
		/^## / {
			if (seen) { exit }
			seen = 1
		}
		seen { print }
	' "$changelog_file")
	if [[ -n "$changelog_text" ]]; then
		# JSON-encode by escaping backslashes, double quotes, and newlines.
		changelog_json=$(printf '%s' "$changelog_text" | python3 -c '
import json, sys
sys.stdout.write(json.dumps(sys.stdin.read()))
' 2>/dev/null || echo '""')
	fi
fi

# Write the release manifest. Rune clients fetch this object directly
# from the public downloads CDN at:
#   ${DOWNLOAD_HOST}/${gcs_arch}/manifest.json
# There is no server-side proxy; the file IS the manifest endpoint.
manifest_tmp="$(mktemp "${TMPDIR:-/tmp}/rune-manifest-XXXXXX.json")"
cat >"$manifest_tmp" <<EOF
{
  "version": "${GIT_TAG}",
  "commit": "$(git rev-parse --short HEAD)",
  "os": "${BLUE_TARGET_OS}",
  "arch": "${BLUE_TARGET_ARCH}",
  "filename": "${versioned}",
  "url": "${DOWNLOAD_HOST}/${gcs_arch}/${versioned}",
  "sha256": "${artifact_sha256}",
  "size": ${artifact_size},
  "published_at": "${published_at}",
  "changelog": ${changelog_json}
}
EOF

echo "Publishing manifest to ${gcs_dir}/manifest.json ..."
gsutil -h "Content-Type:application/json" -h "Cache-Control:max-age=300" \
	cp "$manifest_tmp" "${gcs_dir}/manifest.json"
rm -f "$manifest_tmp"
