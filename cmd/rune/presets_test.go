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

package main

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var presetFiles = []string{
	"preset_modal.yaml",
	"preset_standard_darwin.yaml",
	"preset_standard_linux.yaml",
	"preset_emacs.yaml",
}

// commentedOption matches a commented-out YAML mapping entry or nested
// comment, as opposed to the prose in a section banner.
var commentedOption = regexp.MustCompile(
	`^# ( *(?:#.*|(?:[a-z_0-9]+|"[^"]*"|'[^']*') *:.*))$`)

func TestPresetCommentedBlocksUncomment(t *testing.T) {
	for _, name := range presetFiles {
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		var shipped map[string]any
		if err := yaml.Unmarshal(raw, &shipped); err != nil {
			t.Fatalf("%s as shipped: %v", name, err)
		}
		var out []string
		for _, l := range strings.Split(string(raw), "\n") {
			if m := commentedOption.FindStringSubmatch(l); m != nil {
				out = append(out, m[1])
				continue
			}
			if strings.HasPrefix(l, "#") {
				continue
			}
			out = append(out, l)
		}
		var full map[string]any
		if err := yaml.Unmarshal([]byte(strings.Join(out, "\n")), &full); err != nil {
			t.Errorf("%s fully uncommented: %v", name, err)
			continue
		}
		t.Logf("%s: shipped=%d keys, uncommented=%d keys",
			name, len(shipped), len(full))
	}
}
