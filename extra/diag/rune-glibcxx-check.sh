#!/usr/bin/env bash
# Confirm the suspected root cause of "GLX: No GLXFBConfigs returned":
# a GLIBCXX/GCC_* symbol-version inversion between the bundled (Debian
# bookworm) libstdc++/libgcc_s and the host's much newer Mesa.
#
# Why this breaks GLX: the binary's RUNPATH is $ORIGIN/../lib, so the bundled
# bookworm libstdc++.so.6 / libgcc_s.so.1 load first. glfw then dlopen()s the
# host libGL.so.1, which pulls in the host's C++ Mesa GLX vendor (libGLX_mesa,
# iris_dri) built against a newer libstdc++. Those vendor libs need GLIBCXX
# symbols the bundled libstdc++ does not provide, so Mesa fails to initialise
# the GLX vendor for the screen and glXGetFBConfigs returns 0.
#
# This script only inspects; it changes nothing. Paste the output back.
#
# Usage:
#   ./rune-glibcxx-check.sh [/path/to/rune.app]

set -u

APP="${1:-$HOME/.local/rune.app}"
LIB="$APP/lib"
BIN="$APP/bin/rune"

sec() { printf '\n=== %s ===\n' "$*"; }

# Highest GLIBCXX_3.4.x and GCC_x.y.z version strings exported by a libstdc++
# / libgcc_s object.
max_ver() {
	# $1 = path, $2 = prefix regex (e.g. GLIBCXX_ or GCC_)
	if [ ! -e "$1" ]; then echo "<missing: $1>"; return; fi
	strings "$1" 2>/dev/null \
		| grep -E "^$2[0-9]+(\.[0-9]+)*$" \
		| sort -V | tail -1
}

# Find the host libstdc++ the loader would use if the bundle were not first.
host_lib() {
	# $1 = soname
	if command -v ldconfig >/dev/null 2>&1; then
		ldconfig -p 2>/dev/null | awk -v s="$1" '$0 ~ s {print $NF; exit}'
	fi
}

echo "rune.app: $APP"

sec "1. C++ runtime libs bundled in the app"
for l in libstdc++.so.6 libgcc_s.so.1 libgomp.so.1; do
	if [ -e "$LIB/$l" ]; then echo "  bundled: $LIB/$l -> $(readlink -f "$LIB/$l")"; else echo "  (not bundled: $l)"; fi
done

HOST_STDCXX="$(host_lib libstdc++.so.6)"
HOST_GCCS="$(host_lib libgcc_s.so.1)"
[ -z "$HOST_STDCXX" ] && for d in /usr/lib /usr/lib/x86_64-linux-gnu /usr/lib64; do [ -e "$d/libstdc++.so.6" ] && HOST_STDCXX="$d/libstdc++.so.6" && break; done
[ -z "$HOST_GCCS" ] && for d in /usr/lib /usr/lib/x86_64-linux-gnu /usr/lib64; do [ -e "$d/libgcc_s.so.1" ] && HOST_GCCS="$d/libgcc_s.so.1" && break; done

sec "2. GLIBCXX / GCC max symbol versions: bundle vs host"
printf '  %-26s %-18s %-18s\n' 'object' 'GLIBCXX(max)' 'GCC(max)'
printf '  %-26s %-18s %-18s\n' 'bundle libstdc++.so.6' \
	"$(max_ver "$LIB/libstdc++.so.6" 'GLIBCXX_')" "$(max_ver "$LIB/libstdc++.so.6" 'GCC_')"
printf '  %-26s %-18s %-18s\n' 'host   libstdc++.so.6' \
	"$(max_ver "$HOST_STDCXX" 'GLIBCXX_')" "$(max_ver "$HOST_STDCXX" 'GCC_')"
printf '  %-26s %-18s %-18s\n' 'bundle libgcc_s.so.1' \
	'-' "$(max_ver "$LIB/libgcc_s.so.1" 'GCC_')"
printf '  %-26s %-18s %-18s\n' 'host   libgcc_s.so.1' \
	'-' "$(max_ver "$HOST_GCCS" 'GCC_')"
echo "  host libstdc++ path: ${HOST_STDCXX:-<not found>}"

sec "3. What the host Mesa GLX vendor actually needs from libstdc++"
# libGLX_mesa.so.0 (and the iris DRI driver) are the C++ consumers. Show the
# highest GLIBCXX they reference; if it exceeds the bundle's max in section 2,
# loading the bundled libstdc++ first is the failure.
MESA_GLX=""
for c in \
	"$(host_lib libGLX_mesa.so.0)" \
	/usr/lib/libGLX_mesa.so.0 /usr/lib/x86_64-linux-gnu/libGLX_mesa.so.0; do
	[ -n "$c" ] && [ -e "$c" ] && MESA_GLX="$c" && break
done
if [ -n "$MESA_GLX" ]; then
	echo "  libGLX_mesa: $MESA_GLX"
	echo "  highest GLIBCXX it requires: $(objdump -T "$MESA_GLX" 2>/dev/null | grep -oE 'GLIBCXX_[0-9.]+' | sort -V | tail -1)"
else
	echo "  libGLX_mesa.so.0 not found via ldconfig/standard dirs"
fi
# The DRI driver (iris) is the bigger C++ consumer; search common driver dirs.
for drv in \
	/usr/lib/dri/iris_dri.so \
	/usr/lib/x86_64-linux-gnu/dri/iris_dri.so \
	/usr/lib/gallium-pipe/* ; do
	[ -e "$drv" ] || continue
	echo "  driver $drv highest GLIBCXX: $(objdump -T "$drv" 2>/dev/null | grep -oE 'GLIBCXX_[0-9.]+' | sort -V | tail -1)"
	break
done

sec "4. PROOF: run rune with the bundled libstdc++ REMOVED from the path"
# If forcing the host libstdc++ (by pointing the loader away from the bundle
# for just libstdc++/libgcc) makes the GLX error go away, the inversion is the
# cause. We do this non-destructively with a temp dir of symlinks to the host
# C++ libs placed ahead via LD_LIBRARY_PATH.
if [ -x "$BIN" ] && [ -n "$HOST_STDCXX" ]; then
	TMP="$(mktemp -d)"
	ln -sf "$HOST_STDCXX" "$TMP/libstdc++.so.6"
	[ -n "$HOST_GCCS" ] && ln -sf "$HOST_GCCS" "$TMP/libgcc_s.so.1"
	echo "  using host C++ libs via LD_LIBRARY_PATH=$TMP (overrides bundle), 6s run:"
	( LD_LIBRARY_PATH="$TMP" timeout 6 "$BIN" 2>&1 || true ) | sed 's/^/    /' | tail -20
	echo "  ---"
	echo "  If the 'No GLXFBConfigs' error is GONE above, the fix is: stop"
	echo "  bundling libstdc++.so.6 / libgcc_s.so.1 / libgomp.so.1 (let the host"
	echo "  provide them, like glibc and GL)."
	rm -rf "$TMP"
else
	echo "  (cannot run: missing binary or host libstdc++)"
fi

echo
echo "Done. Paste sections 2-4 back."
