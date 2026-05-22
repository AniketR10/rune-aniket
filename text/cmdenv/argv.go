// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package cmdenv

import (
	"strings"

	"mvdan.cc/sh/v3/pattern"
	"mvdan.cc/sh/v3/syntax"
)

// BuildPluginArgv prepares argv for the workspace executor. When args
// is already a plain command (no shell operators, no globs, no quoting,
// no parameter expansion) it is returned unchanged so the executor can
// fork/exec the program directly. Otherwise the args are joined and
// wrapped in `sh -c <quoted line>` so that the shell interprets pipes,
// redirections, command substitutions, globs, and the like.
//
// An empty args slice is returned unchanged; callers are expected to
// guard against the empty case before invoking the executor.
func BuildPluginArgv(args []string) []string {
	if IsPlainArgv(args) {
		return args
	}
	line := strings.Join(args, " ")
	return []string{"sh", "-c", Quote(line)}
}

// IsPlainArgv reports whether args, joined by a single space, parses
// as a single foreground POSIX command call whose tokens round-trip
// without any shell interpretation. Specifically, every arg must
// re-parse as one literal word with no quoting, no parameter
// expansion, no command substitution, no glob metacharacters, and no
// re-tokenisation (e.g. an embedded space would split one arg into
// two). Statements with leading assignments, background jobs (`&`),
// negation (`!`), or any I/O redirection are also rejected.
//
// IsPlainArgv returns false for an empty slice because there is no
// command to dispatch.
func IsPlainArgv(args []string) bool {
	if len(args) == 0 {
		return false
	}
	line := strings.Join(args, " ")
	file, err := syntax.NewParser().Parse(strings.NewReader(line), "")
	if err != nil {
		return false
	}
	if len(file.Stmts) != 1 {
		return false
	}
	stmt := file.Stmts[0]
	if stmt.Background || stmt.Negated || len(stmt.Redirs) > 0 {
		return false
	}
	call, ok := stmt.Cmd.(*syntax.CallExpr)
	if !ok {
		return false
	}
	if len(call.Assigns) > 0 {
		return false
	}
	if len(call.Args) != len(args) {
		// Re-tokenisation merged or split args; the joined form is not
		// a faithful representation. Be safe and use `sh -c`.
		return false
	}
	for i, word := range call.Args {
		if len(word.Parts) != 1 {
			return false
		}
		lit, ok := word.Parts[0].(*syntax.Lit)
		if !ok {
			return false
		}
		if lit.Value != args[i] {
			return false
		}
		if pattern.HasMeta(lit.Value, 0) {
			return false
		}
		// pattern.HasMeta covers POSIX glob metacharacters but not
		// the shell escape character. A backslash in a literal is
		// a quoting hint that the user expected the shell to
		// interpret, so route through `sh -c`.
		if strings.ContainsRune(lit.Value, '\\') {
			return false
		}
		// A leading tilde at the start of a literal is shell
		// tilde-expansion (~ → $HOME, ~user → /home/user). The
		// parser leaves it intact, but exec'ing the program with
		// a literal `~/...` argv element is not what the user
		// wrote. Route through `sh -c` so the shell expands.
		if strings.HasPrefix(lit.Value, "~") {
			return false
		}
	}
	return true
}
