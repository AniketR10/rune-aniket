#!/bin/bash
# Query the latest published Rune version from the per-arch manifest
# assets on the public GitHub releases.
#
# The release pipeline (cmd/rune/dist.sh) publishes one manifest per
# "<os>-<arch>" target at:
#   <host>/manifest-<os>-<arch>.json
#
# This script fetches each manifest and prints version, commit, and
# publish time so you can see the latest version across all targets.
#
# Usage:
#   query-version.sh [-e prod|staging] [-a <os>-<arch>] [-j]
#
# Options:
#   -e ENV    Release channel: prod (default) or staging.
#   -a ARCH   Query a single "<os>-<arch>" target instead of all.
#   -j        Emit raw JSON manifests instead of a formatted table.
#   -h        Show this help.
#
# Environment overrides:
#   DOWNLOAD_HOST   Override the manifest host entirely (https://...).
set -euo pipefail

PROD_HOST="https://github.com/unstablebuild/rune/releases/latest/download"
STAGING_HOST="https://github.com/unstablebuild/rune-staging/releases/latest/download"

ALL_ARCHES=(darwin-arm64 darwin-amd64 linux-amd64 linux-arm64)

env_name="prod"
single_arch=""
raw_json=0

usage() {
	sed -n '2,22p' "$0" | sed 's/^# \{0,1\}//'
}

while getopts ":e:a:jh" opt; do
	case "$opt" in
	e) env_name="$OPTARG" ;;
	a) single_arch="$OPTARG" ;;
	j) raw_json=1 ;;
	h)
		usage
		exit 0
		;;
	\?)
		echo "ERROR: unknown option -$OPTARG" >&2
		usage >&2
		exit 2
		;;
	:)
		echo "ERROR: option -$OPTARG requires an argument" >&2
		exit 2
		;;
	esac
done

if [[ -n "${DOWNLOAD_HOST:-}" ]]; then
	host="$DOWNLOAD_HOST"
else
	case "$env_name" in
	prod) host="$PROD_HOST" ;;
	staging) host="$STAGING_HOST" ;;
	*)
		echo "ERROR: -e must be 'prod' or 'staging'; got '$env_name'" >&2
		exit 2
		;;
	esac
fi

if [[ "$host" != https://* ]]; then
	echo "ERROR: manifest host must be https://...; got '$host'" >&2
	exit 2
fi

if ! command -v curl >/dev/null 2>&1; then
	echo "ERROR: curl is required but not found in PATH." >&2
	exit 1
fi

if [[ -n "$single_arch" ]]; then
	arches=("$single_arch")
else
	arches=("${ALL_ARCHES[@]}")
fi

# fetch_manifest prints the manifest body for an arch to stdout, or
# returns non-zero (with the HTTP status on stderr) when unavailable.
fetch_manifest() {
	local arch="$1" url body status
	url="${host}/manifest-${arch}.json"
	body="$(curl -fsSL -w $'\n%{http_code}' "$url" 2>/dev/null)" || {
		echo "  (no manifest at ${url})" >&2
		return 1
	}
	status="${body##*$'\n'}"
	body="${body%$'\n'*}"
	if [[ "$status" != "200" ]]; then
		echo "  (HTTP ${status} at ${url})" >&2
		return 1
	fi
	printf '%s' "$body"
}

if [[ "$raw_json" -eq 1 ]]; then
	for arch in "${arches[@]}"; do
		if body="$(fetch_manifest "$arch")"; then
			printf '%s\n' "$body"
		fi
	done
	exit 0
fi

printf '%-16s %-16s %-12s %s\n' "TARGET" "VERSION" "COMMIT" "PUBLISHED"
printf '%-16s %-16s %-12s %s\n' "------" "-------" "------" "---------"

for arch in "${arches[@]}"; do
	if ! body="$(fetch_manifest "$arch")"; then
		printf '%-16s %s\n' "$arch" "unavailable"
		continue
	fi
	if command -v jq >/dev/null 2>&1; then
		read -r version commit published < <(
			printf '%s' "$body" | jq -r '[.version, (.commit // "-"), (.published_at // "-")] | @tsv'
		)
	else
		version="$(printf '%s' "$body" | sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
		commit="$(printf '%s' "$body" | sed -n 's/.*"commit"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
		published="$(printf '%s' "$body" | sed -n 's/.*"published_at"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
		version="${version:--}"
		commit="${commit:--}"
		published="${published:--}"
	fi
	printf '%-16s %-16s %-12s %s\n' "$arch" "$version" "$commit" "$published"
done
