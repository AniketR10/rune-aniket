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
			Path:   real,
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
