// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

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
