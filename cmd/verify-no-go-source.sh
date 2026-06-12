#!/bin/bash
# Verify a release artifact carries no .go source files before it leaves
# this machine. Shared by every component's dist.sh (rune, rune-agent,
# runectl, fuzzy-search) as the final gate before publish.
#
# Usage: verify-no-go-source.sh <artifact>
#   <artifact> is a .tar.gz or .dmg. tar.gz contents are listed; dmg is
#   mounted read-only, scanned, and detached. Any .go entry aborts with
#   a non-zero exit so the caller never publishes the artifact.
set -e

artifact="$1"
if [[ -z "$artifact" ]]; then
    echo "ERROR: verify-no-go-source.sh requires an artifact path." >&2
    exit 1
fi
if [[ ! -e "$artifact" ]]; then
    echo "ERROR: artifact '$artifact' does not exist." >&2
    exit 1
fi

case "$artifact" in
    *.tar.gz|*.tgz)
        if tar -tzf "$artifact" | grep -q '\.go$'; then
            echo "ERROR: artifact '$artifact' contains .go source files — aborting upload." >&2
            tar -tzf "$artifact" | grep '\.go$' >&2
            exit 1
        fi
        ;;
    *.dmg)
        temp_mount="$(mktemp -d "${TMPDIR:-/tmp}/dist-check-XXXXXX")"
        hdiutil attach -quiet -readonly "$artifact" -mountpoint "$temp_mount"
        trap 'hdiutil detach -quiet "$temp_mount" >/dev/null 2>&1 || true; rm -rf "$temp_mount"' EXIT
        if find "$temp_mount" -name '*.go' -print -quit | grep -q .; then
            echo "ERROR: artifact '$artifact' contains .go source files — aborting upload." >&2
            find "$temp_mount" -name '*.go' >&2
            exit 1
        fi
        ;;
    *)
        echo "ERROR: verify-no-go-source.sh does not know how to inspect '$artifact'." >&2
        echo "       Expected a .tar.gz or .dmg artifact." >&2
        exit 1
        ;;
esac
