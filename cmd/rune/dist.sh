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

GIT_TAG=$(git describe --tags --dirty)

# Reject anything that the in-product upgrader would refuse so we
# never publish a manifest that downgrade/version checks treat as
# garbage.
#
# Rules (kept in sync with ide/ideupgrade/manager.go):
#   - version must match canonical semver ("vMAJOR[.MINOR[.PATCH]][-pre][+build]")
#     as accepted by golang.org/x/mod/semver.
#   - no dirty/distance suffix from `git describe` (-dirty, -<N>-g<sha>):
#     those parse as pre-release labels and compare in surprising ways.
#   - download host must be https://; the client refuses anything else.
if [[ ! "$GIT_TAG" =~ ^v[0-9]+(\.[0-9]+){0,2}(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$ ]]; then
    echo "ERROR: tag '${GIT_TAG}' is not canonical semver (e.g. v1.2.3 or v1.2.3-beta.1)."
    echo "       The in-product upgrader (golang.org/x/mod/semver) will refuse it."
    exit 1
fi
case "$GIT_TAG" in
    *-dirty|*-dirty+*)
        echo "ERROR: tag '${GIT_TAG}' has a dirty suffix; commit or stash before publishing."
        exit 1
        ;;
esac
if [[ "$GIT_TAG" =~ -[0-9]+-g[0-9a-f]+$ ]]; then
    echo "ERROR: tag '${GIT_TAG}' looks like an untagged describe output (-N-g<sha>); tag the commit first."
    exit 1
fi
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

# Safety check: ensure the artifact does not contain source code.
case "$BLUE_RELEASE_TAR" in
	*.tar.gz)
		if tar -tzf "$BLUE_RELEASE_TAR" | grep -q '\.go$'; then
			echo "ERROR: artifact contains .go source files — aborting upload."
			tar -tzf "$BLUE_RELEASE_TAR" | grep '\.go$'
			exit 1
		fi
		;;
	*.dmg)
		temp_mount="$(mktemp -d "${TMPDIR:-/tmp}/dist-check-XXXXXX")"
		hdiutil attach -quiet -readonly "$BLUE_RELEASE_TAR" -mountpoint "$temp_mount"
		if find "$temp_mount" -name '*.go' -print -quit | grep -q .; then
			echo "ERROR: artifact contains .go source files — aborting upload."
			find "$temp_mount" -name '*.go'
			hdiutil detach -quiet "$temp_mount"
			rm -rf "$temp_mount"
			exit 1
		fi
		hdiutil detach -quiet "$temp_mount"
		rm -rf "$temp_mount"
		;;
esac

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
