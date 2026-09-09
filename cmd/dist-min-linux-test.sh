#!/bin/bash
# Regression test for the minimum-Linux publish gate
# (cmd/verify-min-linux.sh and its wiring in cmd/rune/dist.sh).
#
# A release artifact whose ELF binary requires a newer glibc symbol
# version than the floor we advertise must never be published:
# the dynamic loader refuses to start it on a floor host ("version
# `GLIBC_2.29' not found"). This test packs a real ELF stub into a
# release-shaped tar and asserts:
#   1. verify-min-linux.sh aborts when a required version exceeds the floor
#   2. verify-min-linux.sh passes when every required version <= floor
#   3. cmd/rune/dist.sh fails closed when a linux publish omits the floor,
#      and aborts (before any publish) when it is set and the artifact
#      is over-floor.
#
# Linux only: it needs a C compiler to mint a real ELF and `file` to
# detect it. On other hosts it is a no-op skip.
#
# The exact VERNEED table is controlled by stubbing `readelf` on PATH so
# the comparison/fail-closed logic is exercised deterministically without
# depending on the host toolchain's actual glibc (mirroring how the macOS
# test controls `minos` precisely via clang -mmacosx-version-min).
set -u

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
gate="$repo_root/cmd/verify-min-linux.sh"
dist="$repo_root/cmd/rune/dist.sh"

cc=""
for c in cc gcc clang; do
    if command -v "$c" >/dev/null 2>&1; then cc="$c"; break; fi
done
if [[ "$(uname)" != "Linux" ]] || [[ -z "$cc" ]] || ! command -v file >/dev/null 2>&1; then
    echo "SKIP: min-linux gate test requires Linux with a C compiler and file." >&2
    exit 0
fi

work="$(mktemp -d "${TMPDIR:-/tmp}/dist-min-linux-XXXXXX")"
trap 'rm -rf "$work"' EXIT

floor_glibc="2.28"

# build_artifact <suffix> <out.tar.gz>
# Compiles a real ELF stub and packs it into a release-shaped tar
# (rune.app/bin/rune). The required symbol versions are not derived from
# this binary — the stubbed readelf below supplies them deterministically.
build_artifact() {
    local suffix="$1" out="$2"
    local tree="$work/tree-$suffix/rune.app/bin"
    mkdir -p "$tree"
    printf 'int main(void){return 0;}\n' >"$work/stub-$suffix.c"
    "$cc" -o "$tree/rune" "$work/stub-$suffix.c" \
        || { echo "FAIL: could not compile ELF stub" >&2; exit 1; }
    ( cd "$work/tree-$suffix" && tar -czf "$out" rune.app )
}

over="$work/over.tar.gz"
ok="$work/ok.tar.gz"
build_artifact over "$over"
build_artifact ok "$ok"

# A fake PATH whose readelf emits a controlled VERNEED table. The required
# versions are chosen by the RUNE_TEST_VERNEED env var ("over" or "ok") so
# a single stub drives every case deterministically. The stub mimics the
# real readelf -V layout the gate parses: a "Version needs section"
# followed by "Name: <SYMBOL>_<ver>" lines.
fakebin="$work/fakebin"
mkdir -p "$fakebin"
cat >"$fakebin/readelf" <<'EOF'
#!/bin/bash
# Only the gate's `readelf -V <file>` form is stubbed; anything else is an
# error so the test never silently exercises the wrong path.
if [[ "$1" != "-V" ]]; then
    echo "stub readelf: unexpected args: $*" >&2
    exit 2
fi
case "${RUNE_TEST_VERNEED:-ok}" in
    over)
        # GLIBC_2.29 is above the 2.28 floor.
        glibc="2.29" ;;
    *)
        # Exactly at floor.
        glibc="2.28" ;;
esac
cat <<TABLE

Version needs section '.gnu.version_r' contains 1 entry:
  000000: Version: 1  File: libc.so.6  Cnt: 3
  0x0010:   Name: GLIBC_ABI_DT_RELR  Flags: none  Version: 4
  0x0020:   Name: GLIBC_2.2.5  Flags: none  Version: 3
  0x0030:   Name: GLIBC_${glibc}  Flags: none  Version: 2
TABLE
EOF
chmod +x "$fakebin/readelf"

failures=0

echo "=== 1) gate aborts when a required version exceeds the floor ==="
out="$(PATH="$fakebin:$PATH" RUNE_TEST_VERNEED=over \
    bash "$gate" "$over" "$floor_glibc" 2>&1)"; status=$?
