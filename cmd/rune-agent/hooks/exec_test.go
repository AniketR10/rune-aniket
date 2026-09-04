// Copyright (C) 2017-2026 Unstable Build, LLC
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

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunCommand_ExitZeroPlainText(t *testing.T) {
	t.Parallel()
	r := runnerWithExec()
	res := r.runCommand(context.Background(),
		Hook{Type: HookTypeCommand, Command: `printf "hello world"`, Timeout: 2 * time.Second},
		nil,
		Payload{HookEventName: EventUserPromptSubmit},
		[]byte(`{}`),
	)
	if res.warn != nil {
		t.Fatalf("unexpected warn: %v", res.warn)
	}
	if res.output.HookSpecificOutput == nil {
		t.Fatalf("expected HookSpecificOutput, got nil")
	}
	if got := res.output.HookSpecificOutput.AdditionalContext; got != "hello world" {
		t.Fatalf("AdditionalContext = %q, want %q", got, "hello world")
	}
}

func TestRunCommand_ExitZeroJSON(t *testing.T) {
	t.Parallel()
	r := runnerWithExec()
	cmd := `printf '{"decision":"block","reason":"nope"}'`
	res := r.runCommand(context.Background(),
		Hook{Type: HookTypeCommand, Command: cmd, Timeout: 2 * time.Second},
		nil,
		Payload{HookEventName: EventPostToolUse},
		[]byte(`{}`),
	)
	if res.warn != nil {
		t.Fatalf("unexpected warn: %v", res.warn)
	}
	if res.output.Decision != "block" || res.output.Reason != "nope" {
		t.Fatalf("output = %+v, want decision=block reason=nope", res.output)
	}
}

func TestRunCommand_ExitTwoBlocks(t *testing.T) {
	t.Parallel()
	r := runnerWithExec()
	res := r.runCommand(context.Background(),
		Hook{Type: HookTypeCommand, Command: `printf "blocked!" 1>&2; exit 2`, Timeout: 2 * time.Second},
		nil,
		Payload{HookEventName: EventPostToolUse},
		[]byte(`{}`),
	)
	if res.warn != nil {
		t.Fatalf("unexpected warn: %v", res.warn)
	}
	if res.output.Decision != "block" || res.output.Reason != "blocked!" {
		t.Fatalf("output = %+v, want decision=block reason=blocked!", res.output)
	}
}

func TestRunCommand_OtherExitWarns(t *testing.T) {
	t.Parallel()
	r := runnerWithExec()
	res := r.runCommand(context.Background(),
		Hook{Type: HookTypeCommand, Command: `exit 7`, Timeout: 2 * time.Second},
		nil,
		Payload{HookEventName: EventPostToolUse},
		[]byte(`{}`),
	)
	if res.warn == nil {
		t.Fatalf("expected warn for non-0/2 exit")
	}
	if res.output.Decision == "block" {
		t.Fatalf("non-0/2 exit must not block")
	}
}

func TestRunCommand_Timeout(t *testing.T) {
	t.Parallel()
	r := runnerWithExec()
	res := r.runCommand(context.Background(),
		Hook{Type: HookTypeCommand, Command: `sleep 5`, Timeout: 50 * time.Millisecond},
		nil,
		Payload{HookEventName: EventStop},
		[]byte(`{}`),
	)
	if res.warn == nil || !strings.Contains(res.warn.Error(), "timed out") {
		t.Fatalf("expected timeout warn, got %v", res.warn)
	}
}

func TestRunCommand_EnvVars(t *testing.T) {
	t.Parallel()
	r := runnerWithExec()
	res := r.runCommand(context.Background(),
		Hook{Type: HookTypeCommand, Command: `printf "%s|%s" "$RUNE_HOOK_EVENT" "$RUNE_DIALOGUE_ID"`, Timeout: 2 * time.Second},
		[]string{"RUNE_HOOK_EVENT=PostToolUse", "RUNE_DIALOGUE_ID=abc"},
		Payload{HookEventName: EventUserPromptSubmit},
		[]byte(`{}`),
	)
	if res.warn != nil {
		t.Fatalf("unexpected warn: %v", res.warn)
	}
	got := res.output.HookSpecificOutput.AdditionalContext
	if got != "PostToolUse|abc" {
		t.Fatalf("env = %q, want PostToolUse|abc", got)
	}
}

// --- helpers ---

// runnerWithExec returns a Runner whose runCommand path executes
// through localExec (real shell).
func runnerWithExec() *Runner {
	return &Runner{executor: localExec{}}
}
