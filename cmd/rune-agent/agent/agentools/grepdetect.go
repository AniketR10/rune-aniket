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

// grepWrapperPrograms invoke another command as their effective
// payload. When one of these is the program word, we keep scanning
// the remaining literal arguments for a grep-like program name so
// `find . | xargs grep foo` is intercepted just like a plain
// `grep foo` would be.
var grepWrapperPrograms = map[string]bool{
	"xargs":    true,
	"env":      true,
	"time":     true,
	"command":  true,
	"exec":     true,
	"sudo":     true,
	"doas":     true,
	"nohup":    true,
	"nice":     true,
	"ionice":   true,
	"watch":    true,
	"timeout":  true,
	"stdbuf":   true,
	"parallel": true,
}

// isGrepInvocation reports whether the script invokes grep/rg/ag/ack
// anywhere — directly, behind pipes, inside command substitution, after
// env assignments, on either side of && / ||, or as a redirection
// source/target. The previous "bare grep only" check let the model
// trivially escape with `grep ... 2>/dev/null | sort` or `cat | grep`,
// so we now treat any of those wrappers as a positive match.
//
// Known gap: we do not re-parse the body of `bash -c "..."`. A model
// determined enough to wrap grep in a subshell string still gets
// through, but the canonical evasion patterns from real chats no
// longer work.
func isGrepInvocation(script string) bool {
	script = strings.TrimSpace(script)
	if script == "" {
		return false
	}
	file, err := syntax.NewParser().Parse(strings.NewReader(script), "")
	if err != nil || file == nil {
		return false
	}
	for _, stmt := range file.Stmts {
		if stmtMentionsGrep(stmt) {
			return true
		}
	}
	return false
}

// stmtMentionsGrep returns true if the statement or any of its
// pipeline/redirection components invokes a grep-like program.
func stmtMentionsGrep(stmt *syntax.Stmt) bool {
	if stmt == nil {
		return false
	}
	// Redirections (e.g. `< $(grep ...)`) may themselves contain a
	// command substitution that invokes grep.
	for _, r := range stmt.Redirs {
		if wordMentionsGrep(r.Word) || wordMentionsGrep(r.Hdoc) {
			return true
		}
	}
	return commandMentionsGrep(stmt.Cmd)
}

// commandMentionsGrep walks every command shape the parser can produce
// and reports whether grep appears anywhere inside.
func commandMentionsGrep(cmd syntax.Command) bool {
	switch c := cmd.(type) {
	case nil:
		return false
	case *syntax.CallExpr:
		return callExprMentionsGrep(c)
	case *syntax.BinaryCmd:
		// && || | |& — any side may carry grep.
		return stmtMentionsGrep(c.X) || stmtMentionsGrep(c.Y)
	case *syntax.Subshell:
		return stmtsMentionGrep(c.Stmts)
	case *syntax.Block:
		return stmtsMentionGrep(c.Stmts)
	case *syntax.IfClause:
		return stmtsMentionGrep(c.Cond) ||
			stmtsMentionGrep(c.Then) ||
			commandMentionsGrep(c.Else)
	case *syntax.WhileClause:
		return stmtsMentionGrep(c.Cond) || stmtsMentionGrep(c.Do)
	case *syntax.ForClause:
		return stmtsMentionGrep(c.Do)
	case *syntax.CaseClause:
		for _, item := range c.Items {
			if stmtsMentionGrep(item.Stmts) {
				return true
			}
		}
		return false
	case *syntax.FuncDecl:
		if c.Body != nil {
			return commandMentionsGrep(c.Body.Cmd)
		}
		return false
	default:
		return false
	}
}

func stmtsMentionGrep(stmts []*syntax.Stmt) bool {
	for _, s := range stmts {
		if stmtMentionsGrep(s) {
			return true
		}
	}
	return false
}

// callExprMentionsGrep checks whether a simple command invokes grep,
// taking into account leading environment assignments (`FOO=bar grep
// ...`) and any grep tucked inside argument-level command substitution.
func callExprMentionsGrep(call *syntax.CallExpr) bool {
	if call == nil {
		return false
	}
	// `VAR=$(grep ...)` style — env values may themselves contain grep.
	for _, a := range call.Assigns {
		if wordMentionsGrep(a.Value) {
			return true
		}
	}
	// Walk literal words starting at args[0]. If we see a grep-like
	// program, return true. If we see a wrapper program (xargs, env,
	// sudo, ...), keep walking — wrappers turn the next literal word
	// into the effective program. Unknown literal words break the
	// chain so `echo grep` does not get flagged.
	literalChain := true
	for i, w := range call.Args {
		if wordMentionsGrep(w) {
			return true
		}
		if !literalChain {
			continue
		}
		name, ok := literalProgramName(w)
		if !ok {
			literalChain = false
			continue
		}
		base := filepath.Base(name)
		if grepProgramNames[base] {
			return true
		}
		// Only continue scanning past flags or known wrappers.
		isFlag := strings.HasPrefix(name, "-")
		isWrapper := i == 0 && grepWrapperPrograms[base]
		if !isFlag && !isWrapper {
			literalChain = false
		}
	}
	return false
}

// literalProgramName extracts the literal value of a command-name word.
// Returns ("", false) if the word contains expansions or substitutions
// that we cannot statically resolve.
func literalProgramName(word *syntax.Word) (string, bool) {
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

// wordMentionsGrep returns true when a word contains a command or
// process substitution that itself invokes grep. Plain literals and
// parameter expansions are ignored.
func wordMentionsGrep(word *syntax.Word) bool {
	if word == nil {
		return false
	}
	return partsMentionGrep(word.Parts)
}

func partsMentionGrep(parts []syntax.WordPart) bool {
	for _, part := range parts {
		switch p := part.(type) {
		case *syntax.CmdSubst:
			if stmtsMentionGrep(p.Stmts) {
				return true
			}
		case *syntax.ProcSubst:
			if stmtsMentionGrep(p.Stmts) {
				return true
			}
		case *syntax.DblQuoted:
			if partsMentionGrep(p.Parts) {
				return true
			}
		}
	}
	return false
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
