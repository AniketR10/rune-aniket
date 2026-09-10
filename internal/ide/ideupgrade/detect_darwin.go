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

//go:build darwin

package ideupgrade

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// errTranslocated is wrapped by ErrUpgradeNotSupported when the
// running bundle was launched from macOS's translocation
// quarantine. In that case the install path the user actually has
// is unknown, so we refuse rather than guess.
var errTranslocated = errors.New(
	"macOS Gatekeeper translocated this bundle to a sandbox; move Rune.app to /Applications and re-launch before upgrading")

// translocationPrefixes are the path prefixes macOS uses for app
// translocation. Bundles launched from any of these are read-only
// copies; upgrading them would silently no-op.
var translocationPrefixes = []string{
	"/private/var/folders/",
	"/var/folders/",
}

// detectRunningInstallOS implements the darwin half of
// detectRunningInstall. It walks ancestors of the resolved binary
// path looking for a `*.app` directory with `Contents/Info.plist`
// and refuses any layout that doesn't match.
func detectRunningInstallOS(real string, cfg Config) (detectedInstall, error) {
	if isTranslocatedPath(real) {
		return detectedInstall{}, &ErrUpgradeNotSupported{
			Path:   real,
			Reason: errTranslocated,
		}
	}

	app, err := findAppBundleAncestor(real)
	if err != nil {
		return detectedInstall{}, &ErrUpgradeNotSupported{
			Path:   real,
			Reason: err,
		}
	}

	rel, err := filepath.Rel(app, real)
	if err != nil {
		return detectedInstall{}, &ErrUpgradeNotSupported{
			Path:   real,
			Reason: fmt.Errorf("compute binary relpath: %w", err),
		}
	}
	if strings.HasPrefix(rel, "..") {
		return detectedInstall{}, &ErrUpgradeNotSupported{
			Path:   real,
			Reason: fmt.Errorf("running binary %s is not inside bundle %s", real, app),
		}
	}
	if !strings.HasPrefix(filepath.ToSlash(rel), "Contents/MacOS/") {
		return detectedInstall{}, &ErrUpgradeNotSupported{
			Path: real,
			Reason: fmt.Errorf(
				"running binary %s does not live under Contents/MacOS/ inside %s",
				real, app),
		}
	}

	return detectedInstall{
		InstallRoot:      filepath.Dir(app),
		AppName:          filepath.Base(app),
		CLIBinaryRelPath: rel,
		ExecutablePath:   real,
	}, nil
}

// findAppBundleAncestor walks ancestors of p looking for the
// closest *.app directory that contains Contents/Info.plist. Returns
// errNoAppBundle when no such ancestor exists.
func findAppBundleAncestor(p string) (string, error) {
	dir := p
	for {
		parent := filepath.Dir(dir)
		if parent == dir { // reached filesystem root
			return "", errNoAppBundle
		}
		if strings.HasSuffix(dir, ".app") {
			info, err := os.Stat(filepath.Join(dir, "Contents", "Info.plist"))
			if err == nil && !info.IsDir() {
				return dir, nil
			}
		}
		dir = parent
	}
}

func isTranslocatedPath(p string) bool {
	for _, prefix := range translocationPrefixes {
		if !strings.HasPrefix(p, prefix) {
			continue
		}
		// Path under /private/var/folders is translocated only when
		// "AppTranslocation" appears as a path segment. Other
		// /private/var/folders paths (e.g. a test temp dir) should
		// not be refused.
		if strings.Contains(p, "/AppTranslocation/") {
			return true
		}
	}
	return false
}
