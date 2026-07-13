#!/bin/bash
# Run a cgo build command and fail when the linker reports a linked
# object was built for a newer macOS than the link target.
#
# Why: the rune binary statically links native (cgo) archives. The
# final binary's LC_BUILD_VERSION minos is set by the link line, so it
# can read a low floor (e.g. 13.3) even while it statically contains
# objects compiled for a higher OS. Such a binary launches but can crash
# at runtime against APIs absent on the advertised floor. ld emits
#   ld: warning: object file (...) was built for newer 'macOS' version
#   (26.0) than being linked (13.3)
# for exactly this case. Treating that warning as fatal catches a
# dependency raising the effective floor at build time, not by users.
#
# Usage: cgo-build-guard.sh <shell-command-string>
# The single argument is run verbatim with `bash -c`, so the caller's
# quoting (e.g. CC="clang -arch arm64 ...") is preserved exactly. The
# command's stdout/stderr stream through unchanged; on success the
# captured stderr is scanned and a non-zero exit is forced if the
# newer-macOS warning is present.
set -uo pipefail

if [[ $# -ne 1 ]]; then
    echo "ERROR: usage: cgo-build-guard.sh <shell-command-string>" >&2
    exit 1
fi

stderr_file="$(mktemp "${TMPDIR:-/tmp}/cgo-build-guard-XXXXXX")"
trap 'rm -f "$stderr_file"' EXIT

# Mirror stderr to the terminal and a capture file without disturbing
# stdout, so other diagnostics still surface and exit status is real.
bash -c "$1" 2> >(tee "$stderr_file" >&2)
status=$?
# Ensure the tee in the process substitution has flushed before we scan.
wait

if [[ $status -ne 0 ]]; then
    exit $status
fi

if grep -q "was built for newer 'macOS' version" "$stderr_file"; then
    echo "ERROR: a linked object targets a newer macOS than the build target." >&2
	echo "       This raises the effective minimum OS above what we advertise." >&2
	echo "       Rebuild the offending dependency with a matching deployment target" >&2
	echo "       (e.g. CMAKE_OSX_DEPLOYMENT_TARGET / -mmacosx-version-min)." >&2
	exit 1
fi
