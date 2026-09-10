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

package vtescanner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGroundRun(t *testing.T) {
	cases := []struct {
		name  string
		state State
		buf   string
		want  int
	}{
		{"full printable run", Ground, "hello world", 11},
		{"stops at control byte", Ground, "abc\ndef", 3},
		{"stops at escape", Ground, "abc\x1b[31m", 3},
		{"stops at DEL", Ground, "abc\x7fdef", 3},
		{"includes utf8 sequence", Ground, "abc\xc3\xa9", 5},
		{"includes multi-byte utf8", Ground, "a漢字🙂", 11},
		{"stops before truncated 2 byte", Ground, "abc\xc3", 3},
		{"stops before truncated 3 byte", Ground, "abc\xe6\xbc", 3},
		{"stops before truncated 4 byte", Ground, "abc\xf0\x9f\x98", 3},
		{"stops at stray continuation", Ground, "abc\x80\xbf", 3},
		{"stops at overlong", Ground, "abc\xc0\x80", 3},
		{"stops at surrogate", Ground, "abc\xed\xa0\x80", 3},
		{"stops at out of range lead", Ground, "abc\xf5\x80\x80\x80", 3},
		{"keeps literal replacement char", Ground, "abc\ufffd", 6},
		{"stops at control after utf8", Ground, "漢字\r", 6},
		{"leading control", Ground, "\rabc", 0},
		{"leading utf8", Ground, "漢abc", 6},
		{"empty buffer", Ground, "", 0},
		{"non-ground state", CsiParam, "hello", 0},
		{"utf8 state", Utf8, "hello", 0},
		{"space and tilde boundary", Ground, " ~", 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewScanner(&testDispatcher{})
			s.state = tc.state
			assert.Equal(t, tc.want, s.GroundRun([]byte(tc.buf)))
		})
	}
}
