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
set -e

DOWNLOADS_BUCKET="${DOWNLOADS_BUCKET:-gs://downloads.rune.build}"

GIT_TAG=$(git describe --tags --dirty)

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
echo "  https://${DOWNLOADS_BUCKET#gs://}/${gcs_arch}/${versioned}"
echo "  https://${DOWNLOADS_BUCKET#gs://}/${gcs_arch}/${latest}"
