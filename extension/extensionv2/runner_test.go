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

// Unstable Build LLC ("COMPANY") CONFIDENTIAL
package extensionv2

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/debug"
)

func TestRunnerSocketPath(t *testing.T) {
	t.Parallel()

	r := &Runner{dataDir: "/tmp/rune-data"}

	cases := []struct {
		name string
		uri  string
	}{
		{"file scheme", "file:///home/me/proj"},
		{"ssh scheme with home alias", "ssh://10.0.0.9/~/src/blue"},
		{"ssh scheme with absolute path", "ssh://example.com/srv/app"},
	}

	socketDir := filepath.Join(r.dataDir, "sockets")
	hexSock := regexp.MustCompile(`^[0-9a-f]+\.sock$`)

	for _, tc := range cases {
		t.Run(tc.name+": socket lives under <dataDir>/sockets", func(t *testing.T) {
			uri, err := workspaceapi.ParseURI(tc.uri)
			require.NoError(t, err)

			got := r.socketPath(uri)

			assert.True(t, strings.HasPrefix(got, socketDir+string(filepath.Separator)),
				"socket %q should live under %q", got, socketDir)
			assert.True(t, hexSock.MatchString(filepath.Base(got)),
				"socket basename %q should be hex.sock; some filesystems "+
					"reject path components with special characters",
				filepath.Base(got))
			// macOS' sun_path is 104 bytes including the NUL.
			assert.Less(t, len(got), 104,
				"socket path %q is too long for sun_path", got)
		})
	}

	t.Run("same URI hashes to the same socket", func(t *testing.T) {
		uri, err := workspaceapi.ParseURI("ssh://example.com/srv/app")
		require.NoError(t, err)

		assert.Equal(t, r.socketPath(uri), r.socketPath(uri),
			"hash of the same URI must be stable across calls so a "+
				"re-opened workspace replaces its stale socket cleanly")
	})

	t.Run("different URIs hash to different sockets", func(t *testing.T) {
		a, err := workspaceapi.ParseURI("ssh://example.com/srv/app")
		require.NoError(t, err)
		b, err := workspaceapi.ParseURI("ssh://example.com/srv/other")
		require.NoError(t, err)

		assert.NotEqual(t, r.socketPath(a), r.socketPath(b),
			"distinct workspaces must not collide on the same socket")
	})

	t.Run("falls back to TempDir when dataDir would overflow sun_path", func(t *testing.T) {
		// macOS sockaddr_un.sun_path is 104 bytes. A typical
		// t.TempDir() under /var/folders/... already eats ~95
		// bytes, leaving no room for "sockets/<hash>.sock".
		// Without a fallback the ENOENT/EINVAL from bind would
		// surface deep inside extension setup; the runner
		// should silently relocate to a shorter path instead.
		long := strings.Repeat("a", 90)
		deep := &Runner{dataDir: filepath.Join(string(filepath.Separator), long)}
		uri, err := workspaceapi.ParseURI("ssh://example.com/srv/app")
		require.NoError(t, err)

		got := deep.socketPath(uri)

		assert.Less(t, len(got), 104,
			"fallback socket %q must fit in sun_path", got)
		assert.Equal(t, filepath.Dir(got), filepath.Clean(os.TempDir()),
			"fallback should live directly under os.TempDir(); got %q", got)
		assert.True(t, strings.HasPrefix(filepath.Base(got), debug.Package+"-"),
			"fallback basename should be prefixed with %q to namespace it; "+
				"got %q", debug.Package, filepath.Base(got))
	})
}
