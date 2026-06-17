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

package idepkg

import (
	"os"
	"strings"
	"testing"
)

// SetPathEnv must prepend the managed bin dir so Rune-managed toolchains
// (e.g. the bundled go) win over same-named system executables such as a
// distro /usr/bin/go. Appending would let the system binary shadow ours.
func TestSetPathEnvPrependsBinDir(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("PATH", "/usr/bin:/usr/local/bin")

	if err := SetPathEnv(dataDir); err != nil {
		t.Fatalf("SetPathEnv: %v", err)
	}

	binDir := makeBinDirname(dataDir)
	path := os.Getenv("PATH")
	entries := strings.Split(path, ":")
	if len(entries) == 0 || entries[0] != binDir {
		t.Fatalf("managed bin dir %q must be first in PATH, got %q", binDir, path)
	}
}
