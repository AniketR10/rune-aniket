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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// seedLinuxAppDir materializes <root>/<appName>/bin/<binName> matching
// the layout install.sh ships.
func seedLinuxAppDir(t *testing.T, root, appName, binName string) string {
	t.Helper()
	binDir := filepath.Join(root, appName, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	bin := filepath.Join(binDir, binName)
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755))
	resolved, err := filepath.EvalSymlinks(bin)
	require.NoError(t, err)
	return resolved
}

func TestDetectRunningInstall_LinuxHappyPath(t *testing.T) {
	root := t.TempDir()
	bin := seedLinuxAppDir(t, root, "rune.app", "rune")

	got, err := detectRunningInstall(Config{
		Executable: func() (string, error) { return bin, nil },
	})
	require.NoError(t, err)
	require.Equal(t, "rune.app", got.AppName)
	require.Equal(t, filepath.Join("bin", "rune"), got.CLIBinaryRelPath)
	resolvedRoot, err := filepath.EvalSymlinks(root)
	require.NoError(t, err)
	require.Equal(t, resolvedRoot, got.InstallRoot)
}

func TestDetectRunningInstall_LinuxRefusesNonBinLayout(t *testing.T) {
	root := t.TempDir()
	// Binary lives directly under <root>/rune, not <root>/bin/rune.
	bin := filepath.Join(root, "rune")
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755))

	_, err := detectRunningInstall(Config{
		Executable: func() (string, error) { return bin, nil },
	})
	require.Error(t, err)
	var notSupported *ErrUpgradeNotSupported
	require.True(t, errors.As(err, &notSupported))
	require.ErrorIs(t, notSupported.Reason, errNoAppBundle)
}

func TestDetectRunningInstall_LinuxRefusesSystemAndSandboxPrefixes(t *testing.T) {
	cases := []struct {
		name string
		path string
		want error
	}{
		{"usr", "/usr/bin/rune", errSystemInstall},
		{"opt", "/opt/rune/bin/rune", errSystemInstall},
		{"bin", "/bin/rune", errSystemInstall},
		{"sbin", "/sbin/rune", errSystemInstall},
		{"snap", "/snap/rune/current/bin/rune", errSandboxInstall},
		{"flatpak", "/var/lib/flatpak/app/rune/bin/rune", errSandboxInstall},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := detectRunningInstall(Config{
				Executable: func() (string, error) { return tc.path, nil },
			})
			require.Error(t, err)
			var notSupported *ErrUpgradeNotSupported
			require.True(t, errors.As(err, &notSupported))
			require.ErrorIs(t, notSupported.Reason, tc.want)
		})
	}
}
