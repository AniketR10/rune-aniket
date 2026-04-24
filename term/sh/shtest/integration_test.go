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


package shtest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/handlertest"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/term/sh"
)

type testHandler struct {
	handleFn func(
		context.Context, repl.Command, repl.ProgressWriter,
	) (iterator.Iterator[component.Responsive], error)
	completeFn func(
		context.Context, string, []string,
	) (iterator.Iterator[string], error)
}

func (t *testHandler) HandleCommand(
	ctx context.Context, cmd repl.Command, pw repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	if t.handleFn != nil {
		return t.handleFn(ctx, cmd, pw)
	}
	return iterator.FromSlice[component.Responsive](nil), nil
}

func (t *testHandler) Complete(
	ctx context.Context, cmd string, args []string,
) (iterator.Iterator[string], error) {
	if t.completeFn != nil {
		return t.completeFn(ctx, cmd, args)
	}
	return iterator.FromSlice[string](nil), nil
}

// Note: The REPL handler dispatches commands
// asynchronously via scheduleNextTick, which requires
// the tui.Run event loop. In tests without the event
// loop, command output callbacks are dropped. Expected
// strings verify echoed command lines (added
// synchronously) and prompt state.

func nopSchedule(func()) bool          { return false }
func nopInterrupter() term.Interrupter { return term.NopInterrupter() }

