#!/usr/bin/env bash
#
# Verify that a commit message certifies the Developer Certificate of Origin
# (see ./DCO) with a Signed-off-by trailer matching the commit author.

set -euo pipefail

msg_file="${1:?usage: dco-signoff.sh COMMIT_MSG_FILE}"
subject="$(sed -n '/^[^#]/{p;q;}' "$msg_file")"

case "$subject" in
Merge\ * | fixup!\ * | squash!\ * | amend!\ *) exit 0 ;;
esac

ident="$(git var GIT_AUTHOR_IDENT)"
name="${ident%% <*}"
email="${ident#*<}"
email="${email%%>*}"
expected="Signed-off-by: ${name} <${email}>"

trailers="$(git interpret-trailers --parse "$msg_file")"

if printf '%s\n' "$trailers" | grep -qxF "$expected"; then
	exit 0
fi

found="$(printf '%s\n' "$trailers" | grep -i '^Signed-off-by:' || true)"

if [ -n "$found" ]; then
	echo "DCO sign-off does not match the commit author." >&2
	echo "  author: ${name} <${email}>" >&2
	printf '  found:  %s\n' "$found" >&2
else
	echo "Missing DCO sign-off (see CONTRIBUTING.md and ./DCO)." >&2
	echo "  expected: ${expected}" >&2
fi

echo >&2
echo "Commit with 'git commit -s', or amend with:" >&2
echo "  git commit -s --amend --no-edit" >&2
exit 1
