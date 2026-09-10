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

package markdown

import (
	"testing"

	"github.com/unstablebuild/rune-go-sdk/term"
)

func FuzzParse(f *testing.F) {
	f.Add("# Hello")
	f.Add("Hello **world**")
	f.Add("- item\n- item2")
	f.Add("```go\ncode\n```")
	f.Add("> quote")
	f.Add("---")
	f.Add("| a | b |\n|---|---|\n| 1 | 2 |")
	f.Add("")
	f.Add("# H1\n\n## H2\n\n### H3")
	f.Add("*italic* **bold** `code` ~~strike~~")
	f.Add("[link](http://example.com)")

	cfg := DefaultConfig()
	f.Fuzz(func(t *testing.T, input string) {
		_, err := parse(input, &cfg)
		if err != nil {
			t.Skip()
		}
	})
}

func FuzzRender(f *testing.F) {
	f.Add("# Hello", 80, 24)
	f.Add("Hello **world**", 40, 10)
	f.Add("- item\n- item2", 20, 5)
	f.Add("", 10, 10)
	f.Add("# Title\n\nParagraph", 1, 1)
	f.Add("Long text that needs to wrap around", 5, 3)

	f.Fuzz(func(t *testing.T, input string, width, height int) {
		if width < 0 || height < 0 || width > 1000 || height > 1000 {
			t.Skip()
		}

		md, err := New(input)
		if err != nil {
			t.Skip()
		}
		md.Resize(width, height)

		if width > 0 && height > 0 {
			w := term.NewStringWriter(width, height)
			md.Draw(w)
		}
	})
}

func FuzzScroll(f *testing.F) {
	f.Add("# H1\n\n# H2\n\n# H3", 10)
	f.Add("Line\nLine\nLine\nLine\nLine\nLine\nLine\nLine\nLine\nLine\n", 5)
	f.Add("Short", 100)

	f.Fuzz(func(t *testing.T, input string, scrollOps int) {
		if scrollOps < 0 || scrollOps > 1000 {
			t.Skip()
		}

		md, err := New(input)
		if err != nil {
			t.Skip()
		}
		md.Resize(40, 5)

		for range scrollOps {
			if scrollOps%2 == 0 {
				md.SeekDown()
			} else {
				md.SeekUp()
			}
		}

		if md.SeekOffset() < 0 {
			t.Errorf("offset became negative: %d", md.SeekOffset())
		}
		if md.SeekOffset() > md.MaxSeekOffset() {
			t.Errorf("offset exceeded max: %d > %d", md.SeekOffset(), md.MaxSeekOffset())
		}
	})
}
