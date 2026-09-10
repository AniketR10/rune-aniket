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
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// bashHints returns the hints to append to a bash tool result for a
// command that ran in workDir.
func bashHints(command, workDir string) []string {
	var hints []string
	if hint := redundantCDHint(command, workDir); hint != "" {
		hints = append(hints, hint)
	}
	if hint := bashToolHint(command); hint != "" {
		hints = append(hints, hint)
	}
	return hints
}

// redundantCDHint reports that a leading `cd` into the directory the
// tool already runs in is unnecessary. Models routinely prefix every
// command with `cd <workspace root> &&`, which burns tokens and hides
// the fact that the tool is already anchored there.
func redundantCDHint(command, workDir string) string {
	target, ok := leadingCDTarget(command)
	if !ok || workDir == "" {
		return ""
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(workDir, target)
	}
	if filepath.Clean(target) != filepath.Clean(workDir) {
		return ""
	}
	return fmt.Sprintf(
		"This command already ran in %s, so the leading `cd` was redundant. "+
			"The bash tool always starts in the workspace root; use the working_dir "+
			"parameter when you need a different directory.",
		filepath.Clean(workDir),
	)
}

// leadingCDTarget extracts the literal argument of a `cd` that runs as
// the first command of the script, including behind `&&`/`;`/`||`.
// Returns ("", false) when the first command is not a statically
// resolvable single-argument `cd`.
func leadingCDTarget(script string) (string, bool) {
	script = strings.TrimSpace(script)
	if script == "" {
		return "", false
	}
	file, err := syntax.NewParser().Parse(strings.NewReader(script), "")
	if err != nil || file == nil || len(file.Stmts) == 0 {
		return "", false
	}
	cmd := file.Stmts[0].Cmd
	// `a && b && c` parses left-associatively, so the leading command
	// is the leftmost leaf.
	for {
		bin, ok := cmd.(*syntax.BinaryCmd)
		if !ok || bin.X == nil {
			break
		}
		cmd = bin.X.Cmd
	}
	call, ok := cmd.(*syntax.CallExpr)
	if !ok || len(call.Args) != 2 || len(call.Assigns) != 0 {
		return "", false
	}
	if name, ok := literalProgramName(call.Args[0]); !ok || name != "cd" {
		return "", false
	}
	return literalProgramName(call.Args[1])
}

// bashToolHint inspects a shell command string and returns a hint suggesting
// a dedicated built-in tool when the command could be replaced by one.
// Returns empty string when no hint applies.
func bashToolHint(command string) string {
	for _, h := range toolHintRules {
		if h.pattern.MatchString(command) {
			return h.hint
		}
	}
	return ""
}

type toolHintRule struct {
	pattern *regexp.Regexp
	hint    string
}

var toolHintRules = []toolHintRule{
	{
		pattern: regexp.MustCompile(`\b(grep|rg|ag|ack)\b`),
		hint:    "Use the search_content or find_files tool instead of grep/rg for text search, or search_symbols/find_definition for symbol search.",
	},
	{
		pattern: regexp.MustCompile(`\bfind\s`),
		hint:    "Use the find_files tool instead of the find command.",
	},
	{
		pattern: regexp.MustCompile(`\b(cat|head|tail|less|more)\b`),
		hint:    "Use the read_file tool instead of cat/head/tail to read file contents.",
	},
	{
		pattern: regexp.MustCompile(`\b(gofmt|goimports|go\s+fmt)\b`),
		hint:    "Use the format_file tool instead of running formatters via shell.",
	},
	{
		pattern: regexp.MustCompile(`\bsed\b`),
		hint:    "Use apply_patch or rename_symbol instead of sed for file modifications.",
	},
	{
		pattern: regexp.MustCompile(`\bwc\s+-l\b`),
		hint:    "Use read_file or outline_file to inspect file contents instead of wc.",
	},
}