if [[ $status -eq 0 ]]; then
    echo "$out"; echo "FAIL: gate passed an over-floor artifact." >&2
    failures=$((failures + 1))
elif ! grep -q "above the advertised floor" <<<"$out"; then
    echo "$out"; echo "FAIL: gate aborted but not via the version check." >&2
    failures=$((failures + 1))
else
    echo "ok: gate aborted over-floor artifact (exit $status)"
fi

echo "=== 2) gate passes when every required version == floor ==="
out="$(PATH="$fakebin:$PATH" RUNE_TEST_VERNEED=ok \
    bash "$gate" "$ok" "$floor_glibc" 2>&1)"; status=$?
if [[ $status -ne 0 ]]; then
    echo "$out"; echo "FAIL: gate rejected an at-floor artifact." >&2
    failures=$((failures + 1))
else
    echo "ok: gate passed at-floor artifact"
fi

# Extend the fake PATH so gh/bluectl abort loudly: reaching either
# means a publish was attempted past the gate, which is itself the bug.
for tool in gh bluectl; do
    cat >"$fakebin/$tool" <<EOF
#!/bin/bash
echo "FAIL: $tool ran — the min-linux gate did not abort the publish." >&2
exit 99
EOF
    chmod +x "$fakebin/$tool"
done

# Hermetic tagged repo so dist.sh's check-release-tag passes and we reach
# the min-linux gate rather than tripping the tag guard first.
gitrepo="$work/repo"
mkdir -p "$gitrepo"
(
    cd "$gitrepo"
    git init -q
    git config user.email tester@example.com
    git config user.name tester
    git remote add origin https://example.com/rune.git
    git commit -q --allow-empty -m seed
    git tag v0.0.0
) || { echo "FAIL: could not seed hermetic git repo" >&2; exit 1; }

# run_dist <artifact> <verneed> [with-floor]
# Runs dist.sh against the artifact with publish tools and readelf stubbed
# out. <verneed> selects the controlled VERNEED table ("over"/"ok"). When
# a third arg is given the RUNE_MIN_GLIBC floor is exported; otherwise it
# stays unset so the fail-closed path is exercised.
run_dist() {
    (
        cd "$gitrepo" || exit 1
        export PATH="$fakebin:$PATH"
        export RUNE_TEST_VERNEED="$2"
        export BLUE_RELEASE_TAR="$1" BLUE_TARGET_OS=linux BLUE_TARGET_ARCH=amd64
        export RELEASE_REPO="example/rune" DOWNLOAD_HOST="https://example.com"
        if [[ $# -ge 3 ]]; then
            export RUNE_MIN_GLIBC="$floor_glibc"
        else
            unset RUNE_MIN_GLIBC
        fi
        bash "$dist" 2>&1
    )
}

echo "=== 3a) dist.sh fails closed when the floor is unset (linux) ==="
out="$(run_dist "$ok" ok)"; status=$?
if [[ $status -eq 0 ]]; then
    echo "$out"; echo "FAIL: dist.sh published without the min-linux floor." >&2
    failures=$((failures + 1))
elif grep -q "ran —" <<<"$out"; then
    echo "$out"; echo "FAIL: publish tool ran before the fail-closed check." >&2
    failures=$((failures + 1))
elif ! grep -q "RUNE_MIN_GLIBC not set" <<<"$out"; then
    echo "$out"; echo "FAIL: aborted, but not via the fail-closed check." >&2
    failures=$((failures + 1))
else
    echo "ok: dist.sh refused to publish without the min-linux floor (exit $status)"
fi

echo "=== 3b) dist.sh aborts an over-floor artifact before publishing ==="
out="$(run_dist "$over" over with-floor)"; status=$?
if [[ $status -eq 0 ]]; then
    echo "$out"; echo "FAIL: dist.sh published an over-floor artifact." >&2
    failures=$((failures + 1))
elif grep -q "ran —" <<<"$out"; then
    echo "$out"; echo "FAIL: publish tool ran before the min-linux gate." >&2
    failures=$((failures + 1))
elif ! grep -q "above the advertised floor" <<<"$out"; then
    echo "$out"; echo "FAIL: aborted, but not via the min-linux gate." >&2
    failures=$((failures + 1))
else
    echo "ok: dist.sh aborted over-floor publish (exit $status)"
fi

if [[ $failures -ne 0 ]]; then
    echo "FAILED: $failures min-linux gate check(s) did not behave as required." >&2
    exit 1
fi
echo "PASS: min-linux gate blocks over-floor artifacts and fails closed."
