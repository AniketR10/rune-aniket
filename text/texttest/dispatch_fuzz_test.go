// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.

package texttest

import (
	"context"
	"strings"
	"testing"

	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/text/cmdenv"
)

// FuzzDispatchCommandAlias fuzzes text.Component.DispatchCommand
// alias expansion against arbitrary alias bodies. The contract: an
// arbitrary string registered as an alias body must never panic the
// dispatcher. Successful dispatch is allowed but not required —
// malformed bodies are expected to surface as an error from either
// NewComponent (cycle / parse) or DispatchCommand (expansion).
//
// The fuzzer holds dispatched args fixed so the corpus space stays
// focused on what mvdan.cc/sh and our alias splitter see in
// production. FuzzDispatchCommandDirectArgs covers the symmetric
// path for direct (non-alias) argv.
func FuzzDispatchCommandAlias(f *testing.F) {
	seeds := []string{
		"echo $1",
		"echo ${1}",
		"echo $1 $2",
		"echo $$HOME",
		`echo "quoted $1 stays one field"`,
		"echo ${MISSING:-default}",
		"echo ${MISSING:?bang}",
		"echo $(echo nope)",
		"echo `echo nope`",
		"echo $FILE $LINE $COLUMN",
		"echo ${1}${2}${3}",
		"echo $$",
		"echo ${",
		"echo $",
		"",
		" ",
		"echo \x00",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	// Pin host env for names mvdan.cc/sh's fallback might otherwise
	// resolve from the developer's shell.
	for _, name := range []string{
		"HOME", "PATH", "SHELL", "USER", "FOO", "BAR",
	} {
		f.Setenv(name, "")
	}

	uri, err := workspaceapi.ParseURI("file:///fuzz")
	if err != nil {
		f.Fatalf("parse uri: %v", err)
	}

	envSource := cmdenv.Source(func(name string) (string, bool) {
		if name == "WORKSPACE_URI" {
			return "file:///fuzz", true
		}
		if name == "WORKSPACE_PATH" {
			return "/fuzz", true
		}
		return "", false
	})

	const aliasName = "fuzzalias"

	f.Fuzz(func(t *testing.T, body string) {
		// Strings with embedded NULs aren't valid YAML/Starlark
		// alias bodies in production; skip to avoid burning the
		// budget on inputs users can't actually create.
		if strings.ContainsRune(body, 0) {
			return
		}
		// An alias body whose first token is the alias name itself
		// would form a self-cycle and NewComponent would reject it
		// at construction time — that's a contract test (covered
		// elsewhere), not a dispatch fuzz case.
		if strings.HasPrefix(strings.TrimSpace(body), aliasName) {
			return
		}

		cfg := text.DefaultConfig()
		cfg.ScheduleNextTick = func(fn func()) bool { fn(); return true }
		cfg.EnvSource = envSource
		cfg.CommandAliases = map[string]text.CommandAlias{
			aliasName: {Commands: []string{body}},
		}

		c, _, nerr := newTestComponentErr(NopEditor(), cfg)
		if nerr != nil {
			// Cycle / parse-time rejection is fine; the contract
			// is no panic.
			return
		}
		win, _ := c.Focus()

		// Register a sink that swallows anything dispatched so a
		// successful expansion doesn't fail "no handler" and to
		// keep the fuzzer's signal focused on the
		// dispatch/expansion machinery itself.
		sink := text.FuncCommandHandler(
			func(context.Context, textapi.Command) error { return nil },
			nil)
		for _, name := range []string{
			"echo", "sink", "newWindow", "edit", "open",
			"git", "gsed", "sed", "cat", "ls",
		} {
			c.SubscribeCommand(testCommand(name, "", ""), sink)
		}

		cmd := textapi.Command{
			Resource: NewTestHandler(),
			URI:      uri,
			Name:     aliasName,
			Args:     []string{"a1", "a2", "a3"},
			Window:   win,
		}
		// Contract: must not panic. Returned ok/err are allowed
		// to take any value.
		_, _ = dispatchWithAliases(context.Background(), c, cmd)
	})
}

// FuzzDispatchCommandDirectArgs fuzzes the dispatched-argv expansion
// path. Each fuzzed input is split on NUL into a multi-arg vector
// (NUL is the one byte that can never appear inside a real argv
// field), so the fuzzer can explore argv shape as well as per-arg
// content from a single string seed.
func FuzzDispatchCommandDirectArgs(f *testing.F) {
	seeds := []string{
		"$FILE",
		"$FILE\x00--all",
		"$1",
		"$$HOME",
		"${MISSING:-fb}",
		"$(echo nope)",
		"`echo nope`",
		"hello $WORD world",
		"\x00",
		"",
		"plain",
		"${UNCLOSED",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	for _, name := range []string{
		"HOME", "PATH", "SHELL", "USER", "FOO", "BAR",
	} {
		f.Setenv(name, "")
	}

	uri, err := workspaceapi.ParseURI("file:///fuzz")
	if err != nil {
		f.Fatalf("parse uri: %v", err)
	}

	f.Fuzz(func(t *testing.T, packed string) {
		args := strings.Split(packed, "\x00")

		cfg := text.DefaultConfig()
		cfg.ScheduleNextTick = func(fn func()) bool { fn(); return true }

		c, _, nerr := newTestComponentErr(NopEditor(), cfg)
		if nerr != nil {
			return
		}
		win, _ := c.Focus()

		c.SubscribeCommand(testCommand("sink", "", ""),
			text.FuncCommandHandler(
				func(context.Context, textapi.Command) error { return nil },
				nil))

		cmd := textapi.Command{
			Resource: NewTestHandler(),
			URI:      uri,
			Name:     "sink",
			Args:     args,
			Window:   win,
		}
		_, _ = dispatchWithAliases(context.Background(), c, cmd)
	})
}
