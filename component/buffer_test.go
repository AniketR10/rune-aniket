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

package component

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/rune/cell"
)

func TestResponsiveStringDraw(t *testing.T) {
	t.Run("should behave like String with single line strings and enough space", func(t *testing.T) {
		tcases := []struct {
			in  string
			out string
			cfg component.StringResponsiveConfig
		}{
			{
				in:  "aaaa",
				out: "aaaa \n     \n     \n     \n     ",
			},
			{
				in:  "X\nX\nX\nX\nX\nX\nX\nX\n",
				out: "X    \nX    \nX    \nX    \nX    ",
			},
			{
				in:  "a",
				out: "a    \n     \n     \n     \n     ",
			},
		}

		for _, tcase := range tcases {
			t.Run("Buffer", func(t *testing.T) {
				testString(t, func(in string) tui.Component {
					b := cell.NewBuffer()
					b.WriteString(in)
					return Buffer(b, tcase.cfg)
				}, 5, 5, tcase.in, tcase.out)
			})
		}
	})

	t.Run("should not split words in half", func(t *testing.T) {
		tcases := []struct {
			in  string
			out string
			cfg component.StringResponsiveConfig
		}{
			{
				in:  "XX XX XX\nYYYYY YYYY\nZZZZZZZZZZZZ ZZZZZZZZZZZZ ZZZZZZZZZZZZ",
				out: "XX   \nXX XX\nYYYYY\n YYYY\nZZZZZ",
				cfg: component.StringResponsiveConfig{NoSplitWords: true},
			},
		}

		for _, tcase := range tcases {
			t.Run("BufferResponsive", func(t *testing.T) {
				testString(t, func(in string) tui.Component {
					b := cell.NewBuffer()
					b.WriteString(in)
					return Buffer(b, tcase.cfg)
				}, 5, 5, tcase.in, tcase.out)
			})
		}
	})

	t.Run("should wrap lines around", func(t *testing.T) {
		tcases := []struct {
			in     string
			out    string
			cfg    component.StringResponsiveConfig
			height int
		}{
			{
				in:  "XXXXXXXXXX\nBBBBBBBBBB\nCCCCCCCCCCCC\nDDDDDDDDDD\nEEEEEEEEEEE\nFFFFFFFFFF\n",
				out: "XXXXX\nXXXXX\nBBBBB\nBBBBB\nCCCCC",
			},
			{
				in:  "XXXXXXXXXXXXXXXX\nYYYYYYYYYnZZZZZZZZZ",
				out: "XXXXX\nXXXXX\nXXXXX\nX    \nYYYYY",
			},
			{
				in:  "X\n1111 \n222222222222222222222222222222222",
				out: "X    \n1111 \n22222\n22222\n22222",
			},
			{
				in: "X\n111\n222222222222222222222222222222222",
				out: `┌───┐
│X  │
│111│
│222│
└───┘`,
				cfg: component.StringResponsiveConfig{StringConfig: component.StringConfig{FrameCharSet: component.FrameCharSetDefault()}},
			},
			{
				in: "XXXX\n111\n22",
				out: `┌───┐
│XXX│
│X  │
│111│
└───┘`,
				cfg: component.StringResponsiveConfig{StringConfig: component.StringConfig{FrameCharSet: component.FrameCharSetDefault()}},
			},
			{
				in: "XXXXXXXXXX\nBBBBBBBBBB\nCCCCCCCCCCCC\nDDDDDDDDDD\nEEEEEEEEEEE\nFFFFFFFFFF",
				cfg: component.StringResponsiveConfig{
					StringConfig: component.StringConfig{
						Alignment:         component.AlignmentCentered,
						FrameCharSet:      component.FrameCharSetDefault(),
						PaddingVertical:   2,
						PaddingHorizontal: 2,
					},
				},
				height: 67,
				out: `┌───┐
│   │
│ X │
│ X │
│ X │
│ X │
│ X │
│ X │
│ X │
│ X │
│ X │
│ X │
│ B │
│ B │
│ B │
│ B │
│ B │
│ B │
│ B │
│ B │
│ B │
│ B │
│ C │
│ C │
│ C │
│ C │
│ C │
│ C │
│ C │
│ C │
│ C │
│ C │
│ C │
│ C │
│ D │
│ D │
│ D │
│ D │
│ D │
│ D │
│ D │
│ D │
│ D │
│ D │
│ E │
│ E │
│ E │
│ E │
│ E │
│ E │
│ E │
│ E │
│ E │
│ E │
│ E │
│ F │
│ F │
│ F │
│ F │
│ F │
│ F │
│ F │
│ F │
│ F │
│ F │
│   │
└───┘`,
			},
		}

		for _, tcase := range tcases {
			t.Run("BufferResponsive", func(t *testing.T) {
				height := 5
				if tcase.height != 0 {
					height = tcase.height
				}
				testString(t, func(in string) tui.Component {
					b := cell.NewBuffer()
					b.WriteString(in)
					return Buffer(b, tcase.cfg)
				}, 5, height, tcase.in, tcase.out)
			})
		}
	})
}

