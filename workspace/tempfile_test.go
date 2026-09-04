// Copyright (C) 2017-2026 Unstable Build, LLC
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

package workspace

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

func TestCreateTemp(t *testing.T) {
	suite := []struct {
		description string
		dir         string
		pattern     string
		expectErr   string
		assert      func(*testing.T, workspaceapi.File)
	}{
		{"returns error if path is passed in pattern", "", "a/b", "pattern cannot contain separator", nil},
		{"uses dir if passed", "/stomp", "", "", func(t *testing.T, file workspaceapi.File) {
			require.Equal(t, "/stomp", filepath.Dir(file.Name()), file.Name())
		}},
		{"uses pattern if passed", "/stomp", "sup_*_bla", "", func(t *testing.T, file workspaceapi.File) {
			require.True(t, strings.HasPrefix(file.Name(), "/stomp/sup_"))
			require.True(t, strings.HasSuffix(file.Name(), "_bla"))
		}},
		{"uses TMPDIR env var if set", "/stomp", "sup_*_bla", "", func(t *testing.T, file workspaceapi.File) {
			require.True(t, strings.HasPrefix(file.Name(), "/stomp/sup_"))
			require.True(t, strings.HasSuffix(file.Name(), "_bla"))
		}},
	}

	for _, test := range suite {
		ctx := context.Background()
		cfg := config.NopConfig()
		uri, err := workspaceapi.ParseURI("memory:///a")
		require.NoError(t, err)
		t.Run(test.description, func(t *testing.T) {
			memfs, err := NewMemoryScheme(ctx, cfg, uri)
			require.NoError(t, err)
			for range 1000 {
				file, err := CreateTemp(memfs, test.dir, test.pattern)
				if test.expectErr != "" {
					require.Error(t, err)
					require.True(t, strings.Contains(err.Error(), test.expectErr), err.Error())
				} else {
					require.NoError(t, err)
					actual, err := memfs.Open(file.Name())
					require.NoError(t, err)
					require.Equal(t, file.Name(), actual.Name())
					require.NoError(t, actual.Close())
					if test.assert != nil {
						test.assert(t, file)
					}
				}
				if err == nil {
					require.NoError(t, file.Close())
				}
			}
		})
	}
}
