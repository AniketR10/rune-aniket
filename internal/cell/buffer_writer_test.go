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

package cell

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestWriterContext(t *testing.T) {
	type myKey string

	writer := NewBufferWriter(
		context.WithValue(context.Background(), myKey("a"), "a"), 10, 10)

	value := writer.Context().Value(myKey("a"))
	require.NotNil(t, value)
	str, ok := value.(string)
	require.True(t, ok)
	assert.Equal(t, "a", str)

	writer.SetContext(
		context.WithValue(context.Background(), myKey("b"), "b"),
	)

	value = writer.Context().Value(myKey("a"))
	require.Nil(t, value)

	value = writer.Context().Value(myKey("b"))
	require.NotNil(t, value)
	str, ok = value.(string)
	require.True(t, ok)
	assert.Equal(t, "b", str)
}

func TestWriteFlush(t *testing.T) {
	width, height := 5, 6
	writer := NewBufferWriter(context.Background(), width, height)

	c := 'E'
	for i := width - 1; i >= 0; i-- {
		for j := height - 1; j >= 0; j-- {
			if i > j-1 {
				writer.SetCell(term.Coordinates{X: j, Y: i}, term.Cell{Ch: c})
			}
		}
		c--
	}

	// should be fine to wtry to write out of bounds
	writer.SetCell(term.Coordinates{X: width + 1, Y: height + 1}, term.Cell{Ch: '='})

	require.NoError(t, writer.Flush())

	expected := "A\x00\x00\x00\x00\nBB\x00\x00\x00\n" +
		"CCC\x00\x00\nDDDD\x00\nEEEEE\n\x00\x00\x00\x00\x00"
	assert.Equal(t, expected, term.CellsToString(writer.RawCells()))
}

func benchBufferWriter(b *testing.B, n int) {
	width, height := n, n
	writer := NewBufferWriter(context.Background(), width, height)
	for i := 0; i < b.N; i++ {
		writer.Clear(term.Attributes{})
		c := 'E'
		for i := width - 1; i >= 0; i-- {
			for j := height - 1; j >= 0; j-- {
				if i > j-1 {
					writer.SetCell(term.Coordinates{X: j, Y: i}, term.Cell{Ch: c})
				}
			}
			c--
		}
	}
}

func BenchmarkBufferWriter10(b *testing.B) {
	benchBufferWriter(b, 10)
}
func BenchmarkBufferWriter100(b *testing.B) {
	benchBufferWriter(b, 100)
}
func BenchmarkBufferWriter1000(b *testing.B) {
	benchBufferWriter(b, 1000)
}
