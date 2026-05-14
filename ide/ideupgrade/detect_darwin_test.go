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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// seedAppBundle materializes <root>/<appName>/Contents/MacOS/<binName>
// and Contents/Info.plist so detectRunningInstall accepts the layout.
// Returns the resolved binary path (EvalSymlinks-friendly).
func seedAppBundle(t *testing.T, root, appName, binName string) string {
	t.Helper()
	binDir := filepath.Join(root, appName, "Contents", "MacOS")
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	bin := filepath.Join(binDir, binName)
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755))
	plist := filepath.Join(root, appName, "Contents", "Info.plist")
	require.NoError(t, os.WriteFile(plist, []byte("<plist/>"), 0o644))
	resolved, err := filepath.EvalSymlinks(bin)
	require.NoError(t, err)
	return resolved
}

func TestDetectRunningInstall_DarwinHappyPath(t *testing.T) {
	root := t.TempDir()
	bin := seedAppBundle(t, root, "Rune.app", "rune")

	got, err := detectRunningInstall(Config{
		Executable: func() (string, error) { return bin, nil },
	})
	require.NoError(t, err)
	require.Equal(t, "Rune.app", got.AppName)
	require.Equal(t,
		filepath.Join("Contents", "MacOS", "rune"),
		got.CLIBinaryRelPath)
	// InstallRoot should be the resolved parent of the .app, not the
	// raw t.TempDir() (which on darwin may live under /var/folders/).
	resolvedRoot, err := filepath.EvalSymlinks(root)
	require.NoError(t, err)
	require.Equal(t, resolvedRoot, got.InstallRoot)
}

func TestDetectRunningInstall_DarwinAcceptsNonRuneAppName(t *testing.T) {
	root := t.TempDir()
	bin := seedAppBundle(t, root, "Rune-Beta.app", "rune")

	got, err := detectRunningInstall(Config{
		Executable: func() (string, error) { return bin, nil },
	})
	require.NoError(t, err)
	require.Equal(t, "Rune-Beta.app", got.AppName)
}

func TestDetectRunningInstall_DarwinViaSymlink(t *testing.T) {
	root := t.TempDir()
	bin := seedAppBundle(t, root, "Rune.app", "rune")

	link := filepath.Join(t.TempDir(), "rune-shim")
	require.NoError(t, os.Symlink(bin, link))

	got, err := detectRunningInstall(Config{
		Executable: func() (string, error) { return link, nil },
	})
	require.NoError(t, err)
	require.Equal(t, "Rune.app", got.AppName)
}

func TestDetectRunningInstall_DarwinRefusesDevBinary(t *testing.T) {
	root := t.TempDir()
	dev := filepath.Join(root, "go-build", "rune")
	require.NoError(t, os.MkdirAll(filepath.Dir(dev), 0o755))
	require.NoError(t, os.WriteFile(dev, []byte("#!/bin/sh\n"), 0o755))

	_, err := detectRunningInstall(Config{
		Executable: func() (string, error) { return dev, nil },
	})
	require.Error(t, err)
	var notSupported *ErrUpgradeNotSupported
	require.True(t, errors.As(err, &notSupported))
	require.ErrorIs(t, notSupported.Reason, errNoAppBundle)
}

func TestDetectRunningInstall_DarwinRefusesAppWithoutInfoPlist(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "Rune.app", "Contents", "MacOS")
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	bin := filepath.Join(binDir, "rune")
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755))
	// no Info.plist

	_, err := detectRunningInstall(Config{
		Executable: func() (string, error) { return bin, nil },
	})
	require.Error(t, err)
	var notSupported *ErrUpgradeNotSupported
	require.True(t, errors.As(err, &notSupported))
}

func TestDetectRunningInstall_DarwinRefusesBinaryOutsideMacOS(t *testing.T) {
	root := t.TempDir()
	// Create a valid bundle but put the binary in a sibling
	// directory (Contents/Resources) so detection rejects it.
	resourcesDir := filepath.Join(root, "Rune.app", "Contents", "Resources")
	require.NoError(t, os.MkdirAll(resourcesDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(root, "Rune.app", "Contents", "Info.plist"),
		[]byte("<plist/>"), 0o644))
	bin := filepath.Join(resourcesDir, "rune")
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755))

	_, err := detectRunningInstall(Config{
		Executable: func() (string, error) { return bin, nil },
	})
	require.Error(t, err)
	var notSupported *ErrUpgradeNotSupported
	require.True(t, errors.As(err, &notSupported))
}

func TestDetectRunningInstall_DarwinRefusesTranslocated(t *testing.T) {
	// We can't actually run a translocated bundle in a unit test;
	// exercise isTranslocatedPath directly with a synthetic path
	// that matches the macOS pattern.
	translocated := "/private/var/folders/xx/yyy/T/AppTranslocation/UUID/d/Rune.app/Contents/MacOS/rune"
	require.True(t, isTranslocatedPath(translocated))

	_, err := detectRunningInstall(Config{
		Executable: func() (string, error) { return translocated, nil },
	})
	require.Error(t, err)
	var notSupported *ErrUpgradeNotSupported
	require.True(t, errors.As(err, &notSupported))
	require.ErrorIs(t, notSupported.Reason, errTranslocated)
}
