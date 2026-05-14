// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

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
