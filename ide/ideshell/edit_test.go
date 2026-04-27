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

package ideshell

import (
	"testing"

	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/handlertest"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/handler/command"
)

// stubEditor is a minimal command.Editor for tests. It records every
// event delivered to its EditHandler and lets the test invoke an
// optional handle function to mutate the buffer (e.g. append a rune)
// the way a real editor would.
type stubEditor struct {
	seen *[]term.Event
	// initialCursor is recorded by SetCursorAtScroll so tests can
	// assert the host seeded the cursor correctly.
	initialCursor *term.Coordinates
}

func (s stubEditor) Edit(buf *cell.Buffer) command.EditHandler {
	return &stubEditHandler{
		buf:           buf,
		seen:          s.seen,
		initialCursor: s.initialCursor,
	}
}

type stubEditHandler struct {
	buf           *cell.Buffer
	seen          *[]term.Event
	initialCursor *term.Coordinates
}

func (s *stubEditHandler) Resize(int, int)  {}
func (s *stubEditHandler) Draw(term.Writer) {}
func (s *stubEditHandler) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return term.Coordinates{X: s.buf.Columns(0)},
		term.CursorStyleSteadyBar, true
}
func (s *stubEditHandler) SetCursorAtScroll(pos term.Coordinates) bool {
	if s.initialCursor != nil {
		*s.initialCursor = pos
	}
	return true
}
func (s *stubEditHandler) Selection() (string, bool) { return "", false }
func (s *stubEditHandler) Handle(ev term.Event) (bool, bool) {
	if s.seen != nil {
		*s.seen = append(*s.seen, ev)
	}
	if ev.Type != term.EventKey {
		return false, true
	}
	switch ev.Key {
	case term.KeyBackspace:
		cols := s.buf.Columns(0)
		if cols > 0 {
			s.buf.DeleteCell(term.Coordinates{X: cols - 1})
		}
		return false, true
	case term.KeySpace:
		s.buf.WriteString(" ")
		return false, true
	}
	if ev.Ch != 0 {
		s.buf.WriteString(string(ev.Ch))
	}
	return false, true
}

var _ command.EditHandler = (*stubEditHandler)(nil)

// editModeKey mirrors what the IDE production code uses for the
// command Prompt's modal-edit toggle.
var editModeKey = term.KeyComb{Mod: term.ModShift, Key: term.KeyEsc}

func newEditTestHandler(t *testing.T, editor command.Editor) *Handler {
	t.Helper()
	h, _ := New(
		func(func()) bool { return false },
		term.NopInterrupter(),
		Config{
			MaxHistory:  100,
			EditModeKey: editModeKey,
			Editor:      editor,
		},
	)
	t.Cleanup(func() { _ = h.Close() })
	return h
}

func TestEditModeEnterExitReplacesInputText(t *testing.T) {
	var seen []term.Event
	var seeded term.Coordinates
	h := newEditTestHandler(t, stubEditor{
		seen: &seen, initialCursor: &seeded,
	})

	// Type "hello", then enter edit mode, append " world", exit.
	handlertest.RunHandlerSequence(t, h, testWidthH, testHeight,
		[]handlertest.SequenceTestCase{{
			InputSequence: "hello<s-esc><space>world<s-esc>",
			Expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> hello world▐                `,
		}})

	// editor saw the edit-mode events: 6 typed runes (space + 5
	// letters of "world") appended to the buffer it owned.
	assert.Equal(t, 6, len(seen))
	// cursor was seeded at end of "hello" (5 columns).
	assert.Equal(t, 5, seeded.X)
}

func TestEditModeSubmitDispatches(t *testing.T) {
	var dispatched []string
	reg := func(r *CommandRegistry) {
		r.Register("e", "echo", echoCmd{out: &dispatched})
	}
	h, registry := New(
		func(func()) bool { return false },
		term.NopInterrupter(),
		Config{
			MaxHistory:  100,
			EditModeKey: editModeKey,
			Editor:      stubEditor{},
		},
	)
	t.Cleanup(func() { _ = h.Close() })
	reg(registry)

	h.Resize(testWidthH, testHeight)
	feed := func(seq string) {
		for _, c := range seq {
			h.Handle(term.Event{Type: term.EventKey, Ch: c})
		}
	}
	h.Handle(term.Event{
		Type: term.EventKey,
		Mod:  term.ModShift, Key: term.KeyEsc,
	})
	feed("e")
	h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	h.inner.Wait()

	require.Equal(t, []string{"e"}, dispatched)
}

func TestEditModeCtrlCExits(t *testing.T) {
	var seen []term.Event
	h := newEditTestHandler(t, stubEditor{seen: &seen})

	handlertest.RunHandlerSequence(t, h, testWidthH, testHeight,
		[]handlertest.SequenceTestCase{{
			InputSequence: "<s-esc>X<c-c>",
			Expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> X▐                          `,
		}})

	// editor saw only "X" — ctrl-c was intercepted upstream.
	require.Len(t, seen, 1)
	assert.Equal(t, 'X', seen[0].Ch)
}

func TestEditModeTabExits(t *testing.T) {
	var seen []term.Event
	h := newEditTestHandler(t, stubEditor{seen: &seen})

	handlertest.RunHandlerSequence(t, h, testWidthH, testHeight,
		[]handlertest.SequenceTestCase{{
			InputSequence: "<s-esc>Y<tab>",
			Expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> Y▐                          `,
		}})

	require.Len(t, seen, 1)
	assert.Equal(t, 'Y', seen[0].Ch)
}

func TestEditModePanicsWithoutEditor(t *testing.T) {
	assert.Panics(t, func() {
		_, _ = New(
			func(func()) bool { return false },
			term.NopInterrupter(),
			Config{
				EditModeKey: editModeKey,
				// Editor intentionally nil.
			},
		)
	})
}

func TestEditModeDisabledWhenKeyNotSet(t *testing.T) {
	// Editor is nil and EditModeKey is zero — must not panic and
	// shift-esc should pass through normally (be a no-op).
	h, _ := New(
		func(func()) bool { return false },
		term.NopInterrupter(),
		Config{},
	)
	t.Cleanup(func() { _ = h.Close() })

	handlertest.RunHandlerSequence(t, h, testWidthH, testHeight,
		[]handlertest.SequenceTestCase{{
			InputSequence: "<s-esc>z",
			Expected: `                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
                              
> z▐                          `,
		}})
}

// echoCmd is a CommandHandler that records its dispatched name into
// a caller-supplied slice. It is used to verify that edit-mode
// submit forwards the original <enter> through to the inner repl.
type echoCmd struct{ out *[]string }

func (e echoCmd) HandleCommand(
	_ context.Context, cmd repl.Command, _ repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	*e.out = append(*e.out, cmd.Name)
	return iterator.Empty[component.Responsive](), nil
}

func (e echoCmd) Complete(
	context.Context, string, []string,
) (iterator.Iterator[string], error) {
	return iterator.Empty[string](), nil
}

func (e echoCmd) Help(
	context.Context, []string,
) (iterator.Iterator[component.Responsive], error) {
	return iterator.Empty[component.Responsive](), nil
}
