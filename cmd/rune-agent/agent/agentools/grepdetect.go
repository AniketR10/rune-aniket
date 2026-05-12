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
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/configedit"
)

// grepProgramNames lists the search programs we want to intercept. They
// are matched after stripping the directory part, so absolute paths and
// `command grep` style invocations are also covered.
var grepProgramNames = map[string]bool{
	"grep": true,
	"rg":   true,
	"ag":   true,
	"ack":  true,
}

// grepDetectNoForkBuiltins are shell built-ins that do not fork a
// separate process. A bare grep invocation may be preceded by any number
// of these without changing the fact that the only forked command is
// grep.
var grepDetectNoForkBuiltins = map[string]bool{
	"cd": true, "pushd": true, "popd": true,
	"export": true, "set": true, "unset": true,
	"readonly": true, "local": true, "declare": true, "typeset": true,
	"alias": true, "unalias": true,
	":": true, "true": true, "false": true,
}

// isBareGrepCommand reports whether script reduces to a single forked
// invocation of grep/rg/ag/ack, optionally preceded by no-fork builtins
// such as `cd <path>`, with no pipes, redirections, command
// substitutions, or other forked commands.
func isBareGrepCommand(script string) bool {
	script = strings.TrimSpace(script)
	if script == "" {
		return false
	}
	file, err := syntax.NewParser().Parse(strings.NewReader(script), "")
	if err != nil || file == nil {
		return false
	}
	sawGrep := false
	for _, stmt := range file.Stmts {
		ok, hit := grepStmt(stmt)
		if !ok {
			return false
		}
		if hit {
			if sawGrep {
				return false
			}
			sawGrep = true
		}
	}
	return sawGrep
}

// grepStmt walks a single statement. It returns (ok, hit) where ok is
// false if the statement contains anything we cannot reduce (pipes,
// redirections to files, command substitution, unknown forked commands)
// and hit is true if this statement is the bare grep invocation.
func grepStmt(stmt *syntax.Stmt) (bool, bool) {
	if stmt == nil {
		return true, false
	}
	// Any redirection (>, >>, <, etc.) breaks the "bare grep" shape.
	if len(stmt.Redirs) > 0 {
		return false, false
	}
	return grepCommand(stmt.Cmd)
}

func grepCommand(cmd syntax.Command) (bool, bool) {
	switch c := cmd.(type) {
	case *syntax.CallExpr:
		return grepCallExpr(c)
	case *syntax.BinaryCmd:
		// Only && and ; (handled at statement level) chains are
		// acceptable. Treat ||, |, and |& as opaque since they affect
		// how grep's output is consumed.
		if c.Op != syntax.AndStmt {
			return false, false
		}
		okX, hitX := grepStmt(c.X)
		if !okX {
			return false, false
		}
		okY, hitY := grepStmt(c.Y)
		if !okY {
			return false, false
		}
		if hitX && hitY {
			return false, false
		}
		return true, hitX || hitY
	default:
		return false, false
	}
}

func grepCallExpr(call *syntax.CallExpr) (bool, bool) {
	if len(call.Assigns) > 0 {
		// Pure environment assignments aren't grep; treat any leading
		// VAR=val assignment as opaque so we don't accidentally allow a
		// grep invocation with environment injection through.
		if len(call.Args) == 0 {
			return true, false
		}
		return false, false
	}
	if len(call.Args) == 0 {
		return true, false
	}
	name, ok := grepLiteralWord(call.Args[0])
	if !ok {
		return false, false
	}
	base := filepath.Base(name)
	if grepDetectNoForkBuiltins[base] {
		// A no-fork builtin like `cd /tmp` is allowed; ensure its args
		// don't contain command substitution.
		for _, w := range call.Args[1:] {
			if !grepArgWordPure(w) {
				return false, false
			}
		}
		return true, false
	}
	if !grepProgramNames[base] {
		return false, false
	}
	// The forked command is grep. Reject if any argument contains
	// command substitution, process substitution, or other forks.
	for _, w := range call.Args[1:] {
		if !grepArgWordPure(w) {
			return false, false
		}
	}
	return true, true
}

// grepLiteralWord returns the literal value of a word in command-name
// position. Variable expansion or command substitution is not allowed.
func grepLiteralWord(word *syntax.Word) (string, bool) {
	if word == nil || len(word.Parts) == 0 {
		return "", false
	}
	var b strings.Builder
	for _, part := range word.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			b.WriteString(p.Value)
		case *syntax.SglQuoted:
			b.WriteString(p.Value)
		case *syntax.DblQuoted:
			for _, inner := range p.Parts {
				lit, ok := inner.(*syntax.Lit)
				if !ok {
					return "", false
				}
				b.WriteString(lit.Value)
			}
		default:
			return "", false
		}
	}
	name := strings.TrimSpace(b.String())
	if name == "" || strings.ContainsAny(name, " \t\r\n") {
		return "", false
	}
	return name, true
}

// grepArgWordPure reports whether word can be evaluated without forking
// another process: literals, single/double-quoted strings (with no
// command substitution inside), parameter and arithmetic expansions are
// allowed; command and process substitutions are not.
func grepArgWordPure(word *syntax.Word) bool {
	if word == nil {
		return true
	}
	return grepArgPartsPure(word.Parts)
}

