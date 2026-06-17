#!/usr/bin/env bash
# Diagnose the "GLX: No GLXFBConfigs returned" failure on a Linux host.
#
# ebiten/glfw dlopen()s libGL.so.1 and the X11 client libs by bare soname
# (internal/glfw/glx_context_linbsd.c, x11_init_linbsd.c). A bare-soname
# dlopen honours, in order: LD_LIBRARY_PATH, then the binary's DT_RUNPATH
# ($ORIGIN/../lib), then /etc/ld.so.cache. So if a bundled (Debian bookworm)
# libGL/libGLX wins over the host's, GLX FBConfig negotiation against the
# host GPU driver fails. This script reports which libGL actually loads and
# why, without changing anything.
#
# Usage:
#   ./rune-glx-diag.sh [/path/to/rune.app]
# If no path is given, common install locations are probed.

set -u

say() { printf '\n=== %s ===\n' "$*"; }

# ---- locate rune.app -------------------------------------------------------
APP="${1:-}"
if [ -z "$APP" ]; then
	for c in \
		"$HOME/.local/rune.app" \
		"$HOME/.local/share/rune/rune.app" \
		"/opt/rune.app" \
		"/usr/local/rune.app"; do
		if [ -d "$c" ]; then APP="$c"; break; fi
	done
fi

if [ -z "$APP" ] || [ ! -d "$APP" ]; then
	echo "Could not find rune.app. Pass its path explicitly:"
	echo "  $0 /path/to/rune.app"
	echo
	echo "Hint: if a launcher exists, find the real bundle with:"
	echo "  readlink -f \"\$(command -v rune)\"; cat \"\$(command -v rune)\" 2>/dev/null"
	exit 1
fi

BIN="$APP/bin/rune"
LIB="$APP/lib"
echo "rune.app: $APP"
echo "binary:   $BIN"

if [ ! -x "$BIN" ]; then
	echo "WARN: $BIN missing or not executable"
fi

# ---- 1. GL / X11 libs physically present in the bundle? --------------------
say "1. GL/X11/driver libs bundled in $LIB (these should NOT be here)"
if [ -d "$LIB" ]; then
	found=$(ls -1 "$LIB" 2>/dev/null | grep -Ei \
		'^(libGL|libGLX|libGLdispatch|libEGL|libOpenGL|libGLU|libgbm|libdrm|libX11|libX11-xcb|libxcb|libXext|libXrandr|libXrender|libXcursor|libXinerama|libXi|libXfixes|libXdamage|libXxf86vm|libXau|libXdmcp|libxshmfence)\.')
	if [ -n "$found" ]; then
		echo "PROBLEM: bundle contains host-owned libs:"
		echo "$found" | sed 's/^/  /'
	else
		echo "OK: no GL/X11/driver libs in the bundle."
	fi
else
	echo "WARN: $LIB does not exist"
fi

say "1b. full bundle lib listing"
ls -1 "$LIB" 2>/dev/null | sed 's/^/  /' || echo "  (none)"

# ---- 2. binary RUNPATH/RPATH ----------------------------------------------
say "2. DT_RUNPATH / DT_RPATH of the binary"
if command -v objdump >/dev/null 2>&1; then
	objdump -p "$BIN" 2>/dev/null | grep -E 'RPATH|RUNPATH' | sed 's/^/  /' \
		|| echo "  (none reported)"
elif command -v readelf >/dev/null 2>&1; then
	readelf -d "$BIN" 2>/dev/null | grep -E 'RPATH|RUNPATH' | sed 's/^/  /' \
		|| echo "  (none reported)"
else
	echo "  (need objdump or readelf)"
fi

say "2b. NEEDED libraries linked directly into the binary"
if command -v objdump >/dev/null 2>&1; then
	objdump -p "$BIN" 2>/dev/null | awk '$1=="NEEDED"{print "  "$2}'
elif command -v readelf >/dev/null 2>&1; then
	readelf -d "$BIN" 2>/dev/null | awk '/NEEDED/{gsub(/[][]/,""); print "  "$NF}'
fi

# ---- 3. launcher / LD_LIBRARY_PATH ----------------------------------------
say "3. launcher on PATH and any LD_LIBRARY_PATH it sets"
RUNE_ON_PATH=$(command -v rune 2>/dev/null || true)
if [ -n "$RUNE_ON_PATH" ]; then
	echo "  rune on PATH: $RUNE_ON_PATH -> $(readlink -f "$RUNE_ON_PATH" 2>/dev/null)"
	if file "$RUNE_ON_PATH" 2>/dev/null | grep -qi 'text\|script'; then
		echo "  --- launcher contents ---"
		sed 's/^/  /' "$RUNE_ON_PATH"
		if grep -q 'LD_LIBRARY_PATH' "$RUNE_ON_PATH"; then
			echo "  PROBLEM: launcher sets LD_LIBRARY_PATH (overrides RUNPATH and host libs)."
		fi
	else
		echo "  (binary, not a script launcher)"
	fi
