#!/usr/bin/env sh
# Rune installer script — installs Rune on Linux and macOS.
#
# Usage:
#   curl -fsSL https://rune-editor.com/install.sh | sh
#
# Environment variables:
#   RUNE_VERSION      — version to install (default: "latest")
#   RUNE_BUNDLE_PATH  — path to a local tarball/dmg (skips download)
#
# Linux tarball layout:
#   rune.app/
#     bin/rune          — the main binary (rpath = $ORIGIN/../lib)
#     lib/*.so.*        — bundled shared libraries
#     share/applications/rune.desktop
#     share/icons/hicolor/{512x512,1024x1024}/apps/rune.png
#
# macOS DMG layout:
#   Rune.dmg containing Rune.app (signed + notarized)

set -eu

RUNE_DOWNLOAD_HOST="${RUNE_DOWNLOAD_HOST:-https://downloads.rune-editor.com}"

main() {
    platform="$(uname -s)"
    arch="$(uname -m)"
    RUNE_VERSION="${RUNE_VERSION:-latest}"

    if [ "$platform" = "Darwin" ]; then
        platform="macos"
    elif [ "$platform" = "Linux" ]; then
        platform="linux"
    else
        err "Unsupported platform: $platform"
    fi

    case "$arch" in
        x86_64 | amd64)
            arch="amd64"
            ;;
        aarch64 | arm64)
            arch="arm64"
            ;;
        *)
            err "Unsupported architecture: $arch"
            ;;
    esac

    if [ -n "${TMPDIR:-}" ] && [ -d "${TMPDIR}" ]; then
        temp="$(mktemp -d "$TMPDIR/rune-XXXXXX")"
    else
        temp="$(mktemp -d "/tmp/rune-XXXXXX")"
    fi
    trap 'rm -rf "$temp"' EXIT

    # Prefer curl, fall back to wget.
    if command -v curl >/dev/null 2>&1; then
        fetch() { command curl -fsSL "$@"; }
    elif command -v wget >/dev/null 2>&1; then
        fetch() { wget -qO- "$@"; }
    else
        err "Could not find 'curl' or 'wget' in your PATH"
    fi

    # Dispatch to platform-specific installer.
    "$platform" "$@"
}

# download_artifact fetches a release artifact into $1.
# Uses RUNE_BUNDLE_PATH if set, otherwise downloads from the public CDN.
#   $1 — destination file path
#   $2 — filename (e.g. "rune-latest.tar.gz", "Rune-latest.dmg")
#   $3 — os-arch identifier (e.g. "linux-amd64", "darwin-arm64")
download_artifact() {
    dest="$1"
    filename="$2"
    os_arch="$3"

    if [ -n "${RUNE_BUNDLE_PATH:-}" ]; then
        info "Using local bundle: $RUNE_BUNDLE_PATH"
        cp "$RUNE_BUNDLE_PATH" "$dest"
        return
    fi

    download_url="${RUNE_DOWNLOAD_HOST}/${os_arch}/${filename}"
    info "Downloading Rune ${RUNE_VERSION} for ${os_arch}..."
    fetch "$download_url" > "$dest" || err "Failed to download from $download_url"
}

# --------------------------------------------------------------------------
# Linux installer
# --------------------------------------------------------------------------
linux() {
    tarball="$temp/rune-linux-$arch.tar.gz"
    download_artifact "$tarball" "rune-${RUNE_VERSION}.tar.gz" "linux-${arch}"

    # Remove any previous installation.
    rm -rf "$HOME/.local/rune.app"
    mkdir -p "$HOME/.local"

    info "Extracting to ~/.local/rune.app..."
    tar -xzf "$tarball" -C "$HOME/.local/"

    if [ ! -f "$HOME/.local/rune.app/bin/rune" ]; then
        err "Archive did not contain rune.app/bin/rune — unexpected layout"
    fi

    # Symlink the binary into ~/.local/bin.
    mkdir -p "$HOME/.local/bin"
    ln -sf "$HOME/.local/rune.app/bin/rune" "$HOME/.local/bin/rune"

    info "Rune has been installed to ~/.local/rune.app"
    info "Symlinked: ~/.local/bin/rune -> ~/.local/rune.app/bin/rune"

    # Install .desktop file and icon.
    install_desktop

    check_path
}

