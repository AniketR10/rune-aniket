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
