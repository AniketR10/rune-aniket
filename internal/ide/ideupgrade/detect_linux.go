// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

//go:build linux

package ideupgrade

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// errSystemInstall is wrapped by ErrUpgradeNotSupported when the
// running binary lives under a path managed by the system package
// manager (or a sandbox); we should not stomp on those installs.
var (
	errSystemInstall  = errors.New("running binary lives under a system-managed prefix; let your package manager handle updates")
	errSandboxInstall = errors.New("running binary is sandboxed (Snap/Flatpak/AppImage); use the platform's update mechanism")
)

// systemPrefixes are filesystem prefixes managed by the OS or its
// package manager. We refuse to upgrade installs under any of these
// because the user almost certainly didn't install via our tarball.
var systemPrefixes = []string{
	"/usr/",
	"/opt/",
	"/bin/",
	"/sbin/",
}

// sandboxPrefixes are filesystem prefixes used by Linux sandboxed
// distribution mechanisms. Bundles running from those paths are
// updated by the sandbox runtime, not by us.
var sandboxPrefixes = []string{
	"/snap/",
	"/var/lib/snapd/",
	"/var/lib/flatpak/",
	"/run/host/usr/lib/flatpak/",
}

// detectRunningInstallOS implements the linux half of
// detectRunningInstall. The expected layout mirrors `install.sh`:
// `<install root>/<app dir>/<bin rel path>` where the app dir
// contains at least a `bin/` subdirectory.
func detectRunningInstallOS(real string, cfg Config) (detectedInstall, error) {
	for _, p := range sandboxPrefixes {
		if strings.HasPrefix(real, p) {
			return detectedInstall{}, &ErrUpgradeNotSupported{
				Path:   real,
				Reason: errSandboxInstall,
			}
		}
	}
	for _, p := range systemPrefixes {
		if strings.HasPrefix(real, p) {
			return detectedInstall{}, &ErrUpgradeNotSupported{
				Path:   real,
				Reason: errSystemInstall,
			}
		}
	}

	app, rel, err := findAppDirAncestor(real)
	if err != nil {
		return detectedInstall{}, &ErrUpgradeNotSupported{
			Path:   real,
			Reason: err,
		}
	}

	return detectedInstall{
		InstallRoot:      filepath.Dir(app),
		AppName:          filepath.Base(app),
		CLIBinaryRelPath: rel,
		ExecutablePath:   real,
	}, nil
}

// findAppDirAncestor walks ancestors of the running binary looking
// for a directory whose layout looks like the tarball Rune ships:
// the ancestor itself contains a `bin/` subdirectory and the binary
// sits at `<ancestor>/bin/<name>`.
//
// Returns the absolute ancestor path and the binary's relpath to it.
func findAppDirAncestor(real string) (app string, rel string, err error) {
	parent := filepath.Dir(real)
	// The binary must be at `<app>/bin/<name>` (the layout install.sh
	// produces). Anything else is rejected — a stricter shape than
	// darwin's because Linux file system layouts vary too much for
	// the heuristic to be useful otherwise.
	if filepath.Base(parent) != "bin" {
		return "", "", fmt.Errorf(
			"%w: expected binary under <app>/bin/, got %s",
			errNoAppBundle, real)
	}
	candidate := filepath.Dir(parent)
	if candidate == "" || candidate == "/" || candidate == "." {
		return "", "", fmt.Errorf("%w: %s has no app-dir ancestor", errNoAppBundle, real)
	}
	info, statErr := os.Stat(filepath.Join(candidate, "bin"))
	if statErr != nil || !info.IsDir() {
		return "", "", fmt.Errorf("%w: %s/bin does not exist", errNoAppBundle, candidate)
	}
	rel, err = filepath.Rel(candidate, real)
	if err != nil {
		return "", "", fmt.Errorf("compute binary relpath: %w", err)
	}
	return candidate, rel, nil
}