else
	echo "  (no 'rune' on PATH)"
fi
echo "  current shell LD_LIBRARY_PATH='${LD_LIBRARY_PATH:-}'"

# ---- 4. which libGL the loader actually resolves --------------------------
say "4. libGL/X11 the loader resolves for the binary (env LD_LIBRARY_PATH cleared)"
if command -v ldd >/dev/null 2>&1; then
	env -u LD_LIBRARY_PATH ldd "$BIN" 2>/dev/null \
		| grep -Ei 'libGL|libEGL|libX11|libxcb|libdrm|libgbm' | sed 's/^/  /' \
		|| echo "  (none NEEDED directly — expected: glfw dlopens GL/X11 at runtime)"
else
	echo "  (ldd not available)"
fi

say "4b. resolve the dlopen target 'libGL.so.1' two ways"
# How a bare-soname dlopen resolves WITHOUT the bundle on the path:
if command -v ldconfig >/dev/null 2>&1; then
	host_gl=$(ldconfig -p 2>/dev/null | awk '/libGL\.so\.1/{print $NF; exit}')
	echo "  host (ld.so.cache) libGL.so.1: ${host_gl:-<not found>}"
fi
# What the bundle would force if LD_LIBRARY_PATH=$LIB (the suspected launcher):
if [ -e "$LIB/libGL.so.1" ]; then
	echo "  bundle libGL.so.1:             $LIB/libGL.so.1  (would win if on LD_LIBRARY_PATH/RUNPATH)"
else
	echo "  bundle libGL.so.1:             <not in bundle>"
fi

# ---- 5. host GL sanity -----------------------------------------------------
say "5. host OpenGL sanity (independent of Rune)"
if command -v glxinfo >/dev/null 2>&1; then
	glxinfo 2>&1 | grep -Ei 'OpenGL vendor|OpenGL renderer|OpenGL version|direct rendering' \
		| sed 's/^/  /' || echo "  glxinfo produced no GL strings (host GL itself is broken)"
else
	echo "  glxinfo not installed (Debian/Ubuntu: mesa-utils; Arch: mesa-utils/mesa-demos)."
	echo "  Install it and re-run: a working host shows OpenGL vendor/renderer lines."
fi

say "6. session type"
echo "  XDG_SESSION_TYPE='${XDG_SESSION_TYPE:-}'  WAYLAND_DISPLAY='${WAYLAND_DISPLAY:-}'  DISPLAY='${DISPLAY:-}'"

# ---- verdict ---------------------------------------------------------------
say "VERDICT"
verdict_bundle=0
[ -d "$LIB" ] && ls -1 "$LIB" 2>/dev/null | grep -Eiq \
	'^(libGL|libGLX|libGLdispatch|libEGL|libOpenGL|libX11|libxcb|libdrm|libgbm)\.' && verdict_bundle=1
verdict_launcher=0
[ -n "$RUNE_ON_PATH" ] && grep -q 'LD_LIBRARY_PATH' "$RUNE_ON_PATH" 2>/dev/null && verdict_launcher=1

if [ "$verdict_launcher" = 1 ] && [ "$verdict_bundle" = 1 ]; then
	echo "Launcher forces LD_LIBRARY_PATH at a bundle that contains libGL/X11."
	echo "=> The bundled bookworm GL is loaded instead of the host's. Fix BOTH:"
	echo "   - stop bundling GL/X11 libs, and"
	echo "   - drop LD_LIBRARY_PATH from the launcher (rely on RUNPATH \$ORIGIN/../lib)."
elif [ "$verdict_bundle" = 1 ]; then
	echo "Bundle still contains GL/X11 libs; RUNPATH \$ORIGIN/../lib makes glfw's"
	echo "dlopen pick them over the host driver. Fix: remove them from the bundle"
	echo "(the rebuild did not regenerate lib/, or a stale tarball was installed)."
elif [ "$verdict_launcher" = 1 ]; then
	echo "Launcher sets LD_LIBRARY_PATH even though the bundle looks clean —"
	echo "if it points at any dir with a foreign libGL it will still break. Prefer"
	echo "RUNPATH and unset LD_LIBRARY_PATH in the launcher."
else
	echo "Bundle is clean and no launcher forces LD_LIBRARY_PATH. If GLX still fails,"
	echo "the problem is host-side: check section 5 (glxinfo). A missing system"
	echo "libGL/Mesa or a headless/no-DISPLAY session is the likely cause."
fi

echo
echo "Paste the entire output above back to continue."
