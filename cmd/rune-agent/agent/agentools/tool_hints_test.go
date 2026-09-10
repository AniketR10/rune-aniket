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

package agentools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBashToolHint(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string // substring expected in hint, or empty for no hint
	}{
		// grep-family
		{"grep", "grep -rn 'func main' .", "search_content"},
		{"rg", "rg --type go 'Handler'", "search_content"},
		{"ag", "ag 'TODO' src/", "search_content"},
		{"ack", "ack 'FIXME'", "search_content"},

		// find
		{"find", "find . -name '*.go'", "find_files"},

		// cat/head/tail
		{"cat", "cat main.go", "read_file"},
		{"head", "head -n 50 main.go", "read_file"},
		{"tail", "tail -20 main.go", "read_file"},

		// formatters
		{"gofmt", "gofmt -w main.go", "format_file"},
		{"goimports", "goimports -w .", "format_file"},
		{"go fmt", "go fmt ./...", "format_file"},

		// sed
		{"sed", "sed -i 's/old/new/g' file.go", "apply_patch"},

		// wc -l
		{"wc -l", "wc -l main.go", "read_file"},

		// negative cases — should NOT trigger any hint
		{"go test", "go test ./...", ""},
		{"go run", "go run main.go", ""},
		{"make", "make build", ""},
		{"npm install", "npm install", ""},
		{"echo", "echo hello", ""},
		{"git status", "git status", ""},
		{"docker build", "docker build -t app .", ""},
		{"find_no_space", "findutils-config", ""}, // "find" without trailing space
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hint := bashToolHint(tt.command)
			if tt.want == "" {
				assert.Empty(t, hint, "expected no hint for %q", tt.command)
			} else {
				assert.Contains(t, hint, tt.want,
					"hint for %q should mention %q", tt.command, tt.want)
			}
		})
	}
}

func TestRedundantCDHint(t *testing.T) {
	const root = "/Users/me/src/rune"

	tests := []struct {
		name    string
		command string
		workDir string
		want    bool
	}{
		{"cd root and command", "cd /Users/me/src/rune && make test", root, true},
		{"cd root trailing slash", "cd /Users/me/src/rune/ && ls", root, true},
		{"cd root quoted", `cd "/Users/me/src/rune" && ls`, root, true},
		{"cd root single quoted", "cd '/Users/me/src/rune' && ls", root, true},
		{"cd root semicolon", "cd /Users/me/src/rune; ls", root, true},
		{"cd root only", "cd /Users/me/src/rune", root, true},
		{"cd dot", "cd . && ls", root, true},
		{"cd relative to root", "cd ./ && ls", root, true},
		{"cd relative up and back", "cd ../rune && ls", root, true},
		{"cd relative through subdirectory", "cd sub/.. && ls", root, true},
		{"cd matches working_dir", "cd /Users/me/src/rune/sub && ls", "/Users/me/src/rune/sub", true},

		{"cd subdirectory", "cd /Users/me/src/rune/sub && ls", root, false},
		{"cd relative subdirectory", "cd sub && ls", root, false},
		{"cd relative sibling", "cd ../blue && ls", root, false},
		{"cd parent", "cd .. && ls", root, false},
		{"no cd", "make test", root, false},
		{"cd not leading", "make test && cd /Users/me/src/rune", root, false},
		{"cd with expansion", "cd $ROOT && ls", root, false},
		{"cd home", "cd && ls", root, false},
		{"unparseable", "cd /Users/me/src/rune && (", root, false},
		{"empty workDir", "cd /Users/me/src/rune && ls", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hint := redundantCDHint(tt.command, tt.workDir)
			if tt.want {
				assert.Contains(t, hint, "redundant", "expected hint for %q", tt.command)
			} else {
				assert.Empty(t, hint, "expected no hint for %q", tt.command)
			}
		})
	}
}

func TestBashHintsIncludesBothHints(t *testing.T) {
	hints := bashHints("cd /repo && cat main.go", "/repo")
	assert.Len(t, hints, 2)
	assert.Contains(t, hints[0], "redundant")
	assert.Contains(t, hints[1], "read_file")
}
