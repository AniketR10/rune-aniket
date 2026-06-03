// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package idepkg

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestCopyExecutablesClearsQuarantine(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	src := filepath.Join(srcDir, "ext")
	require.NoError(t, os.WriteFile(src, []byte("binary"), 0o755))
	require.NoError(t, unix.Setxattr(src, "com.apple.quarantine",
		[]byte("0081;deadbeef;Test;"), 0))

	require.NoError(t, copyExecutables(
		[]executableEntry{{Name: "ext", Mode: 0o755}}, srcDir, dstDir))

	target := filepath.Join(dstDir, "ext")
	_, err := unix.Getxattr(target, "com.apple.quarantine", make([]byte, 256))
	assert.True(t, errors.Is(err, unix.ENOATTR),
		"quarantine xattr should be cleared, got %v", err)
}

func TestClearQuarantineMissingAttrIsNoError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ext")
	require.NoError(t, os.WriteFile(path, []byte("binary"), 0o755))
	require.NoError(t, clearQuarantine(path))
}
