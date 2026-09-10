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

package hooks

import "testing"

func TestMatchValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
		value   string
		want    bool
	}{
		{"empty matches all", "", "anything", true},
		{"star matches all", "*", "anything", true},

		{"exact single hit", "edit_file", "edit_file", true},
		{"exact single miss", "edit_file", "write_file", false},

		{"exact list hit first", "edit_file|write_file", "edit_file", true},
		{"exact list hit second", "edit_file|write_file", "write_file", true},
		{"exact list miss", "edit_file|write_file", "read_file", false},

		{"regex hit", "^Bash.*", "BashTool", true},
		{"regex miss", "^Bash.*", "Other", false},
		{"regex digits", `[0-9]+`, "abc123", true},

		{"invalid regex", "[abc", "value", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := matchValue(tc.pattern, tc.value); got != tc.want {
				t.Fatalf("matchValue(%q, %q) = %v, want %v", tc.pattern, tc.value, got, tc.want)
			}
		})
	}
}
