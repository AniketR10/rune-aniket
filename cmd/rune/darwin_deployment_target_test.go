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

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDarwinAppBundlesPinMinimumOSVersion guards against shipping a
// macOS app bundle whose minimum OS version is left to the build SDK
// default. When the arm64 build was missing -mmacosx-version-min and
// the bundle plist omitted LSMinimumSystemVersion, the linker recorded
// the build host's SDK (e.g. macOS 26) as the floor, so the DMG would
// not launch on supported releases. Both per-arch plists must declare
// LSMinimumSystemVersion equal to their Makefile deployment-target
// floor. This runs on every platform (it only reads checked-in files),
// so Linux CI catches a regression too; the artifact/binary minos
// checks live in the macOS-only dist gate.
func TestDarwinAppBundlesPinMinimumOSVersion(t *testing.T) {
	repoRoot := repoRootDir(t)
	makefile := readFileString(t, filepath.Join(repoRoot, "cmd", "rune", "Makefile"))

	cases := []struct {
		name      string
		floorVar  string
		plistPath string
	}{
		{
			name:      "arm64",
			floorVar:  "DARWIN_ARM64_MIN_MACOS",
			plistPath: filepath.Join(repoRoot, "extra", "osx", "Rune.app", "Contents", "Info.plist"),
		},
		{
			name:      "amd64",
			floorVar:  "DARWIN_AMD64_MIN_MACOS",
			plistPath: filepath.Join(repoRoot, "extra", "osx", "Info.amd64.plist"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			floor := makefileVar(t, makefile, tc.floorVar)
			plist := readFileString(t, tc.plistPath)
			got := plistString(t, plist, "LSMinimumSystemVersion")
			require.Equalf(t, floor, got,
				"%s LSMinimumSystemVersion must match Makefile %s; "+
					"otherwise the bundle inherits the build SDK's minimum OS",
				tc.name, tc.floorVar)
		})
	}
}

func repoRootDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed")
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoErrorf(t, err, "read %s", path)
	return string(b)
}

// makefileVar returns the value assigned to name via `name ?= value`
// or `name = value`, ignoring surrounding whitespace.
func makefileVar(t *testing.T, makefile, name string) string {
	t.Helper()
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `\s*\??=\s*(\S+)`)
	m := re.FindStringSubmatch(makefile)
	require.Lenf(t, m, 2, "could not find %s assignment in Makefile", name)
	return m[1]
}

// plistString returns the <string> value following the given <key> in
// a property list document.
func plistString(t *testing.T, plist, key string) string {
	t.Helper()
	re := regexp.MustCompile(
		`(?s)<key>` + regexp.QuoteMeta(key) + `</key>\s*<string>(.*?)</string>`)
	m := re.FindStringSubmatch(plist)
	require.Lenf(t, m, 2, "could not find <key>%s</key> in plist", key)
	return m[1]
}
