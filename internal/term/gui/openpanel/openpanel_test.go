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

package openpanel

import (
	"reflect"
	"testing"
)

func TestFinishRoutesByTag(t *testing.T) {
	var first, second [][]string
	firstTag := register(func(paths []string) { first = append(first, paths) })
	secondTag := register(func(paths []string) { second = append(second, paths) })

	finish(secondTag, []string{"/b"})
	finish(firstTag, []string{"/a1", "/a2"})

	if want := [][]string{{"/a1", "/a2"}}; !reflect.DeepEqual(first, want) {
		t.Errorf("first = %v, want %v", first, want)
	}
	if want := [][]string{{"/b"}}; !reflect.DeepEqual(second, want) {
		t.Errorf("second = %v, want %v", second, want)
	}
}

func TestFinishCancelPassesNil(t *testing.T) {
	var got []string
	called := false
	tag := register(func(paths []string) {
		called = true
		got = paths
	})

	finish(tag, nil)

	if !called {
		t.Fatal("done was not invoked on cancel")
	}
	if got != nil {
		t.Errorf("paths = %v, want nil", got)
	}
}

func TestFinishIsSingleUse(t *testing.T) {
	calls := 0
	tag := register(func([]string) { calls++ })

	finish(tag, nil)
	finish(tag, []string{"/late"})

	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestFinishUnknownTagIsIgnored(t *testing.T) {
	finish(0, nil)
	finish(1<<30, []string{"/x"})
}

func TestSplitPaths(t *testing.T) {
	tests := []struct {
		name   string
		joined string
		want   []string
	}{
		{name: "empty payload signals cancel", joined: "", want: nil},
		{name: "single path", joined: "/tmp/a.txt", want: []string{"/tmp/a.txt"}},
		{
			name:   "multiple paths",
			joined: "/tmp/a.txt\x00/tmp/dir b/c.txt",
			want:   []string{"/tmp/a.txt", "/tmp/dir b/c.txt"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := splitPaths(tt.joined); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitPaths(%q) = %v, want %v", tt.joined, got, tt.want)
			}
		})
	}
}
