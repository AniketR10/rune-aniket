#!/bin/bash
# Report the released version of every package we own, per platform/arch.
#
# Language grammar packages are excluded, except for the toolchain packages
# listed in OWNED_LANGUAGES.
#
# Every reported Latest is then downloaded to /dev/null so a package whose
# Latest pointer resolves to a bundle with no artifact in the storage backend
# is caught here rather than by users running `pkg install`.
set -euo pipefail

CONFIG_ROOT="$(cd "$(dirname "$0")" && pwd)/bluectl"
PLATFORMS=(darwin-arm64 darwin-amd64 linux-amd64 linux-arm64)
OWNED_LANGUAGES='["go","rust","python"]'
ENV=prod
BROKEN_EXIT=3

usage() {
	cat <<EOF
Usage: $(basename "$0") [-e prod|staging] [-j]

  -e  bluectl config environment under deploy/bluectl. [default: prod]
  -j  Emit raw JSON lines ({platform,name,version}) instead of a table.
  -h  Display this message.

Exits $BROKEN_EXIT if any reported Latest has no downloadable artifact.
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

# verify downloads each row's Latest bundle to /dev/null and prints every
# version whose artifact is missing from the storage backend.
verify() {
	local platform name version broken=""
	while IFS=$'\t' read -r platform name version; do
		if [ "$version" = "-" ]; then
			continue
		fi
		if ! bluectl -c "$CONFIG_ROOT/$ENV/$platform" \
			release get "$name" "$version" /dev/null >/dev/null 2>&1; then
			broken="$broken$platform	$name	$version"$'\n'
		fi
	done < <(printf '%s\n' "$ROWS" | jq -r '[.platform, .name, .version] | @tsv')

	if [ -z "$broken" ]; then
		return 0
	fi

	cat >&2 <<-EOF

		################################################################################
		################################################################################
		##                                                                            ##
		##   WARNING: BROKEN RELEASES IN '$ENV'
		##                                                                            ##
		##   The packages below advertise a Latest version whose artifact cannot be    ##
		##   downloaded. Every install resolving to Latest WILL FAIL until either the  ##
		##   artifact is uploaded or Latest is repointed at a good version.            ##
		##                                                                            ##
		################################################################################
		################################################################################

	EOF
	{
		printf 'PLATFORM\tPACKAGE\tLATEST\n%s' "$broken" |
			column -t -s "$(printf '\t')"
		echo
	} >&2
	return "$BROKEN_EXIT"
}

ROWS="$(owned)"

if [ -n "${JSON:-}" ]; then
	printf '%s\n' "$ROWS"
else
	printf '%s\n' "$ROWS" |
		jq -s -r --argjson plats "$(printf '%s\n' "${PLATFORMS[@]}" | jq -R . | jq -s .)" '
			(["PACKAGE"] + $plats),
			(group_by(.name)[] | . as $g |
				[$g[0].name] + ($plats | map(. as $p | ($g | map(select(.platform == $p)) | .[0].version) // "-")))
			| @tsv
		' |
		column -t -s "$(printf '\t')"
fi

verify
