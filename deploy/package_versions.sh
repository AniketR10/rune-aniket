#!/bin/bash
# Report the released version of every package we own, per platform/arch.
#
# Language grammar packages are excluded, except for the toolchain packages
# listed in OWNED_LANGUAGES.
set -euo pipefail

CONFIG_ROOT="$(cd "$(dirname "$0")" && pwd)/bluectl"
PLATFORMS=(darwin-arm64 darwin-amd64 linux-amd64 linux-arm64)
OWNED_LANGUAGES='["go","rust","python"]'
ENV=prod

usage() {
	cat <<EOF
Usage: $(basename "$0") [-e prod|staging] [-j]

  -e  bluectl config environment under deploy/bluectl. [default: prod]
  -j  Emit raw JSON lines ({platform,name,version}) instead of a table.
  -h  Display this message.
EOF
}

while getopts ":e:jh" opt; do
	case "$opt" in
	e) ENV="$OPTARG" ;;
	j) JSON=1 ;;
	h)
		usage
		exit 0
		;;
	*)
		usage >&2
		exit 2
		;;
	esac
done

collect() {
	local platform
	for platform in "${PLATFORMS[@]}"; do
		local config="$CONFIG_ROOT/$ENV/$platform"
		[ -d "$config" ] || {
			echo "missing bluectl config: $config" >&2
			exit 1
		}
		bluectl -c "$config" package list -F json |
			jq -c --arg platform "$platform" '
				{
					platform: $platform,
					name: .Name,
					version: (if (.Latest // "") == "" then "-" else .Latest end),
					language: (.Metadata.language == "true"),
				}
			'
	done
}

# Only the darwin-arm64 registry carries the language marker, so a package
# counts as a language grammar if any platform flags it as one.
owned() {
	collect | jq -s -c --argjson owned "$OWNED_LANGUAGES" '
		(map(select(.language) | .name) | unique) as $langs
		| map(select((.name | IN($langs[]) | not) or (.name | IN($owned[]))))
		| map(del(.language))
		| .[]
	'
}

if [ -n "${JSON:-}" ]; then
	owned
	exit 0
fi

owned |
	jq -s -r --argjson plats "$(printf '%s\n' "${PLATFORMS[@]}" | jq -R . | jq -s .)" '
		(["PACKAGE"] + $plats),
		(group_by(.name)[] | . as $g |
			[$g[0].name] + ($plats | map(. as $p | ($g | map(select(.platform == $p)) | .[0].version) // "-")))
		| @tsv
	' |
	column -t -s "$(printf '\t')"