func TestResponsiveHeight(t *testing.T) {
	tcases := []struct {
		in    string
		width int
		out   int
		cfg   component.StringResponsiveConfig
	}{
		{
			in:    "XXXXXXXXXX\nBBBBBBBBBB\nCCCCCCCCCCCC\nDDDDDDDDDD\nEEEEEEEEEEE\nFFFFFFFFFF\n",
			width: 5,
			out:   15,
		},
		{
			in:    "X",
			width: 5,
			out:   1,
		},
		{
			in:    "X\nX\nX\nX\n",
			width: 10,
			out:   5,
		},
		{
			in:    "XXXXXXXXXX",
			width: 10,
			out:   1,
		},
		{
			in:    "XXXXXXXXXX",
			width: -1,
			out:   0,
		},
		{
			in:    "XXXXXXXXXX",
			width: 0, // could trigger division by zero
			out:   0,
		},
		{
			in:    "X\nX\nX\nX\n",
			width: 10,
			out:   7,
			cfg:   component.StringResponsiveConfig{StringConfig: component.StringConfig{FrameCharSet: component.FrameCharSetDefault()}},
		},
		{
			in:    "X",
			width: 5,
			out:   3,
			cfg:   component.StringResponsiveConfig{StringConfig: component.StringConfig{FrameCharSet: component.FrameCharSetDefault()}},
		},
		{
			in:    "XXXXXXXXXX",
			width: 10,
			out:   4,
			cfg:   component.StringResponsiveConfig{StringConfig: component.StringConfig{FrameCharSet: component.FrameCharSetDefault()}},
		},
		{
			in:    "XXXXXXXXXX\nBBBBBBBBBB\nCCCCCCCCCCCC\nDDDDDDDDDD\nEEEEEEEEEEE\nFFFFFFFFFF\n",
			width: 5,
			out:   27,
			cfg:   component.StringResponsiveConfig{StringConfig: component.StringConfig{FrameCharSet: component.FrameCharSetDefault()}},
		},
		{
			in:    "XXXXXXXXXX\nBBBBBBBBBB\nCCCCCCCCCCCC\nDDDDDDDDDD\nEEEEEEEEEEE\nFFFFFFFFFF\n",
			width: 5,
			out:   68,
			cfg: component.StringResponsiveConfig{
				StringConfig: component.StringConfig{
					Alignment:         component.AlignmentCentered,
					FrameCharSet:      component.FrameCharSetDefault(),
					PaddingVertical:   2,
					PaddingHorizontal: 2,
				},
			},
		},
	}

	for _, tcase := range tcases {
		t.Run("BufferResponsive", func(t *testing.T) {
			b := cell.CellsToBuffer(nil)
			b.WriteString(tcase.in)
			s := Buffer(b, tcase.cfg)
			out := s.Height(tcase.width)
			assert.Equal(t, tcase.out, out)
		})
	}
}

func TestBufferWithEdits(t *testing.T) {
	t.Run("Height", func(t *testing.T) {
		b := cell.CellsToBuffer(nil)
		b.WriteString("aa")
		s := Buffer(b, component.StringResponsiveConfig{})

		height := s.Height(1)
		assert.Equal(t, 2, height)

		b.WriteString("a")

		height = s.Height(1)
		assert.Equal(t, 3, height)
	})

	t.Run("Draw", func(t *testing.T) {
		w := term.NewStringWriter(5, 5)
		b := cell.CellsToBuffer(nil)
		b.WriteString("a\nb\nc")
		s := Buffer(b, component.StringResponsiveConfig{})
		s.Resize(4, 4)

		s.Draw(w)
		require.NoError(t, w.Flush())
		assert.Equal(t, "a    \nb    \nc    \n     \n     ", w.String())

		b.WriteString("xyz")

		w = term.NewStringWriter(5, 5)
		s.Draw(w)
		require.NoError(t, w.Flush())
		assert.Equal(t, "a    \nb    \ncxyz \n     \n     ", w.String())
	})
}

func testString(t *testing.T,
	fn func(string) tui.Component,
	width, height int, in, out string,
) {
	w := term.NewStringWriter(width, height)
	comp := fn(in)
	comp.Resize(width, height)
	comp.Draw(w)
	require.NoError(t, w.Flush())
	assert.Equal(t, out, w.String())
}
