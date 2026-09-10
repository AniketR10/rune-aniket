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

package extensionv2

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

func TestExtensionLogPathStableAndUnique(t *testing.T) {
	t.Parallel()

	ws1, err := workspaceapi.ParseURI("file:///tmp/a")
	require.NoError(t, err)
	ws2, err := workspaceapi.ParseURI("file:///tmp/b")
	require.NoError(t, err)

	p1 := extensionLogPath(ws1, "alpha")
	p1again := extensionLogPath(ws1, "alpha")
	p2 := extensionLogPath(ws1, "beta")
	p3 := extensionLogPath(ws2, "alpha")

	assert.Equal(t, p1, p1again, "same inputs should produce same path")
	assert.NotEqual(t, p1, p2, "different ids should produce different paths")
	assert.NotEqual(t, p1, p3, "different workspaces should produce different paths")

	assert.Equal(t,
		filepath.Clean(os.TempDir()),
		filepath.Clean(filepath.Dir(p1)),
	)
	name := filepath.Base(p1)
	matched, err := regexp.MatchString(`^rune-extension-[0-9a-f]{16}\.log$`, name)
	require.NoError(t, err)
	assert.True(t, matched, "filename %q does not match expected pattern", name)
}
