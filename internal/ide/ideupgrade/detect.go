// Copyright (C) 2017-2026 The Rune Authors
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

package ideupgrade

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// detectedInstall is the result of inspecting the running binary to
// decide where an in-place upgrade should write.
type detectedInstall struct {
	// InstallRoot is the directory containing the app bundle.
	InstallRoot string
	// AppName is the directory name of the app bundle (relative to
	// InstallRoot).
	AppName string
	// CLIBinaryRelPath is the binary's path relative to
	// filepath.Join(InstallRoot, AppName).
	CLIBinaryRelPath string
	// ExecutablePath is the resolved path of the running binary
	// (after symlink and EvalSymlinks resolution). Kept for
	// diagnostics in error messages.
	ExecutablePath string
}

// ErrUpgradeNotSupported is returned by detectRunningInstall when the
// running binary does not live inside a layout we know how to
// upgrade in place. Callers should surface Reason to the user
// instead of attempting the upgrade.
type ErrUpgradeNotSupported struct {
	// Path is the resolved path of the running binary, when known.
	Path string
	// Reason describes why the layout cannot be upgraded.
	Reason error
}

func (e *ErrUpgradeNotSupported) Error() string {
	if e.Path == "" {
		return fmt.Sprintf("upgrade not supported: %v", e.Reason)
	}
	return fmt.Sprintf("upgrade not supported for %s: %v", e.Path, e.Reason)
}

func (e *ErrUpgradeNotSupported) Unwrap() error { return e.Reason }

// detectRunningInstall resolves the running binary's location and
// returns the install root that an in-place upgrade should target.
// The resolution is OS-specific (see detect_darwin.go /
// detect_linux.go); shared concerns (symlink resolution, executable
// lookup) live here.
func detectRunningInstall(cfg Config) (detectedInstall, error) {
	exeFn := cfg.Executable
	if exeFn == nil {
		exeFn = os.Executable
	}

	exe, err := exeFn()
	if err != nil {
		return detectedInstall{}, &ErrUpgradeNotSupported{
			Reason: fmt.Errorf("resolve executable: %w", err),
		}
	}
	real, err := filepath.EvalSymlinks(exe)
	if err != nil {
		// Fall back to the raw executable path; symlink resolution
		// can fail under sandboxes (e.g. AppImage) where the symlink
		// target is not visible to us.
		real = exe
	}
	real = filepath.Clean(real)

	return detectRunningInstallOS(real, cfg)
}

// errNoAppBundle is the canonical reason wrapped by
// ErrUpgradeNotSupported when the running binary has no recognized
// install layout ancestor. Tests assert on this with errors.Is.
var errNoAppBundle = errors.New("running binary is not inside a managed Rune install")
