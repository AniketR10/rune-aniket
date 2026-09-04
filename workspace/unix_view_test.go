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
	"fmt"
	"io"

	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/cell"
)

const testFilesLines = 3

var fileWithNoEOL string
var fileWithEOL string

func init() {
	f, err := os.CreateTemp("", "test_raw_cells_1_")
	if err != nil {
		return
	}

	f.WriteString("LINE")
	for i := 1; i < testFilesLines; i++ {
		_, err := f.WriteString("\nLINE")
		if err != nil {
			return
		}
	}

	fileWithNoEOL = f.Name()
	f.Close()

	f, err = os.CreateTemp("", "test_raw_cells_2_")
	if err != nil {
		return
	}
	defer f.Close()

	for range testFilesLines {
		_, err := f.WriteString("LINE\n")
		if err != nil {
			return
		}
	}

	fileWithEOL = f.Name()
}

func TestUnixFile(t *testing.T) {
	require.NotZero(t, fileWithNoEOL)
	require.NotZero(t, fileWithEOL)

	f, err := os.Open(fileWithNoEOL)
	require.NoError(t, err)
	defer f.Close()

	f2, err := os.Open(fileWithEOL)
	require.NoError(t, err)
	defer f2.Close()

	tsuite := []struct {
		input    io.Reader
		expected string
		rows     int
	}{
		{strings.NewReader(""), "", 1},
		{strings.NewReader("fjelkwfjlkew"), "fjelkwfjlkew", 1},
		{strings.NewReader("fjelkwfjlkew\nfewjklfe"), "fjelkwfjlkew\nfewjklfe", 2},
		{f, "LINE\nLINE\nLINE", testFilesLines},
		{f2, "LINE\nLINE\nLINE", testFilesLines},
	}

	for i, tcase := range tsuite {
		reader := tcase.input
		{
			c := cell.NewBuffer()
			_, err := c.ReadFrom(tcase.input)
			require.NoError(t, err)

			reader := NewUnixFileView(c)

			assert.Equal(t, tcase.rows, reader.Rows(), fmt.Sprintf("ReadFrom(%d)", i))
			assert.Equal(t, tcase.expected, reader.String(), i)
		}

		s := reader.(io.Seeker)
		s.Seek(0, 0)

		{
			c := cell.NewBuffer()
			bytes, err := io.ReadAll(tcase.input)
			require.NoError(t, err)
			c.Edit(context.Background(), term.Coordinates{},
				term.Coordinates{}, string(bytes))

			reader := NewUnixFileView(c)

			assert.Equal(t, tcase.rows, reader.Rows(), fmt.Sprintf("insert(%d)", i))
			assert.Equal(t, tcase.expected, reader.String(), i)
		}
	}
}

func TestBufferViewIntegration(t *testing.T) {
	const snippet = "If you accept Hawking radiation and accept " +
		"that black holes radiate away all the information stored inside, eventually, " +
		"if you reverse the arrow of time, you'll find that the start of the universe " +
		"is actually information being injected into black holes, which eventually " +
		"start to spit out particles, stars, galaxies and even life."

	suite := []struct {
		desc    string
		content string
	}{
		{"reads reader content into buffer after reset", snippet},
		{"reads reader content into buffer after reset, with last EOL", snippet + "\n"},
	}

	for _, test := range suite {
		t.Run(test.desc, func(t *testing.T) {
			b := cell.NewBuffer()
			view := NewUnixFileView(b.View())
			b.WithView(view)

			_, err := b.ReadFrom(strings.NewReader(test.content))
			require.NoError(t, err)
			if !view.EndsWithEOL() {
				b.WriteString("\n")
			}
			require.Equal(t, snippet, b.String())

			for i := range 2 {
				b.Reset()
				_, err = b.ReadFrom(strings.NewReader(test.content))
				require.NoError(t, err)
				if !view.EndsWithEOL() {
					b.WriteString("\n")
				}
				assert.Equal(t, snippet, b.String(), i)
			}
		})
	}
}
