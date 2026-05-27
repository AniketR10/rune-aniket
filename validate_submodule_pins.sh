#!/usr/bin/env bash
# Reject commits that stage a submodule pointer the submodule's remote
# does not have. The orphaned SHA would be fetched on every
# `git submodule update`, breaking `make` for anyone who pulls the
# commit.
set -euo pipefail

staged=$(git diff --cached --raw | awk '$2 == "160000" || $1 ~ /160000/ { print $NF }' | sort -u)
[ -z "$staged" ] && exit 0

fail=0
for path in $staged; do
    sha=$(git ls-files --stage -- "$path" | awk '{print $2}')
    if [ -z "$sha" ]; then
        continue
    fi
    if ! git -C "$path" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
        echo "validate-submodule-pins: $path is not initialized; cannot verify $sha" >&2
        fail=1
        continue
    fi
    remote=$(git -C "$path" config --get branch."$(git -C "$path" symbolic-ref --short -q HEAD 2>/dev/null || echo main)".remote 2>/dev/null || echo origin)
    if ! git -C "$path" fetch --quiet "$remote" "$sha" 2>/dev/null; then
        echo "validate-submodule-pins: $path pinned to $sha which is not reachable on $remote" >&2
        echo "  Push the commit upstream or repoint the submodule to a reachable commit before committing." >&2
        fail=1
    fi
done

exit "$fail"