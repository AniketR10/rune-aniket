#!/bin/bash
# Verify that a Linux release artifact never requires a glibc, libstdc++
# (GLIBCXX), or C++ ABI (CXXABI) symbol version newer than the floor we
# advertise to users, before the artifact leaves this machine. Shared by
# cmd/rune/dist.sh as a publish gate alongside verify-no-go-source.sh and
# verify-min-macos.sh.
#
# Why this exists: an ELF binary records the exact versioned symbols it
# needs in its VERNEED table (.gnu.version_r). The dynamic loader refuses
# to start a process when the host's libraries do not export a requested
# version ("version `GLIBC_2.38' not found"). That floor is trivial to
# raise by accident — e.g. a cgo/C++ dependency built against a newer
# toolchain pulls in GLIBC_2.29 or GLIBCXX_3.4.32, silently raising the
# effective runtime floor above what the build image (buster, glibc
# 2.28) guarantees. This gate fails the publish when any shipped ELF
# executable requires a newer version than the expected floor. It is the
# Linux analogue of the Mach-O minos check in verify-min-macos.sh.
#
# Usage: verify-min-linux.sh <artifact.tar.gz> <max-glibc> <max-glibcxx> <max-cxxabi>
#   <artifact.tar.gz>  the release tarball; its contents are extracted.
#   <max-glibc>        highest allowed GLIBC_ version  (e.g. 2.28).
#   <max-glibcxx>      highest allowed GLIBCXX_ version (e.g. 3.4.25).
#   <max-cxxabi>       highest allowed CXXABI_ version  (e.g. 1.3.11).
#
# Any ELF that requires a version above its family floor aborts with a
# non-zero exit so the caller never publishes the artifact. The scan only
# runs where readelf is available (Linux build/CI hosts); on other hosts
# it is a no-op pass, which is correct because non-Linux artifacts carry
# no ELF binaries to check.
set -e

artifact="$1"
max_glibc="$2"
max_glibcxx="$3"
max_cxxabi="$4"
if [[ -z "$artifact" || -z "$max_glibc" || -z "$max_glibcxx" || -z "$max_cxxabi" ]]; then
    echo "ERROR: usage: verify-min-linux.sh <artifact.tar.gz> <max-glibc> <max-glibcxx> <max-cxxabi>" >&2
    exit 1
fi
if [[ ! -e "$artifact" ]]; then
    echo "ERROR: artifact '$artifact' does not exist." >&2
    exit 1
fi
for v in "$max_glibc" "$max_glibcxx" "$max_cxxabi"; do
    if [[ ! "$v" =~ ^[0-9]+(\.[0-9]+)*$ ]]; then
        echo "ERROR: floor '$v' is not a version number." >&2
        exit 1
    fi
done

# Without readelf we cannot read the VERNEED table. This only happens off
# Linux, where the artifact has no ELF binaries to check anyway.
if ! command -v readelf >/dev/null 2>&1; then
    echo "verify-min-linux: readelf unavailable (non-Linux host); skipping ELF version scan."
    exit 0
fi

# ver_gt A B -> success (0) when A is strictly greater than B, using
# dotted-version ordering (so 3.4.32 > 3.4.30 and 2.10 > 2.9).
ver_gt() {
    [[ "$1" != "$2" ]] && [[ "$(printf '%s\n%s\n' "$1" "$2" | sort -V | tail -1)" == "$1" ]]
}

# needed_versions prints the dotted versions required by <file> for the
# given symbol family (GLIBC, GLIBCXX, or CXXABI), one per line. It reads
# only the "Version needs section" (.gnu.version_r) so that a bundled
# library's own *exported* version nodes (.gnu.version_d) never trip the
# gate — we care solely about what the shipped binary requires from the
# host. Names without a numeric version (e.g. GLIBC_ABI_DT_RELR) are
# skipped.
needed_versions() {
    local file="$1" family="$2"
    readelf -V "$file" 2>/dev/null | awk -v fam="$family" '
        /Version needs section/      { invn = 1 }
        /Version definitions section/ { invn = 0 }
        invn && match($0, "Name: " fam "_[0-9][0-9.]*") {
            tok = substr($0, RSTART, RLENGTH)
            sub("Name: " fam "_", "", tok)
            print tok
        }
    '
}

# max_needed prints the highest version required by <file> for <family>,
# or nothing when the file requires no versioned symbol from that family.
max_needed() {
    needed_versions "$1" "$2" | sort -V | tail -1
}

check_family() {
    local file="$1" family="$2" floor="$3" found
    found="$(max_needed "$file" "$family")"
    [[ -z "$found" ]] && return 0
    if ver_gt "$found" "$floor"; then
        echo "ERROR: '${file#"$root"/}' requires ${family}_${found}, above the advertised floor ${family}_${floor}." >&2
        echo "       Publishing would break users whose ${family} is older than ${found}." >&2
        echo "       Rebuild the offending dependency against the buster toolchain, or" >&2
        echo "       stop bundling a newer C++ runtime, so it requires <= ${family}_${floor}." >&2
        return 1
    fi
    return 0
}

root="$(mktemp -d "${TMPDIR:-/tmp}/min-linux-XXXXXX")"
trap 'rm -rf "$root"' EXIT
case "$artifact" in
    *.tar.gz|*.tgz) tar -xzf "$artifact" -C "$root" ;;
    *)
        echo "ERROR: verify-min-linux.sh does not know how to inspect '$artifact'." >&2
        echo "       Expected a .tar.gz artifact." >&2
        exit 1
        ;;
esac

found_elf=0
failures=0
while IFS= read -r f; do
    case "$(file -b "$f" 2>/dev/null)" in
        *ELF*) ;;
        *) continue ;;
    esac
    found_elf=1
    g="$(max_needed "$f" GLIBC)"
    gxx="$(max_needed "$f" GLIBCXX)"
    cxx="$(max_needed "$f" CXXABI)"
    echo "  ${f#"$root"/}: GLIBC ${g:-none} GLIBCXX ${gxx:-none} CXXABI ${cxx:-none}"
    check_family "$f" GLIBC   "$max_glibc"   || failures=$((failures + 1))
    check_family "$f" GLIBCXX "$max_glibcxx" || failures=$((failures + 1))
    check_family "$f" CXXABI  "$max_cxxabi"  || failures=$((failures + 1))
done < <(find "$root" -type f)

if [[ "$found_elf" -eq 0 ]]; then
    echo "ERROR: no ELF executables found in artifact '$artifact'." >&2
    echo "       Expected at least one binary to verify; refusing to publish blind." >&2
    exit 1
fi
if [[ "$failures" -ne 0 ]]; then
    exit 1
fi
echo "verify-min-linux: OK — required GLIBC <= $max_glibc, GLIBCXX <= $max_glibcxx, CXXABI <= $max_cxxabi."