# --------------------------------------------------------------------------
# macOS installer
# --------------------------------------------------------------------------
macos() {
    dmg="$temp/Rune-$arch.dmg"
    download_artifact "$dmg" "Rune-${RUNE_VERSION}.dmg" "darwin-${arch}"

    info "Mounting disk image..."
    hdiutil attach -quiet "$dmg" -mountpoint "$temp/mount"

    # Find the .app inside the DMG (there should be exactly one).
    app="$(cd "$temp/mount/" && echo *.app)"
    if [ -z "$app" ] || [ "$app" = "*.app" ]; then
        hdiutil detach -quiet "$temp/mount"
        err "DMG did not contain a .app bundle"
    fi

    info "Installing $app to /Applications..."
    if [ -d "/Applications/$app" ]; then
        info "Removing existing /Applications/$app"
        rm -rf "/Applications/$app"
    fi
    ditto "$temp/mount/$app" "/Applications/$app"
    hdiutil detach -quiet "$temp/mount"

    # Symlink the CLI binary into ~/.local/bin.
    mkdir -p "$HOME/.local/bin"
    ln -sf "/Applications/$app/Contents/MacOS/rune" "$HOME/.local/bin/rune"

    info "Rune has been installed to /Applications/$app"
    info "Symlinked: ~/.local/bin/rune -> /Applications/$app/Contents/MacOS/rune"

    check_path
}

# --------------------------------------------------------------------------
# Shared helpers
# --------------------------------------------------------------------------

# check_path warns the user if ~/.local/bin is not on $PATH.
check_path() {
    if [ "$(command -v rune 2>/dev/null)" = "$HOME/.local/bin/rune" ]; then
        info "Rune has been installed. Run with 'rune'"
    else
        warn "To run Rune from your terminal, you must add ~/.local/bin to your PATH."
        warn "Run:"
        warn ""
        case "${SHELL:-}" in
            *zsh)
                warn "   echo 'export PATH=\$HOME/.local/bin:\$PATH' >> ~/.zshrc"
                warn "   source ~/.zshrc"
                ;;
            *fish)
                warn "   fish_add_path -U $HOME/.local/bin"
                ;;
            *)
                warn "   echo 'export PATH=\$HOME/.local/bin:\$PATH' >> ~/.bashrc"
                warn "   source ~/.bashrc"
                ;;
        esac
        warn ""
        warn "To run Rune now, '$HOME/.local/bin/rune'"
    fi
}

# install_desktop copies the .desktop file from the tarball into the standard
# XDG location and patches Icon= and Exec= to use absolute paths.
install_desktop() {
    app_dir="$HOME/.local/rune.app"
    src_desktop="$app_dir/share/applications/rune.desktop"

    if [ ! -f "$src_desktop" ]; then
        warn "No .desktop file found in tarball — skipping desktop integration"
        return
    fi

    mkdir -p "$HOME/.local/share/applications"
    desktop_dest="$HOME/.local/share/applications/rune.desktop"
    cp "$src_desktop" "$desktop_dest"

    # Patch Icon= to the absolute path of the bundled 512x512 icon.
    icon_path="$app_dir/share/icons/hicolor/512x512/apps/rune.png"
    if [ -f "$icon_path" ]; then
        sed -i "s|Icon=rune|Icon=$icon_path|g" "$desktop_dest"
    fi

    # Patch Exec= to the absolute path of the binary.
    sed -i "s|Exec=rune|Exec=$app_dir/bin/rune|g" "$desktop_dest"

    info "Installed desktop entry: $desktop_dest"

    # Also install icons into the standard hicolor theme directories so that
    # desktop environments pick them up for app launchers / taskbars.
    for size in 512x512 1024x1024; do
        src_icon="$app_dir/share/icons/hicolor/$size/apps/rune.png"
        if [ -f "$src_icon" ]; then
            dest_dir="$HOME/.local/share/icons/hicolor/$size/apps"
            mkdir -p "$dest_dir"
            cp "$src_icon" "$dest_dir/rune.png"
        fi
    done

    # Update the icon cache if the tool is available.
    if command -v gtk-update-icon-cache >/dev/null 2>&1; then
        gtk-update-icon-cache -f -t "$HOME/.local/share/icons/hicolor" 2>/dev/null || true
    fi
}

info() { printf '\033[1;32m%s\033[0m\n' "$*"; }
warn() { printf '\033[1;33m%s\033[0m\n' "$*" >&2; }
err()  { printf '\033[1;31mError: %s\033[0m\n' "$*" >&2; exit 1; }

main "$@"
