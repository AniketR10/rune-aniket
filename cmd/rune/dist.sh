#!/bin/bash
# Publish a Rune release artifact to the public GCS download bucket.
#
# Required environment:
#   BLUE_RELEASE_TAR    - path to the artifact (.tar.gz or .dmg)
#   BLUE_TARGET_OS      - target OS (e.g. linux, darwin)
#   BLUE_TARGET_ARCH    - target arch (e.g. amd64, arm64)
#
# Optional:
#   DOWNLOADS_BUCKET    - GCS bucket (default: gs://downloads.rune-editor.com)
set -e

DOWNLOADS_BUCKET="${DOWNLOADS_BUCKET:-gs://downloads.rune-editor.com}"

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