func TestIntegration(t *testing.T) {
	cases := []struct {
		name  string
		cases []handlertest.SequenceTestCase
	}{
		{
			name: "empty prompt",
			cases: []handlertest.SequenceTestCase{
				{
					InputSequence: "",
					Expected: "                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"$ ▐                 ",
				},
			},
		},
		{
			name: "simple command echo",
			cases: []handlertest.SequenceTestCase{
				{
					InputSequence: "echo<space>hello<enter>",
					Expected: "                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"$ echo hello        \n" +
						"$ ▐                 ",
				},
			},
		},
		{
			name: "pipe syntax accepted",
			cases: []handlertest.SequenceTestCase{
				{
					InputSequence: "a|b<enter>",
					Expected: "                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"$ a|b               \n" +
						"$ ▐                 ",
				},
			},
		},
		{
			name: "semicolons accepted",
			cases: []handlertest.SequenceTestCase{
				{
					InputSequence: "a;b<enter>",
					Expected: "                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"$ a;b               \n" +
						"$ ▐                 ",
				},
			},
		},
		{
			name: "variable syntax",
			cases: []handlertest.SequenceTestCase{
				{
					InputSequence: "FOO=x<enter>",
					Expected: "                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"$ FOO=x             \n" +
						"$ ▐                 ",
				},
			},
		},
		{
			name: "logical operators",
			cases: []handlertest.SequenceTestCase{
				{
					InputSequence: "a&&b<enter>",
					Expected: "                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"$ a&&b              \n" +
						"$ ▐                 ",
				},
			},
		},
		{
			name: "subshell syntax",
			cases: []handlertest.SequenceTestCase{
				{
					InputSequence: "(echo<space>hi)<enter>",
					Expected: "                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"$ (echo hi)         \n" +
						"$ ▐                 ",
				},
			},
		},
		{
			name: "multiple commands",
			cases: []handlertest.SequenceTestCase{
				{
					InputSequence: "cmd1<enter>",
					Expected: "                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"$ cmd1              \n" +
						"$ ▐                 ",
				},
				{
					InputSequence: "cmd2<enter>",
					Expected: "                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"$ cmd1              \n" +
						"$ cmd2              \n" +
						"$ ▐                 ",
				},
			},
		},
		{
			name: "ctrl+C clears",
			cases: []handlertest.SequenceTestCase{
				{
					InputSequence: "hello<c-c>",
					Expected: "                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"^C                  \n" +
						"$ ▐                 ",
				},
			},
		},
		{
			name: "empty input echoes blank prompt",
			cases: []handlertest.SequenceTestCase{
				{
					InputSequence: "<space><space><space><enter>",
					Expected: "                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"$                   \n" +
						"$ ▐                 ",
				},
			},
		},
		{
			name: "overflow scroll",
			cases: []handlertest.SequenceTestCase{
				{
					InputSequence: "c1<enter>c2<enter>c3<enter>c4<enter>" +
						"c5<enter>c6<enter>c7<enter>c8<enter>c9<enter>",
					Expected: "$ c1                \n" +
						"$ c2                \n" +
						"$ c3                \n" +
						"$ c4                \n" +
						"$ c5                \n" +
						"$ c6                \n" +
						"$ c7                \n" +
						"$ c8                \n" +
						"$ c9                \n" +
						"$ ▐                 ",
				},
			},
		},
		{
			name: "syntax error non-crash",
			cases: []handlertest.SequenceTestCase{
				{
					InputSequence: "echo<space>\"unterminated<enter>",
					Expected: "                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"                    \n" +
						"$ echo \"unterminated\n" +
						"$ ▐                 ",
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			underlying := &testHandler{}
			h := repl.New(
				sh.New(underlying),
				nopSchedule, nopInterrupter(),
				repl.WithPrompt("$ "),
			)
			handlertest.RunHandlerSequence(
				t, h, 20, 10, tc.cases,
			)
		})
	}
}

func TestCtrlDEOF(t *testing.T) {
	underlying := &testHandler{}
	h := repl.New(
		sh.New(underlying),
		nopSchedule, nopInterrupter(),
		repl.WithPrompt("$ "),
	)
	h.Resize(20, 10)
	exit, handled := h.Handle(term.Event{
		Type: term.EventKey,
		Ch:   'd',
		Mod:  term.ModCtrl,
	})
	assert.True(t, exit)
	assert.True(t, handled)
}

func TestTabCompletionPassthrough(t *testing.T) {
	called := false
	underlying := &testHandler{
		completeFn: func(
			_ context.Context,
			_ string, _ []string,
		) (iterator.Iterator[string], error) {
			called = true
			return iterator.FromSlice([]string{
				"foobar", "foobaz",
			}), nil
		},
	}
	h := repl.New(
		sh.New(underlying),
		nopSchedule, nopInterrupter(),
		repl.WithPrompt("$ "),
	)
	h.Resize(30, 10)

	keys, err := term.ParseKeys("foo<tab>")
	require.NoError(t, err)
	for _, k := range keys {
		h.Handle(term.Event{
			Type: term.EventKey,
			Ch:   k.Ch,
			Mod:  k.Mod,
			Key:  k.Key,
		})
	}
	assert.True(t, called)
}

func TestHistoryRecall(t *testing.T) {
	underlying := &testHandler{}
	h := repl.New(
		sh.New(underlying),
		nopSchedule, nopInterrupter(),
		repl.WithPrompt("$ "),
	)
	h.Resize(30, 10)

	keys, err := term.ParseKeys("first<enter>second<enter><up>")
	require.NoError(t, err)
	for _, k := range keys {
		h.Handle(term.Event{
			Type: term.EventKey,
			Ch:   k.Ch,
			Mod:  k.Mod,
			Key:  k.Key,
		})
	}

	got := handlertest.DrawHandler(h, 30, 10)
	assert.Contains(t, got, "second")
}

func TestCustomPrompt(t *testing.T) {
	underlying := &testHandler{}
	h := repl.New(
		sh.New(underlying),
		nopSchedule, nopInterrupter(),
		repl.WithPrompt(">> "),
	)
	cases := []handlertest.SequenceTestCase{
		{
			InputSequence: "",
			Expected: "                    \n" +
				"                    \n" +
				"                    \n" +
				"                    \n" +
				"                    \n" +
				"                    \n" +
				"                    \n" +
				"                    \n" +
				"                    \n" +
				">> ▐                ",
		},
	}
	handlertest.RunHandlerSequence(t, h, 20, 10, cases)
}
