#!/usr/bin/env bash
# Reject commits that stage a submodule pointer the submodule's remote
# does not have. The orphaned SHA would be fetched on every
# `git submodule update`, breaking `make` for anyone who pulls the
# commit.
set -euo pipefail

# git runs commit hooks with GIT_INDEX_FILE (and friends) pointing at
# the parent repository's index, often as a relative path. Those
# variables must not leak into submodule invocations or every git -C
# call resolves them inside the submodule and fails, misreporting a
# reachable pin as unreachable.
subgit() {
    env -u GIT_INDEX_FILE -u GIT_DIR -u GIT_WORK_TREE git -C "$@"
}

staged=$(git diff --cached --raw | awk '$2 == "160000" || $1 ~ /160000/ { print $NF }' | sort -u)
[ -z "$staged" ] && exit 0

fail=0
for path in $staged; do
    sha=$(git ls-files --stage -- "$path" | awk '{print $2}')
    if [ -z "$sha" ]; then
        continue
    fi
    if ! subgit "$path" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
        echo "validate-submodule-pins: $path is not initialized; cannot verify $sha" >&2
        fail=1
        continue
    fi
    remote=$(subgit "$path" config --get branch."$(subgit "$path" symbolic-ref --short -q HEAD 2>/dev/null || echo main)".remote 2>/dev/null || echo origin)
    if ! subgit "$path" fetch --quiet "$remote" "$sha" 2>/dev/null; then
        echo "validate-submodule-pins: $path pinned to $sha which is not reachable on $remote" >&2
        echo "  Push the commit upstream or repoint the submodule to a reachable commit before committing." >&2
        fail=1
    fi
done

exit "$fail"
