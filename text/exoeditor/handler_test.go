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

package exoeditor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/term/vte/vteprobe"
	"unstable.build/go-tui/text"
)

func TestPickLocation(t *testing.T) {
	t.Parallel()

	locs := []textapi.Location{
		{From: term.Coordinates{X: 0, Y: 1}},
		{From: term.Coordinates{X: 4, Y: 3}},
		{From: term.Coordinates{X: 2, Y: 5}},
	}

	cases := []struct {
		name    string
		cursor  term.Coordinates
		forward bool
		want    term.Coordinates
	}{
		{
			name:    "next from start jumps to first",
			cursor:  term.Coordinates{X: 0, Y: 0},
			forward: true,
			want:    locs[0].From,
		},
		{
			name:    "next from middle jumps to next",
			cursor:  term.Coordinates{X: 0, Y: 3},
			forward: true,
			want:    locs[1].From,
		},
		{
			name:    "next past last wraps to first",
			cursor:  term.Coordinates{X: 9, Y: 9},
			forward: true,
			want:    locs[0].From,
		},
		{
			name:    "prev from end jumps to last",
			cursor:  term.Coordinates{X: 9, Y: 9},
			forward: false,
			want:    locs[2].From,
		},
		{
			name:    "prev from middle jumps to prev",
			cursor:  term.Coordinates{X: 9, Y: 3},
			forward: false,
			want:    locs[1].From,
		},
		{
			name:    "prev before first wraps to last",
			cursor:  term.Coordinates{X: 0, Y: 0},
			forward: false,
			want:    locs[2].From,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := pickLocation(
				text.LocationSlice(locs), tc.cursor, tc.forward)
			assert.True(t, ok)
			assert.Equal(t, tc.want, got.From)
		})
	}
}

func TestPickLocationEmptyListReturnsFalse(t *testing.T) {
	t.Parallel()
	_, ok := pickLocation(
		text.LocationSlice(nil), term.Coordinates{}, true)
	assert.False(t, ok)
}

// TestMoveToLocationReturnsFalseForUnknownList guards the public
// MoveTo{Next,Prev}Location contract for IDs that were never set via
// SetLocationList.
func TestMoveToLocationReturnsFalseForUnknownList(t *testing.T) {
	t.Parallel()
	h := &editorHandler{locations: text.NewLocationStore()}
	assert.False(t, h.MoveToNextLocation("missing"))
	assert.False(t, h.MoveToPrevLocation("missing"))
}

// stubVTEHandler records the events the readiness gate injects so tests
// can assert on the keystrokes forwarded to the embedded editor without
// a live vte. Every other vteHandler method is unused by these tests and
// panics if exercised, surfacing accidental dependencies.
type stubVTEHandler struct {
	events []term.Event
}

func (s *stubVTEHandler) Handle(ev term.Event) (bool, bool) {
	s.events = append(s.events, ev)
	return false, true
}

func (s *stubVTEHandler) Resize(int, int)                                    { panic("unused") }
func (s *stubVTEHandler) Cursor() (term.Coordinates, term.CursorStyle, bool) { panic("unused") }
func (s *stubVTEHandler) Selection() (string, bool)                          { panic("unused") }
func (s *stubVTEHandler) Draw(term.Writer)                                   { panic("unused") }
func (s *stubVTEHandler) Close() error                                       { panic("unused") }
func (s *stubVTEHandler) MaxSeekOffset() int                                 { panic("unused") }
func (s *stubVTEHandler) SeekOffset() int                                    { panic("unused") }
func (s *stubVTEHandler) SeekUp() bool                                       { panic("unused") }
func (s *stubVTEHandler) SeekDown() bool                                     { panic("unused") }

// gotoReadinessHarness builds a minimal editorHandler with a real goto
// template, a recording vteHandler stub, and a recording scheduleNextTick
// so the readiness gate can be exercised without a live vte.
type gotoReadinessHarness struct {
	h         *editorHandler
	vte       *stubVTEHandler
	scheduled []func()
}

