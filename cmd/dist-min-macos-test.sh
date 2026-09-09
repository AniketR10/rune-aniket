#!/bin/bash
# Regression test for the minimum-macOS publish gate
# (cmd/verify-min-macos.sh and its wiring in cmd/rune/dist.sh).
#
# A release artifact whose Mach-O binary demands a newer macOS than the
# floor we advertise must never be published: macOS refuses to launch
# it, producing "You can't use this version of the application with this
# version of macOS". This test compiles stub binaries with a known
# `minos` and asserts:
#   1. verify-min-macos.sh aborts when a binary's minos exceeds the floor
#   2. verify-min-macos.sh passes when every binary's minos <= floor
#   3. cmd/rune/dist.sh fails closed when a darwin publish omits
#      RUNE_MIN_MACOS, and aborts (before any publish) when it is set and
#      the artifact is over-floor.
#
# macOS only: it needs clang to mint Mach-O binaries and otool to read
# load commands. On other hosts it is a no-op skip.
set -u

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
gate="$repo_root/cmd/verify-min-macos.sh"
dist="$repo_root/cmd/rune/dist.sh"

if [[ "$(uname)" != "Darwin" ]] || ! command -v clang >/dev/null 2>&1 \
    || ! command -v otool >/dev/null 2>&1; then
    echo "SKIP: min-macos gate test requires macOS with clang and otool." >&2
    exit 0
fi

work="$(mktemp -d "${TMPDIR:-/tmp}/dist-min-macos-XXXXXX")"
trap 'rm -rf "$work"' EXIT

# build_artifact <dir-suffix> <minos> <out.tar.gz>
# Compiles a stub Mach-O for the given deployment target and packs it
# into a release-shaped tar (rune_darwin_arm64/rune).
build_artifact() {
    local suffix="$1" minos="$2" out="$3"
    local tree="$work/tree-$suffix/rune_darwin_arm64"
    mkdir -p "$tree"
    printf 'int main(void){return 0;}\n' >"$work/stub-$suffix.c"
    clang -arch arm64 -mmacosx-version-min="$minos" \
        -o "$tree/rune" "$work/stub-$suffix.c" \
        || { echo "FAIL: could not compile stub binary at minos $minos" >&2; exit 1; }
    ( cd "$work/tree-$suffix" && tar -czf "$out" rune_darwin_arm64 )
}

floor="13.3"
over="$work/over.tar.gz"
ok="$work/ok.tar.gz"
build_artifact over 26.0 "$over"
build_artifact ok "$floor" "$ok"

failures=0

echo "=== 1) gate aborts when minos ($over=26.0) exceeds floor ($floor) ==="
out="$(bash "$gate" "$over" "$floor" 2>&1)"; status=$?
if [[ $status -eq 0 ]]; then
    echo "$out"; echo "FAIL: gate passed an over-floor artifact." >&2
    failures=$((failures + 1))
elif ! grep -q "above the advertised floor" <<<"$out"; then
    echo "$out"; echo "FAIL: gate aborted but not via the minos check." >&2
    failures=$((failures + 1))
else
    echo "ok: gate aborted over-floor artifact (exit $status)"
fi

echo "=== 2) gate passes when minos == floor ($floor) ==="
out="$(bash "$gate" "$ok" "$floor" 2>&1)"; status=$?
if [[ $status -ne 0 ]]; then
    echo "$out"; echo "FAIL: gate rejected an at-floor artifact." >&2
    failures=$((failures + 1))
else
    echo "ok: gate passed at-floor artifact"
fi

# A fake PATH where gh/bluectl abort loudly: reaching either means a
# publish was attempted past the gate, which is itself the bug.
fakebin="$work/fakebin"
mkdir -p "$fakebin"
for tool in gh bluectl; do
    cat >"$fakebin/$tool" <<EOF
#!/bin/bash
echo "FAIL: $tool ran — the min-macos gate did not abort the publish." >&2
exit 99
EOF
    chmod +x "$fakebin/$tool"
done

# Hermetic tagged repo so dist.sh's check-release-tag passes and we reach
# the min-macos gate rather than tripping the tag guard first.
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

# run_dist <artifact> [floor]
# Runs dist.sh against the artifact with publish tools stubbed out. When
# a floor is given it is exported as RUNE_MIN_MACOS; when omitted the var
# stays unset so the fail-closed path is exercised.
run_dist() {
    (
        cd "$gitrepo" || exit 1
        export PATH="$fakebin:$PATH"
        export BLUE_RELEASE_TAR="$1" BLUE_TARGET_OS=darwin BLUE_TARGET_ARCH=arm64
        export RELEASE_REPO="example/rune" DOWNLOAD_HOST="https://example.com"
        if [[ $# -ge 2 ]]; then
            export RUNE_MIN_MACOS="$2"
        else
            unset RUNE_MIN_MACOS
        fi
        bash "$dist" 2>&1
    )
}

echo "=== 3a) dist.sh fails closed when RUNE_MIN_MACOS is unset (darwin) ==="
out="$(run_dist "$ok")"; status=$?
if [[ $status -eq 0 ]]; then
    echo "$out"; echo "FAIL: dist.sh published without RUNE_MIN_MACOS." >&2
    failures=$((failures + 1))
elif grep -q "ran —" <<<"$out"; then
    echo "$out"; echo "FAIL: publish tool ran before the fail-closed check." >&2
    failures=$((failures + 1))
elif ! grep -q "RUNE_MIN_MACOS is not set" <<<"$out"; then
    echo "$out"; echo "FAIL: aborted, but not via the fail-closed check." >&2
    failures=$((failures + 1))
else
    echo "ok: dist.sh refused to publish without RUNE_MIN_MACOS (exit $status)"
fi

echo "=== 3b) dist.sh aborts an over-floor artifact before publishing ==="
out="$(run_dist "$over" "$floor")"; status=$?
if [[ $status -eq 0 ]]; then
    echo "$out"; echo "FAIL: dist.sh published an over-floor artifact." >&2
    failures=$((failures + 1))
elif grep -q "ran —" <<<"$out"; then
    echo "$out"; echo "FAIL: publish tool ran before the min-macos gate." >&2
    failures=$((failures + 1))
elif ! grep -q "above the advertised floor" <<<"$out"; then
    echo "$out"; echo "FAIL: aborted, but not via the min-macos gate." >&2
    failures=$((failures + 1))
else
    echo "ok: dist.sh aborted over-floor publish (exit $status)"
fi

if [[ $failures -ne 0 ]]; then
    echo "FAILED: $failures min-macos gate check(s) did not behave as required." >&2
    exit 1
fi
echo "PASS: min-macos gate blocks over-floor artifacts and fails closed."