func grepArgPartsPure(parts []syntax.WordPart) bool {
	for _, part := range parts {
		switch p := part.(type) {
		case *syntax.Lit, *syntax.SglQuoted, *syntax.ParamExp, *syntax.ArithmExp:
			// pure
		case *syntax.DblQuoted:
			if !grepArgPartsPure(p.Parts) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// forceBuiltinToolsKey is the configedit key consulted by the bash/
// exec_command grep guard. true = always reject bare grep invocations;
// false = always allow; absent = ask the user.
const forceBuiltinToolsKey = "force_builtin_tools"

// setForceBuiltinTools persists force_builtin_tools to the workspace
// config. Subsequent Resolve calls on the corresponding configedit.Bool
// observe the new value because cfg.SetBool updates the in-memory
// overlay.
func setForceBuiltinTools(ctx context.Context, cfg configedit.Setter, v bool) error {
	return cfg.SetBool(ctx, forceBuiltinToolsKey, v)
}

// builtinToolsErrorMessage returns the canonical message returned to
// the model when a grep invocation is rejected. The exact tool names
// depend on the provider's tool set (e.g. search_content vs
// grep_files).
func builtinToolsErrorMessage(provider string) string {
	searchTool := "search_content"
	switch provider {
	case "openai", "codex":
		searchTool = "grep_files"
	}
	return "Do not use grep for searching. Prefer Rune's semantic " +
		"tools, which understand code structure:\n" +
		"  • `find_definition` (takes a symbol name) — locate where a " +
		"function/type/var is defined.\n" +
		"  • `find_references` (takes a symbol name) — list every use " +
		"site of a symbol across the workspace.\n" +
		"  • `find_implementations` (takes an interface name) — list " +
		"all concrete types that satisfy an interface.\n" +
		"  • `describe_symbol` (takes a symbol name) — show the type " +
		"signature and doc comment for a symbol without reading the file.\n" +
		"  • `search_symbols` (takes a fuzzy query) — find a symbol " +
		"by partial or approximate name when the exact name is unknown.\n" +
		"  • `outline_file` (takes a file path) — inspect a file's " +
		"top-level structure instead of reading the whole file with " +
		"`read_file`.\n" +
		"Only when you are searching for a non-symbol string (a comment, " +
		"error message, or literal value) use the `" + searchTool +
		"` tool."
}

// grepGuard encapsulates the dependencies needed to decide whether a
// bare grep invocation should be allowed. The prompter is read from
// the per-call context (agent.PrompterFromContext); the configuration
// is consulted for the persisted user preference.
//
// forceBuiltinTools is the deferred configedit.Bool view of the
// "force_builtin_tools" key. It is built once at construction time
// and resolved per call so writes through cfg.SetBool (Always/Never
// choices) are observed immediately in the same session.
type grepGuard struct {
	cfg               configedit.Config
	forceBuiltinTools configedit.Bool
}

// newGrepGuard returns a grepGuard wired to cfg. Panics if cfg is nil.
func newGrepGuard(cfg configedit.Config) grepGuard {
	if cfg == nil {
		panic("agentools: grepGuard cfg must not be nil")
	}
	return grepGuard{
		cfg:               cfg,
		forceBuiltinTools: cfg.GetBool(forceBuiltinToolsKey),
	}
}

// decideGrep decides whether to reject a bare grep invocation. The
// returned (rejected, decided) pair has these meanings:
//   - decided == false: caller should run the command (no guard is
//     active, or no prompter is configured to ask the user).
//   - decided == true && rejected == true: caller should return the
//     canonical "use builtin tools" error.
//   - decided == true && rejected == false: user explicitly allowed
//     this invocation; caller should run the command.
func (g grepGuard) decideGrep(ctx context.Context) (rejected, decided bool) {
	v, err := g.forceBuiltinTools.Resolve(ctx)
	switch {
	case err == nil:
		return v, true
	case errors.Is(err, configedit.ErrNotFound):
		// fall through to prompt
	default:
		slog.Warn("get force_builtin_tools from config", "error", err)
	}
	prompter := agent.PrompterFromContext(ctx)
	if prompter == nil {
		// No way to ask the user; default to allowing the command so
		// that workflows without a prompter (tests, e2e tools) are not
		// silently blocked.
		return false, false
	}
	resp, err := prompter.Prompt(ctx, agent.PromptRequest{
		Title:  "Allow grep?",
		Header: "grep",
		Body: "The model is trying to run `grep`/`rg`. Rune has builtin " +
			"semantic search tools that are usually better. Allow this " +
			"call anyway?",
		Options: []agent.PromptOption{
			{Value: "yes", Label: "Yes", Description: "Allow this single call"},
			{Value: "always", Label: "Always", Description: "Allow grep; remember the choice"},
			{Value: "no", Label: "No", Description: "Reject this single call"},
			{Value: "never", Label: "Never", Description: "Reject grep; remember the choice"},
		},
	})
	if err != nil {
		// Dismissed: treat as "no" — reject this single call.
		return true, true
	}
	var choice string
	if len(resp.Values) > 0 {
		choice = resp.Values[0]
	}
	switch choice {
	case "yes":
		return false, true
	case "always":
		if err := setForceBuiltinTools(ctx, g.cfg, false); err != nil {
			slog.Warn("persist force_builtin_tools=false", "error", err)
		}
		return false, true
	case "no":
		return true, true
	case "never":
		if err := setForceBuiltinTools(ctx, g.cfg, true); err != nil {
			slog.Warn("persist force_builtin_tools=true", "error", err)
		}
		return true, true
	default:
		// Unknown response: be safe and reject.
		return true, true
	}
}