func newGotoReadinessHarness(t *testing.T, tpl string) *gotoReadinessHarness {
	t.Helper()
	parsed, err := parseGotoTemplate(tpl)
	require.NoError(t, err)

	hr := &gotoReadinessHarness{}
	hr.vte = &stubVTEHandler{}
	h := &editorHandler{gotoTemplate: parsed, vteHandler: hr.vte}
	h.scheduleNextTick = func(fn func()) bool {
		hr.scheduled = append(hr.scheduled, fn)
		return true
	}
	hr.h = h
	return hr
}

// wantEvents renders tpl for the 1-based coords of pos and converts the
// keys into the events injectGoto forwards, for exact assertion.
func wantEvents(g gotoTemplate, pos term.Coordinates) []term.Event {
	keys := g.Render(pos.Y+1, pos.X+1)
	out := make([]term.Event, 0, len(keys))
	for _, k := range keys {
		out = append(out, keyCombToEvent(k))
	}
	return out
}

// markReady simulates refreshProbe's first successful probe: it stores a
// probe result and flushes any pending goto via scheduleNextTick.
func (hr *gotoReadinessHarness) markReady() {
	h := hr.h
	h.probeStateMu.Lock()
	firstProbe := h.lastProbe.Load() == nil
	h.lastProbe.Store(&vteprobe.Result{})
	var flush *term.Coordinates
	if firstProbe {
		flush = h.pendingGoto
		h.pendingGoto = nil
	}
	h.probeStateMu.Unlock()

	if flush != nil {
		pos := *flush
		h.scheduleNextTick(func() { h.injectGoto(pos) })
	}
}

func (hr *gotoReadinessHarness) runScheduled() {
	for _, fn := range hr.scheduled {
		fn()
	}
}

const readinessTpl = "<esc>:{line}<enter>"

func TestSetCursorAtScrollGatedUntilReady(t *testing.T) {
	hr := newGotoReadinessHarness(t, readinessTpl)
	pos := term.Coordinates{X: 4, Y: 9}

	require.True(t, hr.h.SetCursorAtScroll(pos))
	assert.Empty(t, hr.vte.events, "keys must not be injected before the editor is ready")
	assert.Empty(t, hr.scheduled, "nothing scheduled before first probe")
	require.NotNil(t, hr.h.pendingGoto)
	assert.Equal(t, pos, *hr.h.pendingGoto)

	hr.markReady()
	require.Len(t, hr.scheduled, 1, "first probe flushes the queued goto exactly once")
	assert.Empty(t, hr.vte.events, "flush is deferred to the scheduled tick")
	assert.Nil(t, hr.h.pendingGoto)

	hr.runScheduled()
	assert.Equal(t, wantEvents(hr.h.gotoTemplate, pos), hr.vte.events)
}

func TestSetCursorAtScrollReadyInjectsImmediately(t *testing.T) {
	hr := newGotoReadinessHarness(t, readinessTpl)
	hr.markReady()
	require.Empty(t, hr.scheduled, "no pending goto means nothing is scheduled")

	pos := term.Coordinates{X: 2, Y: 5}
	require.True(t, hr.h.SetCursorAtScroll(pos))
	assert.Empty(t, hr.scheduled, "ready path injects synchronously, no tick")
	assert.Equal(t, wantEvents(hr.h.gotoTemplate, pos), hr.vte.events)
}

func TestSecondProbeDoesNotReflush(t *testing.T) {
	hr := newGotoReadinessHarness(t, readinessTpl)
	require.True(t, hr.h.SetCursorAtScroll(term.Coordinates{X: 1, Y: 1}))

	hr.markReady()
	require.Len(t, hr.scheduled, 1)

	hr.markReady()
	assert.Len(t, hr.scheduled, 1, "a subsequent probe must not re-schedule the goto")
}

func TestSetCursorAtScrollEmptyTemplate(t *testing.T) {
	hr := newGotoReadinessHarness(t, "")
	assert.False(t, hr.h.SetCursorAtScroll(term.Coordinates{X: 1, Y: 1}))
	assert.Empty(t, hr.vte.events)
	assert.Empty(t, hr.scheduled)
	assert.Nil(t, hr.h.pendingGoto)
}
