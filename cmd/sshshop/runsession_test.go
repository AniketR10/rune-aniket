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

package main

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
)

// fakeScreen mirrors the SDK's RunScreen tests. We intentionally avoid
// tcell.SimulationScreen because its PostEvent implementation recurses.
type fakeScreen struct {
	mu     sync.Mutex
	width  int
	height int
	evch   chan term.Event
}

func newFakeScreen(w, h int) *fakeScreen {
	return &fakeScreen{width: w, height: h, evch: make(chan term.Event, 16)}
}

func (s *fakeScreen) SetContent(int, int, rune, []rune, uint8, term.Style) {}
func (s *fakeScreen) UnionStyle(int, int, term.Style)                      {}
func (s *fakeScreen) Fill(rune, term.Style)                                {}
func (s *fakeScreen) ShowCursor(int, int)                                  {}
func (s *fakeScreen) HideCursor()                                          {}
func (s *fakeScreen) SetCursorStyle(term.CursorStyle)                      {}
func (s *fakeScreen) Size() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.width, s.height
}
func (s *fakeScreen) Show()                   {}
func (s *fakeScreen) Poll() <-chan term.Event { return s.evch }
func (s *fakeScreen) PostEvent(ev term.Event) error {
	select {
	case s.evch <- ev:
		return nil
	default:
		return term.ErrEventQFull
	}
}
func (s *fakeScreen) Bell() {}

type drawCounterHandler struct{ draws atomic.Int32 }

func (h *drawCounterHandler) Resize(int, int)  {}
func (h *drawCounterHandler) Draw(term.Writer) { h.draws.Add(1) }
func (h *drawCounterHandler) Handle(ev term.Event) (bool, bool) {
	return ev.Type == term.EventKey && ev.Key == term.KeyEsc, true
}
func (h *drawCounterHandler) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return term.Coordinates{}, term.CursorStyleDefault, false
}
func (h *drawCounterHandler) Selection() (string, bool) { return "", false }

// TestWriterInterrupterPublishesMultipleInterrupts verifies that the
// per-session writerInterrupter can trigger more than one redraw when
// used against tui.RunWriter with that SAME writer instance. This is
// the contract sshshop relies on for shaders and any other background
// animations: multiple interrupt ticks must schedule multiple redraws
// without waiting for any keypress.
func TestWriterInterrupterPublishesMultipleInterrupts(t *testing.T) {
	s := newFakeScreen(20, 8)

	h := &drawCounterHandler{}
	w := term.NewScreenWriter(s)
	done := make(chan error, 1)
	go func() { done <- tui.RunWriter(h, w) }()

	interrupter := writerInterrupter{w: w}

	require.Eventually(t, func() bool {
		return h.draws.Load() >= 1
	}, time.Second, 20*time.Millisecond)
	base := h.draws.Load()

	require.NoError(t, interrupter.Interrupt(context.Background()))
	require.Eventually(t, func() bool {
		return h.draws.Load() > base
	}, time.Second, 20*time.Millisecond)
	afterFirst := h.draws.Load()

	require.NoError(t, interrupter.Interrupt(context.Background()))
	require.Eventually(t, func() bool {
		return h.draws.Load() > afterFirst
	}, time.Second, 20*time.Millisecond,
		"second interrupt should schedule another redraw without any keypress")

	require.True(t, w.PublishEvent(term.Event{Type: term.EventKey, Key: term.KeyEsc}))
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("RunScreen did not exit after Esc")
	}
}
