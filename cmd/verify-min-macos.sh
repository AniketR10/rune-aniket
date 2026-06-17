#!/bin/bash
# Verify that a macOS release artifact's effective minimum OS version does
# not exceed the floor we advertise to users, before the artifact leaves
# this machine. Shared by cmd/rune/dist.sh as a publish gate alongside
# verify-no-go-source.sh.
#
# Why this exists: macOS enforces a binary's LC_BUILD_VERSION `minos` at
# launch (the "You can't use this version of the application with this
# version of macOS" error). That floor is set by the link line and is
# trivial to raise by accident — e.g. building on a newer SDK without
# pinning -mmacosx-version-min, which records the build host's OS as the
# floor. This gate fails the publish when any shipped Mach-O executable
# demands a newer macOS than the expected floor.
#
# Usage: verify-min-macos.sh <artifact> <expected-floor>
#   <artifact>        a .tar.gz or .dmg. tar.gz contents are extracted;
#                     dmg is mounted read-only, scanned, and detached.
#   <expected-floor>  the maximum allowed minimum-macOS (e.g. 13.3).
#
# Any executable whose `minos` is greater than <expected-floor> aborts
# with a non-zero exit so the caller never publishes the artifact. The
# scan only runs on macOS (needs otool/vtool); on other hosts it is a
# no-op pass, which is correct because non-darwin artifacts carry no
# Mach-O binaries.
set -e

artifact="$1"
expected="$2"
if [[ -z "$artifact" || -z "$expected" ]]; then
    echo "ERROR: usage: verify-min-macos.sh <artifact> <expected-floor>" >&2
    exit 1
fi
if [[ ! -e "$artifact" ]]; then
    echo "ERROR: artifact '$artifact' does not exist." >&2
    exit 1
fi
if [[ ! "$expected" =~ ^[0-9]+(\.[0-9]+)*$ ]]; then
    echo "ERROR: expected-floor '$expected' is not a version number." >&2
    exit 1
fi

# Without otool we cannot read Mach-O load commands. This only happens
# off macOS, where the artifact has no Mach-O binaries to check anyway.
if ! command -v otool >/dev/null 2>&1; then
    echo "verify-min-macos: otool unavailable (non-macOS host); skipping Mach-O minos scan."
    exit 0
fi

# ver_gt A B -> success (0) when A is strictly greater than B, using
# dotted-version ordering (so 13.10 > 13.3).
ver_gt() {
    [[ "$1" != "$2" ]] && [[ "$(printf '%s\n%s\n' "$1" "$2" | sort -V | tail -1)" == "$1" ]]
}

# binary_minos prints the highest LC_BUILD_VERSION/LC_VERSION_MIN_MACOSX
# `minos` across all slices of a Mach-O file, or nothing when the file
# carries no such load command (not a Mach-O, or a static-only object).
binary_minos() {
    otool -l "$1" 2>/dev/null \
        | awk '/LC_BUILD_VERSION|LC_VERSION_MIN_MACOSX/{f=1} f&&/minos|version/{print $2; f=0}' \
        | sort -V | tail -1
}

scan_dir() {
    local root="$1" worst="" worst_file="" f v
    # Mach-O executables in a release live under the unpacked tree
    # (rune_<os>_<arch>/*) or inside the app bundle (Rune.app/.../MacOS).
    # `file` keeps us from running otool over every resource.
    while IFS= read -r f; do
        case "$(file -b "$f" 2>/dev/null)" in
            *Mach-O*) ;;
            *) continue ;;
        esac
        v="$(binary_minos "$f")"
        [[ -z "$v" ]] && continue
        echo "  ${f#"$root"/}: minos $v"
        if [[ -z "$worst" ]] || ver_gt "$v" "$worst"; then
            worst="$v"
            worst_file="$f"
        fi
    done < <(find "$root" -type f)

    if [[ -z "$worst" ]]; then
        echo "ERROR: no Mach-O executables found in artifact '$artifact'." >&2
        echo "       Expected at least one binary to verify; refusing to publish blind." >&2
        exit 1
    fi
    if ver_gt "$worst" "$expected"; then
        echo "ERROR: '${worst_file#"$root"/}' requires macOS $worst, above the advertised floor $expected." >&2
        echo "       Publishing would break users on macOS $expected through $worst." >&2
        echo "       Pin the deployment target (-mmacosx-version-min / CMAKE_OSX_DEPLOYMENT_TARGET) and rebuild." >&2
        exit 1
    fi
    echo "verify-min-macos: OK — highest minos $worst <= floor $expected."
}

case "$artifact" in
    *.tar.gz|*.tgz)
        work="$(mktemp -d "${TMPDIR:-/tmp}/min-macos-XXXXXX")"
        trap 'rm -rf "$work"' EXIT
        tar -xzf "$artifact" -C "$work"
        scan_dir "$work"
        ;;
    *.dmg)
        mnt="$(mktemp -d "${TMPDIR:-/tmp}/min-macos-XXXXXX")"
        hdiutil attach -quiet -readonly "$artifact" -mountpoint "$mnt"
        trap 'hdiutil detach -quiet "$mnt" >/dev/null 2>&1 || true; rm -rf "$mnt"' EXIT
        scan_dir "$mnt"
        ;;
    *)
        echo "ERROR: verify-min-macos.sh does not know how to inspect '$artifact'." >&2
        echo "       Expected a .tar.gz or .dmg artifact." >&2
        exit 1
        ;;
esac
