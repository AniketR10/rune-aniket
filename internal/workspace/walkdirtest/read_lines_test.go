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

package walkdir

import (
	"context"
	"os"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/rune/internal/workspace"
	"unstable.build/rune/internal/workspace/walkdir"
)

type readLinesTestFile struct {
	fullPath string
	content  string
}

func TestReadLines(t *testing.T) {
	tsuite := []struct {
		desc    string
		inFiles []readLinesTestFile
		wantErr string
		wantOut []string
	}{
		{"no files returns no data", nil, "", nil},
		{"empty files returns no data", []readLinesTestFile{{"a", ""}, {"b", ""}}, "", nil},
		{"one file returns data", []readLinesTestFile{{"a", "0\n1"}}, "", []string{"a:1:0", "a:2:1"}},
		{"multiple files returns data", []readLinesTestFile{{"a", "4\n5"}, {"b", "0\n1"}}, "",
			[]string{"a:1:4", "a:2:5", "b:1:0", "b:2:1"}},
		{"more files than workers", []readLinesTestFile{
			{"a", "4\n5"}, {"b", "0\n1"}, {"c", ""}, {"d", ""}, {"e", ""}, {"f", ""}, {"g", ""},
			{"z", "4\n5"}, {"y", "0\n1"}, {"x", ""}, {"w", ""}, {"s", ""}, {"r", ""}, {"n", ""},
			{"x1", ""}, {"x2", ""}, {"x3", ""}, {"x4", ""}, {"x5", ""},
			{"xx1", ""}, {"xx2", ""}, {"xx3", ""}, {"xx4", ""}, {"xx5", ""},
			{"xxx1", ""}, {"xxx2", ""}, {"xxx3", ""}, {"xxx4", ""}, {"xxx5", ""},
			{"xxxx1", ""}, {"xxxx2", ""}, {"xxxx3", ""}, {"xxxx4", ""}, {"xxxx5", ""},
			{"xxxxx1", ""}, {"xxxxx1x2", ""}, {"xxxxx1x3", ""}, {"xxxxx1x4", ""}, {"xxxxx1x5", ""},
		}, "", []string{
			"a:1:4", "a:2:5", "b:1:0", "b:2:1",
			"y:1:0", "y:2:1", "z:1:4", "z:2:5",
		}},
		{"invalid utf-8 character", []readLinesTestFile{{"a", "a\xc5z"}}, "",
			[]string{"a:1:a\xc5z"}},
		{"skips binary files", []readLinesTestFile{{"a", "\x00\x01..."}}, "", nil},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.desc, func(t *testing.T) {
			uri, err := workspaceapi.ParseURI("memory:///")
			require.NoError(t, err)
			scheme, err := workspace.NewMemoryScheme(context.Background(), config.NopConfig(), uri)
			require.NoError(t, err)
			workspace := workspace.NewSchemeWorkspace(uri, scheme, inlineSchedule)

			for _, file := range tcase.inFiles {
				f, werr := scheme.OpenFile(file.fullPath, os.O_CREATE, 0)
				require.Nil(t, werr)
				_, err = f.Write([]byte(file.content))
				require.NoError(t, err)
				require.NoError(t, f.Sync())
				require.NoError(t, f.Close())
			}
			itIn, err := walkdir.ListFiles(context.Background(), scheme, "")
			require.NoError(t, err)

			// sut
			itOut, err := walkdir.ReadLines(context.Background(), workspace, itIn)
			if tcase.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tcase.wantErr)
				return
			}
			require.NoError(t, err)
			var lines []string
			for {
				line, ok := itOut.Next(context.Background())
				if !ok {
					require.NoError(t, itOut.Err())
					break
				}
				lines = append(lines, line)
			}
			// order doesn't matter and the iterator is unordered
			sort.Strings(lines)
			assert.Equal(t, tcase.wantOut, lines)
			assert.NoError(t, itOut.Close())
		})
	}
}

// inlineSchedule is a synchronous workspace.ScheduleNextTick stub
// that runs fn on the calling goroutine. Test-only: production code
// must use the host event-loop scheduler so reload's buffer
// mutations do not run on a worker goroutine.
func inlineSchedule(fn func()) bool {
	fn()
	return true
}
