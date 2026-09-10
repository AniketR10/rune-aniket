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

package hooks

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
)

func TestRunner_NoMatch(t *testing.T) {
	t.Parallel()
	r := runnerWithGroups(EventPostToolUse, []Group{
		{Matcher: "edit_file", Hooks: []Hook{{Type: HookTypeCommand, Command: "exit 0", Timeout: time.Second}}},
	})
	res := r.Run(context.Background(), Payload{HookEventName: EventPostToolUse, ToolName: "read_file"})
	if !res.Continue || res.Blocked() {
		t.Fatalf("non-matching matcher should yield zero result, got %+v", res)
	}
}

func TestRunner_BlockPrecedence(t *testing.T) {
	t.Parallel()
	// Two hooks: first emits no decision, second blocks. Last
	// reason wins.
	r := runnerWithGroups(EventPostToolUse, []Group{
		{Matcher: "*", Hooks: []Hook{
			{Type: HookTypeCommand, Command: `printf '{"decision":"approve"}'`, Timeout: time.Second},
			{Type: HookTypeCommand, Command: `printf "second" 1>&2; exit 2`, Timeout: time.Second},
		}},
	})
	res := r.Run(context.Background(), Payload{HookEventName: EventPostToolUse, ToolName: "x"})
	if !res.Blocked() {
		t.Fatalf("expected block, got %+v", res)
	}
	if res.Reason != "second" {
		t.Fatalf("Reason = %q, want second", res.Reason)
	}
}

func TestRunner_AdditionalContextConcat(t *testing.T) {
	t.Parallel()
	r := runnerWithGroups(EventUserPromptSubmit, []Group{
		{Matcher: "*", Hooks: []Hook{
			{Type: HookTypeCommand, Command: `printf "first"`, Timeout: time.Second},
			{Type: HookTypeCommand, Command: `printf "second"`, Timeout: time.Second},
		}},
	})
	res := r.Run(context.Background(), Payload{HookEventName: EventUserPromptSubmit, Prompt: "hello"})
	parts := strings.Split(res.AdditionalContext, "\n")
	if len(parts) != 2 {
		t.Fatalf("AdditionalContext = %q, want 2 parts joined by newline", res.AdditionalContext)
	}
	want := map[string]bool{"first": true, "second": true}
	for _, p := range parts {
		if !want[p] {
			t.Fatalf("unexpected part %q", p)
		}
	}
}

func TestRunner_AdditionalContextTruncated(t *testing.T) {
	t.Parallel()
	// Build a hook that emits 12k characters; must be truncated to 10k.
	r := runnerWithGroups(EventUserPromptSubmit, []Group{
		{Matcher: "*", Hooks: []Hook{
			{Type: HookTypeCommand, Command: `head -c 12000 < /dev/zero | tr '\0' x`, Timeout: 2 * time.Second},
		}},
	})
	res := r.Run(context.Background(), Payload{HookEventName: EventUserPromptSubmit})
	if got := len(res.AdditionalContext); got != maxOutputBytes {
		t.Fatalf("len(AdditionalContext) = %d, want %d", got, maxOutputBytes)
	}
}

func TestRunner_ParallelDispatch(t *testing.T) {
	t.Parallel()
	// Three concurrent hooks each sleeping 200ms — total runtime must
	// be well under 600ms (sequential lower bound).
	r := runnerWithGroups(EventStop, []Group{
		{Matcher: "*", Hooks: []Hook{
			{Type: HookTypeCommand, Command: `sleep 0.2`, Timeout: 5 * time.Second},
			{Type: HookTypeCommand, Command: `sleep 0.2`, Timeout: 5 * time.Second},
			{Type: HookTypeCommand, Command: `sleep 0.2`, Timeout: 5 * time.Second},
		}},
	})
	start := time.Now()
	r.Run(context.Background(), Payload{HookEventName: EventStop})
	elapsed := time.Since(start)
	if elapsed > 500*time.Millisecond {
		t.Fatalf("parallel dispatch slower than expected: %s", elapsed)
	}
}

func TestRunner_ContinueFalseRecorded(t *testing.T) {
	t.Parallel()
	r := runnerWithGroups(EventStop, []Group{
		{Matcher: "*", Hooks: []Hook{
			{Type: HookTypeCommand, Command: `printf '{"continue":false,"stopReason":"halt"}'`, Timeout: time.Second},
		}},
	})
	res := r.Run(context.Background(), Payload{HookEventName: EventStop})
	if res.Continue {
		t.Fatalf("Continue=true, want false")
	}
	if res.StopReason != "halt" {
		t.Fatalf("StopReason = %q, want halt", res.StopReason)
	}
}

func TestRunner_NotifierWarnings(t *testing.T) {
	t.Parallel()
	var warns int32
	noti := notifierFn(func() { atomic.AddInt32(&warns, 1) })
	cfg := Config{Stop: []Group{{Matcher: "*", Hooks: []Hook{
		{Type: HookTypeCommand, Command: `exit 7`, Timeout: time.Second},
	}}}}
	r := NewRunner(cfg, localExec{}, noti, "")
	r.Run(context.Background(), Payload{HookEventName: EventStop})
	if atomic.LoadInt32(&warns) != 1 {
		t.Fatalf("warns = %d, want 1", warns)
	}
}

// --- helpers ---

// runnerWithGroups builds a Runner whose groups are wired to the
// given event for testing.
func runnerWithGroups(ev Event, groups []Group) *Runner {
	cfg := Config{}
	switch ev {
	case EventPostToolUse:
		cfg.PostToolUse = groups
	case EventStop:
		cfg.Stop = groups
	case EventUserPromptSubmit:
		cfg.UserPromptSubmit = groups
	case EventNotification:
		cfg.Notification = groups
	}
	return NewRunner(cfg, localExec{}, nil, "")
}

// notifierFn implements Notifier with a no-op stub that increments a
// counter on every Notify call.
type notifierFn func()

func (n notifierFn) Notify(_ browserapi.NotificationLevel, _ string, _ ...any) (string, error) {
	n()
	return "", nil
}
