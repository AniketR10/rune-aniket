// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package vi

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	thandler "unstable.build/go-tui/handler"
	"unstable.build/go-tui/handler/handlertest"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/text/registerset"
	"unstable.build/go-tui/text/texttest"
)

const snippet = `
/*
 * Check if the current buffer should be added to or removed from the list of
 * diff buffers.
 */
	void
diff_buf_adjust(win_T *win)
{
	win_T	*wp;
	int				i;

	if (!win->w_p_diff)
	{
	/* When there is no window showing a diff for this buffer, remove
	 * it from the diffs... */
	FOR_ALL_WINDOWS(wp)
		if (wp->w_buffer == win->w_buffer && wp->w_p_diff)
		break;
	if (wp == NULL)
	{
		i = diff_buf_idx(win->w_buffer);
		if (i != DB_COUNT)
		{
		curtab->tp_diffbuf[i] = NULL;
		curtab->tp_diff_invalid = TRUE;
		diff_redraw(TRUE);
		}
	}
	}
	else
	diff_buf_add(win->w_buffer);
}`

type testSelectionService struct {
	view   cell.View
	expand map[term.Range]term.Range
	shrink map[term.Range]term.Range
}

func (s testSelectionService) Rows() int { return s.view.Rows() }

func (s testSelectionService) Columns(row int) int { return s.view.Columns(row) }

func (s testSelectionService) Cell(at term.Coordinates) (term.Cell, bool) {
	return s.view.Cell(at)
}

func (s testSelectionService) RawCells() [][]term.Cell { return s.view.RawCells() }

func (s testSelectionService) String() string { return s.view.String() }

func (s testSelectionService) SelectionExpand(rng term.Range) (term.Range, bool) {
	next, ok := s.expand[rng]
	return next, ok
}

func (s testSelectionService) SelectionShrink(rng term.Range, caret term.Coordinates) (term.Range, bool) {
	next, ok := s.shrink[rng]
	return next, ok
}

func TestZModeSyntacticSelection(t *testing.T) {
	buf := cell.NewBuffer()
	buf.Init()
	_, err := buf.ReadFrom(strings.NewReader("alpha beta gamma"))
	require.NoError(t, err)

	svc := testSelectionService{
		view: buf.View(),
		expand: map[term.Range]term.Range{
			{Start: term.Coordinates{X: 6}, End: term.Coordinates{X: 6}}:  {Start: term.Coordinates{X: 6}, End: term.Coordinates{X: 10}},
			{Start: term.Coordinates{X: 6}, End: term.Coordinates{X: 10}}: {Start: term.Coordinates{}, End: term.Coordinates{X: 10}},
		},
		shrink: map[term.Range]term.Range{
			{Start: term.Coordinates{}, End: term.Coordinates{X: 10}}:     {Start: term.Coordinates{X: 6}, End: term.Coordinates{X: 10}},
			{Start: term.Coordinates{X: 6}, End: term.Coordinates{X: 10}}: {Start: term.Coordinates{X: 6}, End: term.Coordinates{X: 6}},
		},
	}
	buf.WithView(svc)

	vi := new(viHandlerImpl)
	vi.init(buf, defaultviHandlerImplConfig())
	vi.Resize(80, 10)
	require.True(t, vi.setCursorAtScroll(term.Coordinates{X: 6}))
	assertZRange := func(t *testing.T, want term.Range) {
		t.Helper()
		list, ok := vi.cursor.LocationList(foldHighlightLocationListID)
		require.True(t, ok)
		loc, ok := list.Current()
		require.True(t, ok)
		assert.Equal(t, want.Start, loc.From)
		assert.Equal(t, want.End, loc.To)
	}

	_, handled := vi.Handle(term.Event{Type: term.EventKey, Ch: 'z'})
	require.True(t, handled)
	_, handled = vi.Handle(term.Event{Type: term.EventKey, Ch: 'h'})
	require.True(t, handled)
	_, ok := vi.Selection()
	assert.False(t, ok)
	assertZRange(t, term.Range{Start: term.Coordinates{X: 6}, End: term.Coordinates{X: 10}})
	assert.Equal(t, zMode, vi.mode())

	_, handled = vi.Handle(term.Event{Type: term.EventKey, Ch: 'h'})
	require.True(t, handled)
	_, ok = vi.Selection()
	assert.False(t, ok)
	assertZRange(t, term.Range{Start: term.Coordinates{}, End: term.Coordinates{X: 10}})
	assert.Equal(t, zMode, vi.mode())

	_, handled = vi.Handle(term.Event{Type: term.EventKey, Ch: 'l'})
	require.True(t, handled)
	_, ok = vi.Selection()
	assert.False(t, ok)
	assertZRange(t, term.Range{Start: term.Coordinates{X: 6}, End: term.Coordinates{X: 10}})
	assert.Equal(t, zMode, vi.mode())

	_, handled = vi.Handle(term.Event{Type: term.EventKey, Ch: 'v'})
	require.True(t, handled)
	selection, ok := vi.Selection()
	require.True(t, ok)
	assert.Equal(t, "beta", selection)
	assert.Equal(t, visualMode, vi.mode())
	assert.Equal(t, term.Coordinates{X: 6}, vi.cursorAtScroll())
}

func TestCellAtCursor(t *testing.T) {
	cases := []struct {
		input string
		cell  rune
	}{
		{"k", '\x00'},
		{"j", '/'},
		{"l", '*'},
		{"$", '*'},
	}

	width, height := 20, 10

	writer := term.NewStringWriter(width, height)
	vi := setupVi(t, snippet, 2)
	vi.Resize(width, height)

	for _, tcase := range cases {
		for _, r := range tcase.input {
			_, handled := vi.Handle(term.Event{Type: term.EventKey, Ch: r})
			require.True(t, handled)

			vi.Draw(writer)

			err := writer.Flush()
			require.NoError(t, err)
		}
		c, ok := vi.cursor.Cell()
		if tcase.cell == '\x00' {
			assert.False(t, ok)
		} else {
			assert.True(t, ok)
			assert.Equal(t, tcase.cell, c.Ch)
		}
	}
}

func TestMacroRecorder(t *testing.T) {
	key := func(ch rune) term.Event {
		return term.Event{Type: term.EventKey, Ch: ch}
	}

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "q is unhandled without a macro recorder",
			run: func(t *testing.T) {
				vi := setupVi(t, "", 2)
				before := vi.cursorAtScroll()

				_, handled := vi.Handle(key('q'))
				require.False(t, handled, "q should be unhandled when no recorder is set")
				require.Equal(t, before, vi.cursorAtScroll())
			},
		},
		{
			name: "qa starts recording into register a",
			run: func(t *testing.T) {
				rec := new(testMacroRecorder)
				vi := setupVi(t, "", 2, WithMacroRecorder(rec))

				_, handled := vi.Handle(key('q'))
				require.True(t, handled)
				require.False(t, rec.recording, "not recording yet, waiting for register key")

				_, handled = vi.Handle(key('a'))
				require.True(t, handled)
				require.True(t, rec.recording)
				require.Equal(t, []string{"a"}, rec.started)
			},
		},
		{
			name: "q stops an active recording",
			run: func(t *testing.T) {
				rec := new(testMacroRecorder)
				vi := setupVi(t, "", 2, WithMacroRecorder(rec))

				vi.Handle(key('q'))
				vi.Handle(key('a'))
				require.True(t, rec.recording)

				_, handled := vi.Handle(key('q'))
				require.True(t, handled)
				require.False(t, rec.recording)
				require.Equal(t, 1, rec.stopped)
			},
		},
		{
			name: "start and stop leaves handler in normal mode",
			run: func(t *testing.T) {
				rec := new(testMacroRecorder)
				vi := setupVi(t, "", 2, WithMacroRecorder(rec))

				vi.Handle(key('q'))
				vi.Handle(key('b'))
				vi.Handle(key('q'))

				require.Equal(t, normalMode, vi.mode())
			},
		},
		{
			name: "uppercase register is normalized to lowercase",
			run: func(t *testing.T) {
				rec := new(testMacroRecorder)
				vi := setupVi(t, "", 2, WithMacroRecorder(rec))

				vi.Handle(key('q'))
				vi.Handle(key('Z'))
				require.True(t, rec.recording)
				require.Equal(t, []string{"z"}, rec.started,
					"uppercase Z should normalize to register z")
			},
		},
		{
			name: "multiple different registers can be recorded sequentially",
			run: func(t *testing.T) {
				rec := new(testMacroRecorder)
				vi := setupVi(t, "", 2, WithMacroRecorder(rec))

				// Record into a.
				vi.Handle(key('q'))
				vi.Handle(key('a'))
				vi.Handle(key('q'))
				require.Equal(t, 1, rec.stopped)

				// Record into b.
				vi.Handle(key('q'))
				vi.Handle(key('b'))
				vi.Handle(key('q'))
				require.Equal(t, 2, rec.stopped)

				require.Equal(t, []string{"a", "b"}, rec.started)
			},
		},
		{
			name: "q followed by invalid register aborts without starting",
			run: func(t *testing.T) {
				rec := new(testMacroRecorder)
				vi := setupVi(t, "", 2, WithMacroRecorder(rec))

				vi.Handle(key('q'))
				// Space is not a valid register.
				_, handled := vi.Handle(term.Event{Type: term.EventKey, Key: term.KeySpace})
				require.True(t, handled, "invalid register key still consumed")
				require.False(t, rec.recording, "should not start recording for invalid register")
				require.Empty(t, rec.started)
			},
		},
		{
			name: "q followed by ctrl-modified key aborts without starting",
			run: func(t *testing.T) {
				rec := new(testMacroRecorder)
				vi := setupVi(t, "", 2, WithMacroRecorder(rec))

				vi.Handle(key('q'))
				_, handled := vi.Handle(term.Event{Type: term.EventKey, Ch: 'a', Mod: term.ModCtrl})
				require.True(t, handled)
				require.False(t, rec.recording, "ctrl-modified key is not a valid register")
				require.Empty(t, rec.started)
			},
		},
		{
			name: "keys between start and stop are dispatched normally",
			run: func(t *testing.T) {
				rec := new(testMacroRecorder)
				vi := setupVi(t, "hello", 2, WithMacroRecorder(rec))
				vi.Resize(20, 5)

				vi.Handle(key('q'))
				vi.Handle(key('a'))
				require.True(t, rec.recording)

				// Move cursor right while recording.
				vi.Handle(key('l'))
				vi.Handle(key('l'))

				require.Equal(t, term.Coordinates{X: 2}, vi.cursorAtScroll(),
					"cursor should have moved during recording")

				vi.Handle(key('q'))
				require.False(t, rec.recording)
			},
		},
		{
			name: "insert mode during recording works and q stops after Esc",
			run: func(t *testing.T) {
				rec := new(testMacroRecorder)
				vi := setupVi(t, "", 2, WithMacroRecorder(rec))
				vi.Resize(20, 5)

				// Start recording register a.
				vi.Handle(key('q'))
				vi.Handle(key('a'))
				require.True(t, rec.recording)

				// Enter insert, type, Esc.
				vi.Handle(key('i'))
				require.Equal(t, insertMode, vi.mode())
				vi.Handle(key('x'))
				vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyEsc})
				require.Equal(t, normalMode, vi.mode())

				require.Equal(t, "x", vi.less.Buffer().String())

				// Stop recording.
				vi.Handle(key('q'))
				require.False(t, rec.recording)
				require.Equal(t, 1, rec.stopped)
			},
		},
		{
			name: "recording into unnamed register uses default register ID",
			run: func(t *testing.T) {
				rec := new(testMacroRecorder)
				vi := setupVi(t, "", 2, WithMacroRecorder(rec))

				vi.Handle(key('q'))
				vi.Handle(key('"'))
				require.True(t, rec.recording)
				require.Equal(t, []string{clipboard.DefaultRegisterID}, rec.started)
			},
		},
		{
			name: "recording into special registers",
			run: func(t *testing.T) {
				for _, tt := range []struct {
					register rune
					wantID   string
				}{
					{register: '0', wantID: registerset.Normalize("0")},
					{register: '+', wantID: registerset.Normalize("+")},
					{register: '_', wantID: registerset.Normalize("_")},
					{register: '/', wantID: registerset.Normalize("/")},
					{register: '.', wantID: registerset.Normalize(".")},
					{register: '-', wantID: registerset.Normalize("-")},
				} {
					t.Run(string(tt.register), func(t *testing.T) {
						rec := new(testMacroRecorder)
						vi := setupVi(t, "", 2, WithMacroRecorder(rec))

						vi.Handle(key('q'))
						vi.Handle(key(tt.register))
						require.True(t, rec.recording)
						require.Equal(t, []string{tt.wantID}, rec.started)
					})
				}
			},
		},
		{
			name: "stop on q does not leave pending macro state",
			run: func(t *testing.T) {
				rec := new(testMacroRecorder)
				vi := setupVi(t, "hello world", 2, WithMacroRecorder(rec))
				vi.Resize(20, 5)

				// Record qa ... q.
				vi.Handle(key('q'))
				vi.Handle(key('a'))
				vi.Handle(key('l'))
				vi.Handle(key('q'))
				require.False(t, rec.recording)

				// The next 'l' should be a normal cursor movement, not
				// consumed by a stale pending-macro state.
				before := vi.cursorAtScroll()
				vi.Handle(key('l'))
				require.Equal(t, term.Coordinates{X: before.X + 1}, vi.cursorAtScroll())
			},
		},
		{
			name: "count is preserved while entering register name",
			run: func(t *testing.T) {
				// The vi handler sets doResetCount = false for q so
				// the count survives from the digit keys through q.
				rec := new(testMacroRecorder)
				vi := setupVi(t, "hello world", 2, WithMacroRecorder(rec))
				vi.Resize(20, 5)

				// Build up count "3", then q.
				vi.Handle(key('3'))
				vi.Handle(key('q'))

				// The count 3 should still be active while waiting for
				// the register name.
				require.Equal(t, 3, vi.count)
			},
		},
		{
			name: "q after stop can begin a new recording immediately",
			run: func(t *testing.T) {
				rec := new(testMacroRecorder)
				vi := setupVi(t, "", 2, WithMacroRecorder(rec))

				// First recording: qa ... q.
				vi.Handle(key('q'))
				vi.Handle(key('a'))
				vi.Handle(key('q'))
				require.Equal(t, 1, rec.stopped)

				// Immediately start a new recording: qb.
				vi.Handle(key('q'))
				vi.Handle(key('b'))
				require.True(t, rec.recording)
				require.Equal(t, []string{"a", "b"}, rec.started)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}

type testMacroRecorder struct {
	started   []string
	stopped   int
	recording bool
}

func (r *testMacroRecorder) Start(registerID string) {
	r.started = append(r.started, registerID)
	r.recording = true
}

func (r *testMacroRecorder) Stop() {
	r.stopped++
	r.recording = false
}

func (r *testMacroRecorder) IsRecording() bool {
	return r.recording
}

type testMacroPlayer struct {
	plays   []testPlay
	err     error
	playing bool
}

type testPlay struct {
	registerID string
	count      int
}

func (p *testMacroPlayer) Play(registerID string, count int) error {
	p.plays = append(p.plays, testPlay{registerID: registerID, count: count})
	return p.err
}

func (p *testMacroPlayer) IsPlaying() bool {
	return p.playing
}

func TestMacroPlayback(t *testing.T) {
	key := func(ch rune) term.Event {
		return term.Event{Type: term.EventKey, Ch: ch}
	}

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "@ is unhandled without a macro player",
			run: func(t *testing.T) {
				vi := setupVi(t, "", 2)
				_, handled := vi.Handle(key('@'))
				require.False(t, handled, "@ should be unhandled when no player is set")
			},
		},
		{
			name: "@a triggers playback of register a",
			run: func(t *testing.T) {
				player := new(testMacroPlayer)
				vi := setupVi(t, "", 2, WithMacroPlayer(player))

				_, handled := vi.Handle(key('@'))
				require.True(t, handled)
				require.Empty(t, player.plays, "not yet played, waiting for register key")

				_, handled = vi.Handle(key('a'))
				require.True(t, handled)
				require.Equal(t, []testPlay{{registerID: "a", count: 1}}, player.plays)
			},
		},
		{
			name: "@@ replays the last used register",
			run: func(t *testing.T) {
				player := new(testMacroPlayer)
				vi := setupVi(t, "", 2, WithMacroPlayer(player))

				// First play @a.
				vi.Handle(key('@'))
				vi.Handle(key('a'))
				require.Len(t, player.plays, 1)

				// @@ should replay register a.
				vi.Handle(key('@'))
				vi.Handle(key('@'))
				require.Len(t, player.plays, 2)
				require.Equal(t, "a", player.plays[1].registerID)
			},
		},
		{
			name: "@@ with no prior register is no-op",
			run: func(t *testing.T) {
				player := new(testMacroPlayer)
				vi := setupVi(t, "", 2, WithMacroPlayer(player))

				// @@ with no previous register (lastPlayedRegister == 0).
				vi.Handle(key('@'))
				_, handled := vi.Handle(key('@'))
				require.True(t, handled, "consumed but no play triggered")
				require.Empty(t, player.plays)
			},
		},
		{
			name: "count prefix is passed to Play",
			run: func(t *testing.T) {
				player := new(testMacroPlayer)
				vi := setupVi(t, "", 2, WithMacroPlayer(player))

				vi.Handle(key('5'))
				vi.Handle(key('@'))
				vi.Handle(key('b'))
				require.Equal(t, []testPlay{{registerID: "b", count: 5}}, player.plays)
			},
		},
		{
			name: "count is reset after playback",
			run: func(t *testing.T) {
				player := new(testMacroPlayer)
				vi := setupVi(t, "hello", 2, WithMacroPlayer(player))
				vi.Resize(20, 5)

				vi.Handle(key('3'))
				vi.Handle(key('@'))
				vi.Handle(key('a'))

				// Count should be reset to default after playback.
				require.Equal(t, 1, vi.count)
			},
		},
		{
			name: "uppercase register is normalized",
			run: func(t *testing.T) {
				player := new(testMacroPlayer)
				vi := setupVi(t, "", 2, WithMacroPlayer(player))

				vi.Handle(key('@'))
				vi.Handle(key('Z'))
				require.Equal(t, []testPlay{{registerID: "z", count: 1}}, player.plays)
			},
		},
		{
			name: "count is preserved while entering register name",
			run: func(t *testing.T) {
				player := new(testMacroPlayer)
				vi := setupVi(t, "", 2, WithMacroPlayer(player))

				vi.Handle(key('7'))
				vi.Handle(key('@'))
				// Count should still be preserved after @.
				require.Equal(t, 7, vi.count)
			},
		},
		{
			name: "@ followed by invalid register aborts without playing",
			run: func(t *testing.T) {
				player := new(testMacroPlayer)
				vi := setupVi(t, "", 2, WithMacroPlayer(player))

				vi.Handle(key('@'))
				// Space is not a valid register.
				_, handled := vi.Handle(term.Event{Type: term.EventKey, Key: term.KeySpace})
				require.True(t, handled, "invalid register key still consumed")
				require.Empty(t, player.plays)
			},
		},
		{
			name: "@ followed by ctrl-modified key aborts without playing",
			run: func(t *testing.T) {
				player := new(testMacroPlayer)
				vi := setupVi(t, "", 2, WithMacroPlayer(player))

				vi.Handle(key('@'))
				_, handled := vi.Handle(term.Event{Type: term.EventKey, Ch: 'a', Mod: term.ModCtrl})
				require.True(t, handled)
				require.Empty(t, player.plays)
			},
		},
		{
			name: "@ does not leave pending state after abort",
			run: func(t *testing.T) {
				player := new(testMacroPlayer)
				vi := setupVi(t, "hello", 2, WithMacroPlayer(player))
				vi.Resize(20, 5)

				vi.Handle(key('@'))
				// Invalid register: space.
				vi.Handle(term.Event{Type: term.EventKey, Key: term.KeySpace})

				// The next 'l' should be normal cursor movement.
				before := vi.cursorAtScroll()
				vi.Handle(key('l'))
				require.Equal(t, term.Coordinates{X: before.X + 1}, vi.cursorAtScroll())
			},
		},
		{
			name: "@@ after @a remembers a",
			run: func(t *testing.T) {
				player := new(testMacroPlayer)
				vi := setupVi(t, "", 2, WithMacroPlayer(player))

				vi.Handle(key('@'))
				vi.Handle(key('a'))

				// Clear plays to isolate.
				player.plays = nil

				// 3@@ should repeat last register (a) three times.
				vi.Handle(key('3'))
				vi.Handle(key('@'))
				vi.Handle(key('@'))
				require.Equal(t, []testPlay{{registerID: "a", count: 3}}, player.plays)
			},
		},
		{
			name: "null key event does not consume pendingPlayback",
			run: func(t *testing.T) {
				player := new(testMacroPlayer)
				vi := setupVi(t, "", 2, WithMacroPlayer(player))

				vi.Handle(key('@'))
				// Null event (e.g. from releasing Shift after typing @).
				vi.Handle(term.Event{Type: term.EventKey})
				// The real register key should still trigger playback.
				vi.Handle(key('a'))
				require.Equal(t, []testPlay{{registerID: "a", count: 1}}, player.plays)
			},
		},
		{
			name: "null key event does not consume pendingMacro",
			run: func(t *testing.T) {
				rec := new(testMacroRecorder)
				vi := setupVi(t, "", 2, WithMacroRecorder(rec))

				vi.Handle(key('q'))
				// Null event (e.g. from releasing Shift).
				vi.Handle(term.Event{Type: term.EventKey})
				// The real register key should still start recording.
				vi.Handle(key('a'))
				require.Equal(t, []string{"a"}, rec.started)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}

func TestMatchingRuneHighlight(t *testing.T) {
	t.Run("normal movement", func(t *testing.T) {
		width, height := 20, 10

		vi := setupVi(t, snippet, 2)
		vi.Resize(width, height)

		for _, r := range "jjjjjjj" {
			_, handled := vi.Handle(term.Event{Type: term.EventKey, Ch: r})
			require.True(t, handled)
		}

		c, ok := vi.cursor.Cell()
		assert.True(t, ok)
		require.Equal(t, '{', c.Ch)

		list, ok := vi.cursor.LocationList(matchingLocID)
		require.True(t, ok)
		loc, ok := list.Current()
		require.True(t, ok)
		assert.Equal(t, tcell.AttrReverse, loc.Attr.Attrs)
		assert.Equal(t, term.Coordinates{Y: 31}, loc.From)
		assert.Equal(t, term.Coordinates{Y: 31, X: 1}, loc.To)
	})

	t.Run("SetCursorAtScroll", func(t *testing.T) {
		width, height := 20, 10

		vi := setupVi(t, snippet, 2)
		vi.Resize(width, height)

		vi.setCursorAtScroll(term.Coordinates{Y: 7})

		c, ok := vi.cursor.Cell()
		assert.True(t, ok)
		require.Equal(t, '{', c.Ch)

		list, ok := vi.cursor.LocationList(matchingLocID)
		require.True(t, ok)
		loc, ok := list.Current()
		require.True(t, ok)
		assert.Equal(t, tcell.AttrReverse, loc.Attr.Attrs)
		assert.Equal(t, term.Coordinates{Y: 31}, loc.From)
		assert.Equal(t, term.Coordinates{Y: 31, X: 1}, loc.To)
	})

	t.Run("MoveToNextLocation", func(t *testing.T) {
		width, height := 20, 10

		vi := setupVi(t, snippet, 2)
		vi.Resize(width, height)

		vi.cursor.SetLocationList(textapi.LocationPriorityInfo, "mylist",
			textapi.LocationSlice([]textapi.Location{{From: term.Coordinates{Y: 7}}}))
		vi.moveToNextLocation("mylist")

		c, ok := vi.cursor.Cell()
		assert.True(t, ok)
		require.Equal(t, '{', c.Ch)

		list, ok := vi.cursor.LocationList(matchingLocID)
		require.True(t, ok)
		loc, ok := list.Current()
		require.True(t, ok)
		assert.Equal(t, tcell.AttrReverse, loc.Attr.Attrs)
		assert.Equal(t, term.Coordinates{Y: 31}, loc.From)
		assert.Equal(t, term.Coordinates{Y: 31, X: 1}, loc.To)
	})

	t.Run("MoveToPrevLocation", func(t *testing.T) {
		width, height := 20, 10

		vi := setupVi(t, snippet, 2)
		vi.Resize(width, height)

		vi.cursor.SetLocationList(textapi.LocationPriorityInfo, "mylist",
			textapi.LocationSlice([]textapi.Location{{From: term.Coordinates{Y: 7}}}))
		vi.moveToPrevLocation("mylist")

		c, ok := vi.cursor.Cell()
		assert.True(t, ok)
		require.Equal(t, '{', c.Ch)

		list, ok := vi.cursor.LocationList(matchingLocID)
		require.True(t, ok)
		loc, ok := list.Current()
		require.True(t, ok)
		assert.Equal(t, tcell.AttrReverse, loc.Attr.Attrs)
		assert.Equal(t, term.Coordinates{Y: 31}, loc.From)
		assert.Equal(t, term.Coordinates{Y: 31, X: 1}, loc.To)
	})
}

func TestViIntegrationSequence(t *testing.T) {
	cases := []handlertest.SequenceTestCase{
		{"",
			`▐                   
/*                  
 * Check if the curr
 * diff buffers.    
 */                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              NORMAL`},
		{"jjjj",
			`                    
/*                  
 * Check if the curr
 * diff buffers.    
▐*/                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              NORMAL`},
		{"/NULL>jjjjjjjjkkkkkkkk", // test vi.free anchoring
			`  if (wp == ▐ULL)   
  {                 
    i = diff_buf_idx
    if (i != DB_COUN
    {               
    curtab->tp_diffb
    curtab->tp_diff_
    diff_redraw(TRUE
    searching 'NULL'
              NORMAL`},
		{"Ahello",
			`f (wp == NULL)hello▐
                    
 i = diff_buf_idx(wi
 if (i != DB_COUNT) 
 {                  
 curtab->tp_diffbuf[
 curtab->tp_diff_inv
 diff_redraw(TRUE); 
 }                  
              INSERT`},
		{"<hhhhC<",
			`f (wp == NULL▐      
                    
 i = diff_buf_idx(wi
 if (i != DB_COUNT) 
 {                  
 curtab->tp_diffbuf[
 curtab->tp_diff_inv
 diff_redraw(TRUE); 
 }                  
              NORMAL`},
		{"p",
			`f (wp == NULL)hell▐ 
                    
 i = diff_buf_idx(wi
 if (i != DB_COUNT) 
 {                  
 curtab->tp_diffbuf[
 curtab->tp_diff_inv
 diff_redraw(TRUE); 
 }                  
              NORMAL`},
		{"F(",
			`f ▐wp == NULL)hello 
                    
 i = diff_buf_idx(wi
 if (i != DB_COUNT) 
 {                  
 curtab->tp_diffbuf[
 curtab->tp_diff_inv
 diff_redraw(TRUE); 
 }                  
              NORMAL`},
		{"f)",
			`f (wp == NULL▐hello 
                    
 i = diff_buf_idx(wi
 if (i != DB_COUNT) 
 {                  
 curtab->tp_diffbuf[
 curtab->tp_diff_inv
 diff_redraw(TRUE); 
 }                  
              NORMAL`},
		{"F=",
			`f (wp =▐ NULL)hello 
                    
 i = diff_buf_idx(wi
 if (i != DB_COUNT) 
 {                  
 curtab->tp_diffbuf[
 curtab->tp_diff_inv
 diff_redraw(TRUE); 
 }                  
              NORMAL`},
		{",", // , after F= reverses direction: searches forward, no = found, cursor stays
			`f (wp =▐ NULL)hello 
                    
 i = diff_buf_idx(wi
 if (i != DB_COUNT) 
 {                  
 curtab->tp_diffbuf[
 curtab->tp_diff_inv
 diff_redraw(TRUE); 
 }                  
              NORMAL`},
		{";", // ; after F= repeats backward: finds = at col 6
			`f (wp ▐= NULL)hello 
                    
 i = diff_buf_idx(wi
 if (i != DB_COUNT) 
 {                  
 curtab->tp_diffbuf[
 curtab->tp_diff_inv
 diff_redraw(TRUE); 
 }                  
              NORMAL`},
		{"D",
			`f (wp▐              
                    
 i = diff_buf_idx(wi
 if (i != DB_COUNT) 
 {                  
 curtab->tp_diffbuf[
 curtab->tp_diff_inv
 diff_redraw(TRUE); 
 }                  
              NORMAL`},
		{"sbrillo",
			`f (wpbrillo▐        
                    
 i = diff_buf_idx(wi
 if (i != DB_COUNT) 
 {                  
 curtab->tp_diffbuf[
 curtab->tp_diff_inv
 diff_redraw(TRUE); 
 }                  
              INSERT`},
		{"<hhhhhhR == NULL)",
			`f (w == NULL)▐      
                    
 i = diff_buf_idx(wi
 if (i != DB_COUNT) 
 {                  
 curtab->tp_diffbuf[
 curtab->tp_diff_inv
 diff_redraw(TRUE); 
 }                  
             REPLACE`},
		{"<r]h",
			`f (w == NUL▐]       
                    
 i = diff_buf_idx(wi
 if (i != DB_COUNT) 
 {                  
 curtab->tp_diffbuf[
 curtab->tp_diff_inv
 diff_redraw(TRUE); 
 }                  
              NORMAL`},
		{"VypzH",
			`  if (w == NULL]    
 ▐if (w == NULL]    
  {                 
    i = diff_buf_idx
    if (i != DB_COUN
    {               
    curtab->tp_diffb
    curtab->tp_diff_
    diff_redraw(TRUE
              NORMAL`},
		{"/i =>",
			`  if (w == NULL]    
  if (w == NULL]    
  {                 
    ▐ = diff_buf_idx
    if (i != DB_COUN
    {               
    curtab->tp_diffb
    curtab->tp_diff_
     searching 'i ='
              NORMAL`},
		{"dd",
			`  if (w == NULL]    
  if (w == NULL]    
  {                 
 ▐  if (i != DB_COUN
    {               
    curtab->tp_diffb
    curtab->tp_diff_
    diff_redraw(TRUE
    }               
              NORMAL`},
		{"h",
			`  if (w == NULL]    
  if (w == NULL]    
  {                 
 ▐  if (i != DB_COUN
    {               
    curtab->tp_diffb
    curtab->tp_diff_
    diff_redraw(TRUE
    }               
              NORMAL`},
		{"h",
			`  if (w == NULL]    
  if (w == NULL]    
  {                 
 ▐  if (i != DB_COUN
    {               
    curtab->tp_diffb
    curtab->tp_diff_
    diff_redraw(TRUE
    }               
              NORMAL`},
		{"df=",
			`  if (w == NULL]    
  if (w == NULL]    
  {                 
▐DB_COUNT)          
    {               
    curtab->tp_diffb
    curtab->tp_diff_
    diff_redraw(TRUE
    }               
              NORMAL`},
		{"/i>kkFDcndi<ldw",
			`  if (w == NULL]    
  if (w == NULL]    
  {                 
di▐urtab->tp_diff_in
    diff_redraw(TRUE
    }               
  }                 
  }                 
  else              
              NORMAL`},
		{"gg",
			`▐                   
/*                  
 * Check if the curr
 * diff buffers.    
 */                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              NORMAL`},
		{"12gg",
			` * diff buffers.    
 */                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
  int        i;     
                    
 ▐if (!win->w_p_diff
              NORMAL`},
		{"gg",
			`▐                   
/*                  
 * Check if the curr
 * diff buffers.    
 */                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              NORMAL`},
		{"jjyyp",
			`                    
/*                  
 * Check if the curr
▐* Check if the curr
 * diff buffers.    
 */                 
  void              
diff_buf_adjust(win_
{                   
              NORMAL`},
		{"lllcc *",
			`                    
/*                  
 * Check if the curr
 *▐                 
 * diff buffers.    
 */                 
  void              
diff_buf_adjust(win_
{                   
              INSERT`},
		{"<jllipotato<",
			`                    
/*                  
 * Check if the curr
 *                  
 * potat▐diff buffer
 */                 
  void              
diff_buf_adjust(win_
{                   
              NORMAL`},
		{"7h1j1l1k1h7l",
			`                    
/*                  
 * Check if the curr
 *                  
 * potat▐diff buffer
 */                 
  void              
diff_buf_adjust(win_
{                   
              NORMAL`},
		{"1g1g1g1gg",
			`▐                   
/*                  
 * Check if the curr
 *                  
 * potatodiff buffer
 */                 
  void              
diff_buf_adjust(win_
{                   
              NORMAL`},
		{"gg5gg",
			`                    
/*                  
 * Check if the curr
 *                  
▐* potatodiff buffer
 */                 
  void              
diff_buf_adjust(win_
{                   
              NORMAL`},
		{"8l3h",
			`                    
/*                  
 * Check if the curr
 *                  
 * po▐atodiff buffer
 */                 
  void              
diff_buf_adjust(win_
{                   
              NORMAL`},
		{"10000000000000000000000000000000000000000h",
			`                    
/*                  
 * Check if the curr
 *                  
▐* potatodiff buffer
 */                 
  void              
diff_buf_adjust(win_
{                   
              NORMAL`},
		{"3j",
			`                    
/*                  
 * Check if the curr
 *                  
 * potatodiff buffer
 */                 
  void              
▐iff_buf_adjust(win_
{                   
              NORMAL`},
		{"8l",
			`                    
/*                  
 * Check if the curr
 *                  
 * potatodiff buffer
 */                 
  void              
diff_buf▐adjust(win_
{                   
              NORMAL`},
		// 10j means move 10 rows down; not go to start of line (0) and move 1 row down
		{"10jhh",
			`  win_T  *wp;       
  int        i;     
                    
  if (!win->w_p_diff
  {                 
  /* When there is n
   * it from the dif
  FOR_ALL_WINDOWS(wp
    if (▐p->w_buffer
              NORMAL`},
		{"2kl",
			`  win_T  *wp;       
  int        i;     
                    
  if (!win->w_p_diff
  {                 
  /* When there is n
   * it ▐rom the dif
  FOR_ALL_WINDOWS(wp
    if (wp->w_buffer
              NORMAL`},
		{"10k",
			` *▐                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
  int        i;     
                    
  if (!win->w_p_diff
  {                 
              NORMAL`},
		{"922337203685477580719973197k",
			`▐                   
/*                  
 * Check if the curr
 *                  
 * potatodiff buffer
 */                 
  void              
diff_buf_adjust(win_
{                   
              NORMAL`},
		{"jj",
			`                    
/*                  
▐* Check if the curr
 *                  
 * potatodiff buffer
 */                 
  void              
diff_buf_adjust(win_
{                   
              NORMAL`},
		{"4\\$h", // '4' will be forgotten because of $ (go to last line char)
			`                    
                    
ved from the list ▐f
                    
                    
                    
                    
                    
                    
              NORMAL`},
		{"98765432123456789098765432100000000000000013414l",
			`                    
                    
ved from the list o▐
                    
                    
                    
                    
                    
                    
              NORMAL`},
		{"33<h",
			`                    
                    
ved from the list ▐f
                    
                    
                    
                    
                    
                    
              NORMAL`},
		{"6gg3h",
			`                    
/*                  
 * Check if the curr
 *                  
 * potatodiff buffer
▐*/                 
  void              
diff_buf_adjust(win_
{                   
              NORMAL`},
		{"111<0gg",
			`▐                   
/*                  
 * Check if the curr
 *                  
 * potatodiff buffer
 */                 
  void              
diff_buf_adjust(win_
{                   
              NORMAL`},
		{"99999ggkk",
			`  {                 
dicurtab->tp_diff_in
    diff_redraw(TRUE
    }               
  }                 
  }                 
 ▐else              
  diff_buf_add(win->
}                   
              NORMAL`},
		{"kkk3dd",
			`  {                 
dicurtab->tp_diff_in
    diff_redraw(TRUE
 ▐diff_buf_add(win->
}                   
                    
                    
                    
                    
              NORMAL`},
		{"\\$zH",
			`                    
tp_diff_invalid = TR
edraw(TRUE);        
_add(win->w_buffer)▐
                    
                    
                    
                    
                    
              NORMAL`},
		{"\\$44\\^", // 44 should be ignored
			`{                   
curtab->tp_diff_inva
  diff_redraw(TRUE);
▐iff_buf_add(win->w_
                    
                    
                    
                    
                    
              NORMAL`},
		{"rpl",
			`{                   
curtab->tp_diff_inva
  diff_redraw(TRUE);
p▐ff_buf_add(win->w_
                    
                    
                    
                    
                    
              NORMAL`},
		{"Rabcdef<",
			`{                   
curtab->tp_diff_inva
  diff_redraw(TRUE);
pabcde▐f_add(win->w_
                    
                    
                    
                    
                    
              NORMAL`},
		{"?diff>",
			`{                   
curtab->tp_diff_inva
  ▐iff_redraw(TRUE);
pabcdeff_add(win->w_
                    
                    
                    
                    
    searching 'diff'
              NORMAL`},
		{"n",
			`{                   
curtab->tp_▐iff_inva
  diff_redraw(TRUE);
pabcdeff_add(win->w_
                    
                    
                    
                    
    searching 'diff'
              NORMAL`},
		{"N",
			`{                   
curtab->tp_diff_inva
  ▐iff_redraw(TRUE);
pabcdeff_add(win->w_
                    
                    
                    
                    
    searching 'diff'
              NORMAL`},
		{"j0tf",
			` {                  
icurtab->tp_diff_inv
   diff_redraw(TRUE)
 pabcd▐ff_add(win->w
                    
                    
                    
                    
    searching 'diff'
              NORMAL`},
		{";",
			` {                  
icurtab->tp_diff_inv
   diff_redraw(TRUE)
 pabcde▐f_add(win->w
                    
                    
                    
                    
    searching 'diff'
              NORMAL`},
		{"hTa",
			` {                  
icurtab->tp_diff_inv
   diff_redraw(TRUE)
 pa▐cdeff_add(win->w
                    
                    
                    
                    
    searching 'diff'
              NORMAL`},
		{",",
			` {                  
icurtab->tp_diff_inv
   diff_redraw(TRUE)
 pabcdeff▐add(win->w
                    
                    
                    
                    
    searching 'diff'
              NORMAL`},
	}

	vi := setupViIntegration(t, snippet, 2)
	handlertest.TestHandlerSequence(t, vi, 20, 10, cases)
}

func TestViToggleCase(t *testing.T) {
	t.Run("normal mode toggles char and advances", func(t *testing.T) {
		vi := setupVi(t, "Hello", 2)
		vi.Resize(20, 5)

		// cursor is at 'H', toggle it
		vi.Handle(term.Event{Type: term.EventKey, Ch: '~'})
		assert.Equal(t, "hello", vi.less.Buffer().String())
		assert.Equal(t, term.Coordinates{X: 1}, vi.cursor.Coordinates())

		// toggle 'e' -> 'E'
		vi.Handle(term.Event{Type: term.EventKey, Ch: '~'})
		assert.Equal(t, "hEllo", vi.less.Buffer().String())
		assert.Equal(t, term.Coordinates{X: 2}, vi.cursor.Coordinates())

		// toggle 'l' -> 'L'
		vi.Handle(term.Event{Type: term.EventKey, Ch: '~'})
		assert.Equal(t, "hELlo", vi.less.Buffer().String())
		assert.Equal(t, term.Coordinates{X: 3}, vi.cursor.Coordinates())
	})

	t.Run("normal mode at end of line stays put", func(t *testing.T) {
		vi := setupVi(t, "Ab", 2)
		vi.Resize(20, 5)

		// move to last char 'b'
		vi.Handle(term.Event{Type: term.EventKey, Ch: '$'})
		assert.Equal(t, term.Coordinates{X: 1}, vi.cursor.Coordinates())

		// toggle 'b' -> 'B', cursor cannot advance further
		vi.Handle(term.Event{Type: term.EventKey, Ch: '~'})
		assert.Equal(t, "AB", vi.less.Buffer().String())
	})

	t.Run("visual mode toggles selection", func(t *testing.T) {
		vi := setupVi(t, "Hello World", 2)
		vi.Resize(20, 5)

		// select "Hello" (v then 4l to extend selection through 'o')
		vi.Handle(term.Event{Type: term.EventKey, Ch: 'v'})
		for i := 0; i < 4; i++ {
			vi.Handle(term.Event{Type: term.EventKey, Ch: 'l'})
		}

		// toggle case of selection
		vi.Handle(term.Event{Type: term.EventKey, Ch: '~'})
		assert.Equal(t, "hELLO World", vi.less.Buffer().String())
		assert.Equal(t, normalMode, vi.mode())
	})
}

func TestViCaseChangeOperators(t *testing.T) {
	type testCase struct {
		name          string
		content       string
		events        string
		expectContent string
		expectMode    viMode
	}

	suite := []testCase{
		// ===== gu (lowercase) with motions =====
		{"guw lowercases a word", "Hello World", "guw", "hello World", normalMode},
		{"gu$ lowercases to end of line", "Hello WORLD", "gu$", "hello world", normalMode},
		{"gue lowercases to end of word", "HELLO World", "gue", "hello World", normalMode},
		{"guu lowercases whole line", "HELLO WORLD", "guu", "hello world", normalMode},
		{"gub lowercases backward word", "Hello WORLD", "Wgub", "hello wORLD", normalMode},
		{"guB lowercases backward WORD", "Hello-Two WORLD", "WguB", "hello-two wORLD", normalMode},
		{"guW lowercases forward WORD", "Hello-World test", "guW", "hello-world test", normalMode},
		{"guE lowercases to end of WORD", "Hello-World test", "guE", "hello-world test", normalMode},
		{"gu^ lowercases to first non-blank", "  Hello WORLD", "WWgu^", "  hello wORLD", normalMode},
		{"gu0 lowercases to start of line", "  Hello WORLD", "Wgu0", "  hello WORLD", normalMode},
		{"gul lowercases single char", "Hello World", "gul", "hello World", normalMode},
		{"guh lowercases backward single char (no-op at col 0)", "Hello World", "guh", "Hello World", normalMode},
		{"guh lowercases backward char from col 1", "HEllo", "lguh", "hello", normalMode},
		{"guf lowercases to char", "HELLO World", "gufo", "hello world", normalMode},

		// ===== gU (uppercase) with motions =====
		{"gUw uppercases a word", "hello world", "gUw", "HELLO world", normalMode},
		{"gU$ uppercases to end of line", "hello world", "gU$", "HELLO WORLD", normalMode},
		{"gUe uppercases to end of word", "hello world", "gUe", "HELLO world", normalMode},
		{"gUU uppercases whole line", "hello world", "gUU", "HELLO WORLD", normalMode},
		{"gUb uppercases backward word", "hello world", "WgUb", "HELLO World", normalMode},
		{"gUB uppercases backward WORD", "hello-two world", "WgUB", "HELLO-TWO World", normalMode},
		{"gUW uppercases forward WORD", "hello-world test", "gUW", "HELLO-WORLD test", normalMode},
		{"gUE uppercases to end of WORD", "hello-world test", "gUE", "HELLO-WORLD test", normalMode},
		{"gU^ uppercases to first non-blank", "  hello world", "fogU^", "  HELLO world", normalMode},
		{"gU0 uppercases to start of line", "  hello world", "fogU0", "  HELLO world", normalMode},
		{"gUl uppercases single char", "hello world", "gUl", "HEllo world", normalMode},
		{"gUh uppercases backward single char (no-op at col 0)", "hello world", "gUh", "hello world", normalMode},
		{"gUh uppercases backward char from col 1", "hello", "lgUh", "HEllo", normalMode},
		{"gUf uppercases to char", "hello world", "gUfo", "HELLO world", normalMode},

		// ===== g~ (toggle case) with motions =====
		{"g~w toggles case of a word", "Hello World", "g~w", "hELLO World", normalMode},
		{"g~$ toggles case to end of line", "Hello World", "g~$", "hELLO wORLD", normalMode},
		{"g~e toggles case to end of word", "Hello World", "g~e", "hELLO World", normalMode},
		{"g~~ toggles case of whole line", "Hello World", "g~~", "hELLO wORLD", normalMode},
		{"g~b toggles backward word", "Hello World", "Wg~b", "hELLO world", normalMode},
		{"g~W toggles forward WORD", "Hello-World Test", "g~W", "hELLO-wORLD Test", normalMode},
		{"g~E toggles to end of WORD", "Hello-World Test", "g~E", "hELLO-wORLD Test", normalMode},
		{"g~l toggles single char", "Hello World", "g~l", "hEllo World", normalMode},
		{"g~h toggles backward single char (no-op at col 0)", "Hello World", "g~h", "Hello World", normalMode},
		{"g~h toggles backward char from col 1", "Hello", "lg~h", "hEllo", normalMode},

		// ===== Text objects — inner/around word =====
		{"guiw lowercases inner word", "Hello WORLD Test", "guiw", "hello WORLD Test", normalMode},
		{"gUiw uppercases inner word", "hello world test", "gUiw", "HELLO world test", normalMode},
		{"g~iw toggles case of inner word", "Hello WORLD Test", "g~iw", "hELLO WORLD Test", normalMode},
		{"guaw lowercases around word", "Hello WORLD Test", "guaw", "hello WORLD Test", normalMode},
		{"gUaw uppercases around word", "hello world test", "gUaw", "HELLO world test", normalMode},
		{"g~aw toggles around word", "Hello WORLD Test", "g~aw", "hELLO WORLD Test", normalMode},

		// ===== Text objects — inner/around WORD =====
		{"guiW lowercases inner WORD", "Hello-World Test", "guiW", "hello-world Test", normalMode},
		{"gUiW uppercases inner WORD", "hello-world test", "gUiW", "HELLO-WORLD test", normalMode},
		{"g~iW toggles inner WORD", "Hello-World Test", "g~iW", "hELLO-wORLD Test", normalMode},
		{"guaW lowercases around WORD", "Hello-World Test", "guaW", "hello-world Test", normalMode},

		// ===== Text objects — quotes =====
		{"gui\" lowercases inner double quotes", `say "HELLO WORLD" now`, "fHgui\"", `say "hello world" now`, normalMode},
		{"gUi\" uppercases inner double quotes", `say "hello world" now`, "fhgUi\"", `say "HELLO WORLD" now`, normalMode},
		{"g~i\" toggles inner double quotes", `say "Hello World" now`, "fHg~i\"", `say "hELLO wORLD" now`, normalMode},
		{"gua\" lowercases around double quotes", `say "HELLO WORLD" now`, "fHgua\"", `say "hello world" now`, normalMode},
		{"gui' lowercases inner single quotes", "say 'HELLO' now", "fHgui'", "say 'hello' now", normalMode},
		{"gui` lowercases inner backtick", "say `HELLO` now", "fHgui`", "say `hello` now", normalMode},

		// ===== Text objects — parentheses/blocks =====
		{"guib lowercases inner parens", "call(HELLO, WORLD)", "f(guib", "call(hello, world)", normalMode},
		{"gUib uppercases inner parens", "call(hello, world)", "f(gUib", "call(HELLO, WORLD)", normalMode},
		{"guab lowercases around parens", "call(HELLO, WORLD)", "f(guab", "call(hello, world)", normalMode},
		{"gui) lowercases inner parens alias", "call(HELLO, WORLD)", "f(gui)", "call(hello, world)", normalMode},
		{"gui( lowercases inner parens alias 2", "call(HELLO, WORLD)", "f(gui(", "call(hello, world)", normalMode},

		// ===== Text objects — braces =====
		{"guiB lowercases inner braces", "fn{HELLO WORLD}", "f{guiB", "fn{hello world}", normalMode},
		{"gUiB uppercases inner braces", "fn{hello world}", "f{gUiB", "fn{HELLO WORLD}", normalMode},
		{"guaB lowercases around braces", "fn{HELLO WORLD}", "f{guaB", "fn{hello world}", normalMode},

		// ===== Text objects — brackets =====
		{"gui[ lowercases inner brackets", "arr[HELLO]", "f[gui[", "arr[hello]", normalMode},
		{"gUi[ uppercases inner brackets", "arr[hello]", "f[gUi[", "arr[HELLO]", normalMode},

		// ===== Text objects — angle brackets =====
		{"guit lowercases inner angle", "a <HELLO> b", "f<guit", "a <hello> b", normalMode},
		{"gUit uppercases inner angle", "a <hello> b", "f<gUit", "a <HELLO> b", normalMode},

		// ===== g-sub motions (ge, gE) =====
		{"guge lowercases from cursor to end of previous word", "HELLO WORLD", "Wguge", "HELLo wORLD", normalMode},
		{"gUge uppercases from cursor to end of previous word", "hello world", "WgUge", "hellO World", normalMode},
		{"g~ge toggles from cursor to end of previous word", "Hello World", "Wg~ge", "HellO world", normalMode},
		{"gugE lowercases from cursor to end of previous WORD", "HELLO-TWO WORLD", "WgugE", "HELLO-TWo wORLD", normalMode},
		{"gUgE uppercases from cursor to end of previous WORD", "hello-two world", "WgUgE", "hello-twO World", normalMode},
		{"g~gE toggles from cursor to end of previous WORD", "Hello-Two World", "Wg~gE", "Hello-TwO world", normalMode},

		// ===== Whole-line double forms =====
		{"guu on already lowercase line is no-op", "hello world", "guu", "hello world", normalMode},
		{"gUU on already uppercase line is no-op", "HELLO WORLD", "gUU", "HELLO WORLD", normalMode},
		{"g~~ on single char line", "A", "g~~", "a", normalMode},
		{"guu on single char line", "A", "guu", "a", normalMode},
		{"gUU on single char line", "a", "gUU", "A", normalMode},

		// ===== Cursor positioning before operator =====
		{"guw from middle of word lowercases from cursor forward", "HELLO", "llguw", "HEllO", normalMode},
		{"gUw from middle of word uppercases from cursor forward", "hello", "llgUw", "heLLo", normalMode},
		{"g~w from middle of word toggles from cursor forward", "Hello", "llg~w", "HeLLo", normalMode},
		{"gue from middle of word lowercases to end of word", "HELLO WORLD", "llgue", "HEllo WORLD", normalMode},
		{"gu$ from middle of line lowercases to end", "HELLO WORLD", "llgu$", "HEllo world", normalMode},
		{"gU$ from middle of line uppercases to end", "hello world", "llgU$", "heLLO WORLD", normalMode},

		// ===== Multiline =====
		{"guj lowercases two lines", "HELLO\nWORLD", "guj", "hello\nworld", normalMode},
		{"gUj uppercases two lines", "hello\nworld", "gUj", "HELLO\nWORLD", normalMode},
		{"g~j toggles two lines", "Hello\nWorld", "g~j", "hELLO\nwORLD", normalMode},
		{"guk lowercases upward to previous line", "HELLO\nWORLD", "jguk", "hello\nworld", normalMode},
		{"gUk uppercases upward to previous line", "hello\nworld", "jgUk", "HELLO\nWORLD", normalMode},
		{"guu on first line of multiline only affects first line", "HELLO\nWORLD", "guu", "hello\nWORLD", normalMode},
		{"gUU on second line of multiline only affects that line", "hello\nworld", "jgUU", "hello\nWORLD", normalMode},

		// ===== Empty / whitespace content =====
		{"guw on whitespace only", "   ", "guw", "   ", normalMode},
		{"gUU on empty buffer", "", "gUU", "", normalMode},
		{"guu on empty buffer", "", "guu", "", normalMode},
		{"g~~ on empty buffer", "", "g~~", "", normalMode},

		// ===== Punctuation / non-alpha characters =====
		{"guw on punctuation does not change it", "!@#$%^&*", "guw", "!@#$%^&*", normalMode},
		{"gUw on digits does not change them", "abc123def", "gUw", "ABC123DEf", normalMode},
		{"guw on mixed alpha-punct word", "Hello!", "guw", "hello!", normalMode},
		{"g~w on digits mixed", "a1B2c3", "g~w", "A1b2C3", normalMode},

		// ===== Count + operator =====
		{"2guw lowercases 2 words", "HELLO WORLD TEST", "2guw", "hello WORLD TEST", normalMode},
		{"2gUw uppercases 2 words", "hello world test", "2gUw", "HELLO world test", normalMode},
		{"3guw lowercases 3 words (count does not propagate to motion)", "ONE TWO THREE FOUR", "3guw", "one TWO THREE FOUR", normalMode},
		{"2gUe uppercases word end (count does not propagate)", "hello world test", "2gUe", "HELLO world test", normalMode},
		{"guj lowercases 2 lines from first line", "HELLO\nWORLD\nTEST", "guj", "hello\nworld\nTEST", normalMode},

		// ===== Visual mode case change =====
		{"vu then selection lowercases visual", "HELLO WORLD", "vevu", "hello WORLD", normalMode},
		{"vU then selection uppercases visual", "hello world", "vevU", "HELLO world", normalMode},
		{"v~ toggles case in visual mode", "Hello World", "vev~", "hELLO World", normalMode},
		{"V line visual then u lowercases entire line", "HELLO WORLD", "Vu", "hello world", normalMode},
		{"V line visual then U uppercases entire line", "hello world", "VU", "HELLO WORLD", normalMode},

		// ===== Sentence / paragraph text objects =====
		{"guis lowercases inner sentence", "HELLO WORLD. BYE NOW.", "guis", "hello world. BYE NOW.", normalMode},
		{"guip lowercases inner paragraph", "HELLO WORLD", "guip", "hello world", normalMode},
		{"gUip uppercases inner paragraph", "hello world", "gUip", "HELLO WORLD", normalMode},

		// ===== Edge: line boundaries =====
		{"gu$ at end of line is single char", "HELLO", "$gu$", "HELLo", normalMode},
		{"gU$ at end of line is single char", "hello", "$gU$", "hellO", normalMode},

		// ===== Invalid sequences return to normal mode =====
		{"gu followed by invalid key returns to normal", "HELLO", "guZ", "HELLO", normalMode},
		{"gU followed by invalid key returns to normal", "hello", "gUZ", "hello", normalMode},
		{"g~ followed by invalid key returns to normal", "Hello", "g~Z", "Hello", normalMode},
	}

	for _, tcase := range suite {
		t.Run(tcase.name, func(t *testing.T) {
			vi := setupVi(t, tcase.content, 2)
			vi.Resize(40, 10)

			for _, ch := range tcase.events {
				vi.Handle(term.Event{Type: term.EventKey, Ch: ch})
			}

			assert.Equal(t, tcase.expectContent, vi.less.Buffer().String())
			assert.Equal(t, tcase.expectMode, vi.mode())
		})
	}
}

func TestViCaseChangeTextObjects(t *testing.T) {
	type caseChangeTextObjectCase struct {
		name       string
		content    string
		at         *term.Coordinates
		seq        string
		wantBuffer string
		wantMode   viMode
		wantCursor *term.Coordinates
	}

	run := func(t *testing.T, tc caseChangeTextObjectCase) {
		t.Helper()
		vi := setupVi(t, tc.content, 2)
		vi.Resize(80, 10)
		if tc.at != nil {
			vi.setCursorAtScroll(*tc.at)
		}
		for _, event := range tc.seq {
			vi.Handle(term.Event{Type: term.EventKey, Ch: event})
		}
		if tc.wantBuffer != "" || tc.content == "" {
			assert.Equal(t, tc.wantBuffer, vi.less.Buffer().String())
		}
		assert.Equal(t, tc.wantMode, vi.mode())
		if tc.wantCursor != nil {
			assert.Equal(t, *tc.wantCursor, vi.cursor.CursorAtScroll())
		}
	}

	for _, tc := range []caseChangeTextObjectCase{
		// inner word
		{
			name:       "guiw lowercases inner word at start",
			content:    "HELLO world",
			seq:        "guiw",
			wantBuffer: "hello world",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 0, Y: 0},
		},
		{
			name:       "gUiw uppercases inner word in middle",
			content:    "one two three",
			at:         &term.Coordinates{X: 4, Y: 0},
			seq:        "gUiw",
			wantBuffer: "one TWO three",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 4, Y: 0},
		},
		{
			name:       "g~iw toggles inner word at end",
			content:    "one two Three",
			at:         &term.Coordinates{X: 10, Y: 0},
			seq:        "g~iw",
			wantBuffer: "one two tHREE",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 8, Y: 0},
		},
		// around word
		{
			name:       "guaw lowercases around word",
			content:    "ONE TWO THREE",
			at:         &term.Coordinates{X: 4, Y: 0},
			seq:        "guaw",
			wantBuffer: "ONE two THREE",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 4, Y: 0},
		},
		// inner quotes
		{
			name:       "gui\" lowercases content inside double quotes",
			content:    `say "HELLO WORLD" now`,
			at:         &term.Coordinates{X: 5, Y: 0},
			seq:        "gui\"",
			wantBuffer: `say "hello world" now`,
			wantMode:   normalMode,
		},
		{
			name:       "gUi' uppercases content inside single quotes",
			content:    "say 'hello world' now",
			at:         &term.Coordinates{X: 5, Y: 0},
			seq:        "gUi'",
			wantBuffer: "say 'HELLO WORLD' now",
			wantMode:   normalMode,
		},
		// inner parens
		{
			name:       "guib lowercases inside parentheses",
			content:    "fn(ABC, DEF)",
			at:         &term.Coordinates{X: 3, Y: 0},
			seq:        "guib",
			wantBuffer: "fn(abc, def)",
			wantMode:   normalMode,
		},
		{
			name:       "gUi) uppercases inside parentheses",
			content:    "fn(abc, def)",
			at:         &term.Coordinates{X: 3, Y: 0},
			seq:        "gUi)",
			wantBuffer: "fn(ABC, DEF)",
			wantMode:   normalMode,
		},
		// inner braces
		{
			name:       "guiB lowercases inside braces",
			content:    "fn{ABC DEF}",
			at:         &term.Coordinates{X: 3, Y: 0},
			seq:        "guiB",
			wantBuffer: "fn{abc def}",
			wantMode:   normalMode,
		},
		// inner brackets
		{
			name:       "gui[ lowercases inside brackets",
			content:    "arr[ABC]",
			at:         &term.Coordinates{X: 4, Y: 0},
			seq:        "gui[",
			wantBuffer: "arr[abc]",
			wantMode:   normalMode,
		},
		// inner angle brackets
		{
			name:       "guit lowercases inside angle brackets",
			content:    "tag <ABC> end",
			at:         &term.Coordinates{X: 5, Y: 0},
			seq:        "guit",
			wantBuffer: "tag <abc> end",
			wantMode:   normalMode,
		},
		// around quotes
		{
			name:       "gua\" lowercases around double quotes",
			content:    `say "HELLO" now`,
			at:         &term.Coordinates{X: 5, Y: 0},
			seq:        "gua\"",
			wantBuffer: `say "hello" now`,
			wantMode:   normalMode,
		},
		// around parens
		{
			name:       "guab lowercases around parens",
			content:    "fn(ABC, DEF)",
			at:         &term.Coordinates{X: 3, Y: 0},
			seq:        "guab",
			wantBuffer: "fn(abc, def)",
			wantMode:   normalMode,
		},
		// empty quotes - no change
		{
			name:       "gui\" on empty quotes is no-op",
			content:    `say "" now`,
			at:         &term.Coordinates{X: 4, Y: 0},
			seq:        "gui\"",
			wantBuffer: `say "" now`,
			wantMode:   normalMode,
		},
		// empty parens - no change
		{
			name:       "guib on empty parens is no-op",
			content:    "fn()",
			at:         &term.Coordinates{X: 2, Y: 0},
			seq:        "guib",
			wantBuffer: "fn()",
			wantMode:   normalMode,
		},
		// inner WORD
		{
			name:       "guiW lowercases inner WORD including punctuation",
			content:    "ONE-TWO three",
			seq:        "guiW",
			wantBuffer: "one-two three",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 0, Y: 0},
		},
		// cursor on second line
		{
			name:       "gUiw on second line uppercases word there",
			content:    "hello\nworld",
			at:         &term.Coordinates{X: 0, Y: 1},
			seq:        "gUiw",
			wantBuffer: "hello\nWORLD",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 0, Y: 1},
		},
		// invalid text object key returns to normal mode
		{
			name:       "guiz invalid text object returns normal mode",
			content:    "HELLO WORLD",
			seq:        "guiz",
			wantBuffer: "HELLO WORLD",
			wantMode:   normalMode,
		},
		// retarget: i then a
		{
			name:       "guia retargets from inner to around",
			content:    "ONE TWO THREE",
			at:         &term.Coordinates{X: 4, Y: 0},
			seq:        "guiaw",
			wantBuffer: "ONE two THREE",
			wantMode:   normalMode,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestViCaseChangeSnapshots(t *testing.T) {
	const (
		width  = 30
		height = 5
	)

	newVi := func(t *testing.T, content string) tui.Handler {
		t.Helper()
		return setupViIntegration(t, content, 2)
	}

	t.Run("gu motions sequential", func(t *testing.T) {
		cases := []handlertest.SequenceTestCase{
			{InputSequence: "guw", Expected: "hello▐World                   \n                              \n                              \n                              \n                        NORMAL"},
			{InputSequence: "Wgue", Expected: "hello worl▐                   \n                              \n                              \n                              \n                        NORMAL"},
		}
		handlertest.RunHandlerSequence(t, newVi(t, "Hello World"), width, height, cases)
	})

	t.Run("gU motions sequential", func(t *testing.T) {
		cases := []handlertest.SequenceTestCase{
			{InputSequence: "gUw", Expected: "HELLO▐world                   \n                              \n                              \n                              \n                        NORMAL"},
			{InputSequence: "WgUe", Expected: "HELLO WORL▐                   \n                              \n                              \n                              \n                        NORMAL"},
		}
		handlertest.RunHandlerSequence(t, newVi(t, "hello world"), width, height, cases)
	})

	t.Run("g~ motions sequential", func(t *testing.T) {
		cases := []handlertest.SequenceTestCase{
			{InputSequence: "g~w", Expected: "hELLO▐World                   \n                              \n                              \n                              \n                        NORMAL"},
			{InputSequence: "Wg~$", Expected: "hELLO wORL▐                   \n                              \n                              \n                              \n                        NORMAL"},
		}
		handlertest.RunHandlerSequence(t, newVi(t, "Hello World"), width, height, cases)
	})

	t.Run("whole line operators", func(t *testing.T) {
		fn := func(t *testing.T) tui.Handler {
			return newVi(t, "Hello World")
		}
		cases := []handlertest.SequenceTestCase{
			{InputSequence: "guu", Expected: "hello worl▐                   \n                              \n                              \n                              \n                        NORMAL"},
			{InputSequence: "gUU", Expected: "HELLO WORL▐                   \n                              \n                              \n                              \n                        NORMAL"},
			{InputSequence: "g~~", Expected: "hELLO wORL▐                   \n                              \n                              \n                              \n                        NORMAL"},
		}
		handlertest.RunHandlerIsolated(t, fn, width, height, cases)
	})

	t.Run("case change with text objects", func(t *testing.T) {
		fn := func(t *testing.T) tui.Handler {
			return newVi(t, `say "HELLO WORLD" now`)
		}
		cases := []handlertest.SequenceTestCase{
			{InputSequence: "fHgui\"", Expected: "say \"▐ello world\" now         \n                              \n                              \n                              \n                        NORMAL"},
			{InputSequence: "fhg~i\"", Expected: "▐ay \"HELLO WORLD\" now         \n                              \n                              \n                              \n                        NORMAL"},
		}
		handlertest.RunHandlerIsolated(t, fn, width, height, cases)
	})

	t.Run("case change with ge motion", func(t *testing.T) {
		cases := []handlertest.SequenceTestCase{
			{InputSequence: "$", Expected: "ONE TWO-THREE,FOU▐            \n                              \n                              \n                              \n                        NORMAL"},
			{InputSequence: "guge", Expected: "ONE TWO-THREE▐four            \n                              \n                              \n                              \n                        NORMAL"},
		}
		handlertest.RunHandlerSequence(t, newVi(t, "ONE TWO-THREE,FOUR"), width, height, cases)
	})

	t.Run("case change multiline", func(t *testing.T) {
		fn := func(t *testing.T) tui.Handler {
			return newVi(t, "Hello\nWorld")
		}
		cases := []handlertest.SequenceTestCase{
			{InputSequence: "guj", Expected: "hello                         \n▐orld                         \n                              \n                              \n                        NORMAL"},
		}
		handlertest.RunHandlerIsolated(t, fn, width, height, cases)
	})

	t.Run("visual mode case change", func(t *testing.T) {
		fn := func(t *testing.T) tui.Handler {
			return newVi(t, "Hello World")
		}
		cases := []handlertest.SequenceTestCase{
			{InputSequence: "veu", Expected: "hell▐ World                   \n                              \n                              \n                              \n                        NORMAL"},
			{InputSequence: "veU", Expected: "HELL▐ World                   \n                              \n                              \n                              \n                        NORMAL"},
			{InputSequence: "ve~", Expected: "hELL▐ World                   \n                              \n                              \n                              \n                        NORMAL"},
		}
		handlertest.RunHandlerIsolated(t, fn, width, height, cases)
	})
}

func TestViCount(t *testing.T) {
	motions := []rune{'h', 'j', 'k', 'l'}
	for _, motion := range motions {
		t.Run(fmt.Sprintf(
			"20%c multiplies motion by 20", // doesn't go necessarily to start of line
			motion),
			func(t *testing.T) {
				vi := setupVi(t, snippet, 2)
				vi.Resize(100, 100)
				vi.cursor.MoveToScroll(term.Coordinates{X: 3, Y: 3})
				vi.Handle(term.Event{Type: term.EventKey, Ch: '2'})
				vi.Handle(term.Event{Type: term.EventKey, Ch: '0'})
				vi.Handle(term.Event{Type: term.EventKey, Ch: motion})
				switch motion {
				case 'h':
					assert.Equal(t, vi.cursor.Coordinates(), term.Coordinates{X: 0, Y: 3})
				case 'j':
					assert.Equal(t, vi.cursor.Coordinates(), term.Coordinates{X: 5, Y: 23})
				case 'k':
					assert.Equal(t, vi.cursor.Coordinates(), term.Coordinates{X: 0, Y: 0})
				case 'l':
					assert.Equal(t, vi.cursor.Coordinates(), term.Coordinates{X: 15, Y: 3})
				}
			})
	}
}

func TestViX(t *testing.T) {
	suite := []struct {
		name          string
		moveCursorFn  func(*viHandlerImpl)
		content       string
		events        string
		expectContent string
		expectCoords  *term.Coordinates
	}{
		{
			name:          "X deletes character before cursor",
			content:       "abcd",
			events:        "llX",
			expectContent: "acd",
			expectCoords:  &term.Coordinates{X: 1, Y: 0},
		},
		{
			name:          "counted X deletes multiple previous characters",
			content:       "abcd",
			events:        "lll3X",
			expectContent: "d",
			expectCoords:  &term.Coordinates{X: 0, Y: 0},
		},
		{
			name:          "X at start of line conflates with previous line",
			content:       "ab\ncd",
			events:        "jX",
			expectContent: "abcd",
			expectCoords:  &term.Coordinates{X: 2, Y: 0},
		},
		{
			name:          "X at start of buffer is a no-op",
			content:       "abcd",
			events:        "X",
			expectContent: "abcd",
			expectCoords:  &term.Coordinates{X: 0, Y: 0},
		},
		{
			name: "10000000000000000000000000000X deletes up to buffer start",
			moveCursorFn: func(vi *viHandlerImpl) {
				vi.cursor.MoveEndLine()
			},
			content:       "abcd",
			events:        "10000000000000000000000000000X",
			expectContent: "d",
			expectCoords:  &term.Coordinates{X: 0, Y: 0},
		},
	}

	for _, tcase := range suite {
		t.Run(tcase.name, func(t *testing.T) {
			vi := setupVi(t, tcase.content, 2)
			vi.Resize(20, 9)
			vi.Draw(term.NoopWriter{})
			if tcase.moveCursorFn != nil {
				tcase.moveCursorFn(vi)
			}

			for _, eventChar := range tcase.events {
				vi.Handle(term.Event{Type: term.EventKey, Ch: eventChar})
			}

			assert.Equal(t, tcase.expectContent, vi.less.Buffer().String())
			if tcase.expectCoords != nil {
				assert.Equal(t, *tcase.expectCoords, vi.cursor.Coordinates())
			}
			assert.Equal(t, normalMode, vi.mode())
		})
	}
}

func TestVidd(t *testing.T) {
	suite := []struct {
		name             string
		moveCursorFn     func(*viHandlerImpl)
		content          string
		events           string
		expectContent    string
		expectCoords     term.Coordinates
		expectCoordsWrap term.Coordinates
	}{
		{
			name: "2dd",
			moveCursorFn: func(vi *viHandlerImpl) {
				vi.cursor.MoveToScroll(term.Coordinates{Y: 2})
			},
			content:          "0000\n1111\n2222\n3333\n4444\n5555\n",
			events:           "2dd",
			expectContent:    "0000\n1111\n5555\n",
			expectCoords:     term.Coordinates{Y: 2},
			expectCoordsWrap: term.Coordinates{Y: 4},
		},
		{
			name: "3dd from last line",
			moveCursorFn: func(vi *viHandlerImpl) {
				vi.cursor.MoveLastLine()
			},
			content:          "0000\n1111\n2222\n3333\n4444\n5555\n",
			events:           "3dd",
			expectContent:    "0000\n1111\n2222\n3333\n4444\n5555",
			expectCoords:     term.Coordinates{Y: 5, X: 3},
			expectCoordsWrap: term.Coordinates{Y: 7, X: 1},
		},
		{
			name: "3dd from second to last line",
			moveCursorFn: func(vi *viHandlerImpl) {
				vi.cursor.MoveLastLine()
				vi.cursor.MoveLineUp()
			},
			content:          "0000\n1111\n2222\n3333\n4444\n5555\n",
			events:           "3dd",
			expectContent:    "0000\n1111\n2222\n3333\n4444\n",
			expectCoords:     term.Coordinates{Y: 5},
			expectCoordsWrap: term.Coordinates{Y: 6},
		},
		{
			name:             "10dd from first line wipes all content",
			content:          "0000\n1111\n2222\n3333\n4444\n5555\n",
			events:           "10dd",
			expectContent:    "",
			expectCoords:     term.Coordinates{Y: 0},
			expectCoordsWrap: term.Coordinates{Y: 0},
		},
		{
			name:             "999999999999999999999999999999999999dd",
			content:          "0000\n1111\n2222\n3333\n4444",
			events:           "999999999999999999999999999999999999dd",
			expectContent:    "",
			expectCoords:     term.Coordinates{Y: 0},
			expectCoordsWrap: term.Coordinates{Y: 0},
		},
	}

	for _, tcase := range suite {
		for _, wrap := range []bool{false, true} {
			name := tcase.name
			if wrap {
				name += " (wrap)"
			}
			t.Run(name, func(t *testing.T) {
				vi := setupVi(t, tcase.content, 2, WithWrap(wrap))
				if wrap {
					vi.Resize(2, 9)
				} else {
					vi.Resize(10, 9)
				}
				vi.Draw(term.NoopWriter{})
				if tcase.moveCursorFn != nil {
					tcase.moveCursorFn(vi)
				}
				for _, eventChar := range tcase.events {
					vi.Handle(term.Event{Type: term.EventKey, Ch: eventChar})
				}
				if wrap {
					assert.Equal(t, tcase.expectCoordsWrap, vi.cursor.Coordinates())
				} else {
					assert.Equal(t, tcase.expectCoords, vi.cursor.Coordinates())
				}
				assert.Equal(t, tcase.expectContent, vi.less.Buffer().String())
			})
		}
	}
}

func TestViSubstituteLine(t *testing.T) {
	suite := []struct {
		name          string
		content       string
		events        string
		expectContent string
	}{
		{
			name:          "S on single line clears and enters insert",
			content:       "hello world",
			events:        "S",
			expectContent: "",
		},
		{
			name:          "S on indented line clears content",
			content:       "    indented line",
			events:        "S",
			expectContent: "",
		},
		{
			name:          "S on middle line only affects current line",
			content:       "aaa\nbbb\nccc",
			events:        "jS",
			expectContent: "aaa\n\nccc",
		},
		{
			name:          "S then type replacement text",
			content:       "old text\nsecond line",
			events:        "Snew text",
			expectContent: "new text\nsecond line",
		},
		{
			name:          "S behaves same as cc",
			content:       "hello world\nsecond line",
			events:        "Sreplaced",
			expectContent: "replaced\nsecond line",
		},
	}

	for _, tcase := range suite {
		t.Run(tcase.name, func(t *testing.T) {
			vi := setupVi(t, tcase.content, 2)
			vi.Resize(20, 9)
			vi.Draw(term.NoopWriter{})

			for _, eventChar := range tcase.events {
				vi.Handle(term.Event{Type: term.EventKey, Ch: eventChar})
			}

			assert.Equal(t, tcase.expectContent, vi.less.Buffer().String())
			assert.Equal(t, insertMode, vi.mode())
		})
	}
}

func TestLocationMessage(t *testing.T) {
	cases := []handlertest.SequenceTestCase{
		{"j",
			`                    
▐*                  
 * Check if the curr
 * diff buffers.    
 */                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
            durrdurr`},
		{"jj",
			`                    
/*                  
▐* Check if the curr
 * diff buffers.    
 */                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
  int        i;     `},
		{"jjk",
			`                    
▐*                  
 * Check if the curr
 * diff buffers.    
 */                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
            durrdurr`},
	}

	newVi := func(t *testing.T) tui.Handler {
		vi := setupVi(t, snippet, 2)
		vi.setLocationList(textapi.LocationPriorityInfo, "id",
			textapi.LocationSlice([]textapi.Location{
				{
					Message: "durrdurr",
					From:    term.Coordinates{Y: 1},
					To:      term.Coordinates{Y: 1, X: 5},
				},
			}))
		return vi
	}
	handlertest.RunHandlerIsolated(t, newVi, 20, 10, cases)
}

func TestVidfd(t *testing.T) {
	cases := []handlertest.SequenceTestCase{
		{"jjdfd",
			`                    
/*                  
▐be added to or remo
 * diff buffers.    
 */                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              NORMAL`},
		{"jjcfc",
			`                    
/*                  
▐ if the current buf
 * diff buffers.    
 */                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              INSERT`},
	}

	newVi := func(t *testing.T) tui.Handler {
		return setupViIntegration(t, snippet, 2)
	}
	handlertest.TestHandlerIsolated(t, newVi, 20, 10, cases)
}

func TestWrapMoveDownLastLogicalLine(t *testing.T) {
	sample := `abcde
fghih
ijklm
opkrs
tuvxy
11111
22222
33333
44444
55555
66666`
	vi := setupVi(t, sample, 2, WithWrap(true))
	vi.Resize(3, 20)
	vi.Draw(term.NoopWriter{})
	for _, eventChar := range "jjjjjjjjjjjjjjjjjjjjj" {
		vi.Handle(term.Event{Type: term.EventKey, Ch: eventChar})
	}
	assert.Equal(t, term.Coordinates{X: 3, Y: 10}, vi.cursor.CursorAtScroll())
	vi.cursor.SelectLine()
	assert.Equal(t, "66666\n", vi.cursor.Selection())
}

func TestVisualMoveToChar(t *testing.T) {
	sample := `abcde`
	vi := setupVi(t, sample, 2, WithWrap(false))
	vi.Resize(10, 20)
	vi.Draw(term.NoopWriter{})

	for _, eventChar := range "vfc" {
		vi.Handle(term.Event{Type: term.EventKey, Ch: eventChar})
	}
	selection, ok := vi.Selection()
	require.True(t, ok)
	assert.Equal(t, "abc", selection)
}

func handleViEvents(vi *viHandlerImpl, events []term.Event) {
	for _, ev := range events {
		vi.Handle(ev)
	}
}

func viPositionVisible(vi *viHandlerImpl, pos term.Coordinates) bool {
	win, ok := vi.cursor.WindowCoordinates(pos)
	if !ok {
		return false
	}
	return win.X >= 0 && win.Y >= 0 &&
		win.X < vi.less.Scroll().Width() &&
		win.Y < vi.less.Scroll().SizeHeight()
}

func swappedVisualEnds(anchor, cursor term.Coordinates) (term.Coordinates, term.Coordinates) {
	return anchor, cursor
}

func swappedBlockCorners(anchor, cursor term.Coordinates) (term.Coordinates, term.Coordinates) {
	if anchor.X == cursor.X || anchor.Y == cursor.Y {
		// Same column or same row: O acts like o (full end swap).
		return anchor, cursor // wantAfterCursor=anchor, wantAfterAnchor=cursor
	}
	return term.Coordinates{X: anchor.X, Y: cursor.Y},
		term.Coordinates{X: cursor.X, Y: anchor.Y}
}

func TestVisualSwapSelectionEnd(t *testing.T) {
	type testCase struct {
		name                    string
		content                 string
		width                   int
		height                  int
		before                  []term.Event
		wantMode                viMode
		wantBeforeCursor        term.Coordinates
		wantBeforeAnchor        term.Coordinates
		wantBeforeCursorVisible bool
		wantBeforeAnchorVisible bool
		wantAfterCursorVisible  bool
		wantAfterAnchorVisible  bool
	}

	key := func(ch rune) term.Event {
		return term.Event{Type: term.EventKey, Ch: ch}
	}

	suite := []testCase{
		{
			name:                    "same line middle",
			content:                 "abcd",
			width:                   10,
			height:                  20,
			before:                  []term.Event{key('v'), key('l'), key('l')},
			wantMode:                visualMode,
			wantBeforeCursor:        term.Coordinates{X: 2},
			wantBeforeAnchor:        term.Coordinates{},
			wantBeforeCursorVisible: true,
			wantBeforeAnchorVisible: true,
			wantAfterCursorVisible:  true,
			wantAfterAnchorVisible:  true,
		},
		{
			name:                    "same line beginning to end of line",
			content:                 "abcd",
			width:                   10,
			height:                  20,
			before:                  []term.Event{key('v'), key('$')},
			wantMode:                visualMode,
			wantBeforeCursor:        term.Coordinates{X: 4},
			wantBeforeAnchor:        term.Coordinates{},
			wantBeforeCursorVisible: true,
			wantBeforeAnchorVisible: true,
			wantAfterCursorVisible:  true,
			wantAfterAnchorVisible:  true,
		},
		{
			name:                    "same line end of line to middle",
			content:                 "abcd",
			width:                   10,
			height:                  20,
			before:                  []term.Event{key('$'), key('v'), key('h'), key('h')},
			wantMode:                visualMode,
			wantBeforeCursor:        term.Coordinates{X: 1},
			wantBeforeAnchor:        term.Coordinates{X: 3},
			wantBeforeCursorVisible: true,
			wantBeforeAnchorVisible: true,
			wantAfterCursorVisible:  true,
			wantAfterAnchorVisible:  true,
		},
		{
			name:                    "different lines middle of line",
			content:                 "abcd\nefgh\nijkl",
			width:                   10,
			height:                  20,
			before:                  []term.Event{key('l'), key('v'), key('j'), key('l')},
			wantMode:                visualMode,
			wantBeforeCursor:        term.Coordinates{X: 2, Y: 1},
			wantBeforeAnchor:        term.Coordinates{X: 1},
			wantBeforeCursorVisible: true,
			wantBeforeAnchorVisible: true,
			wantAfterCursorVisible:  true,
			wantAfterAnchorVisible:  true,
		},
		{
			name:                    "start of file",
			content:                 "abcd\nefgh\nijkl",
			width:                   10,
			height:                  20,
			before:                  []term.Event{key('v'), key('j'), key('j'), key('l')},
			wantMode:                visualMode,
			wantBeforeCursor:        term.Coordinates{X: 1, Y: 2},
			wantBeforeAnchor:        term.Coordinates{},
			wantBeforeCursorVisible: true,
			wantBeforeAnchorVisible: true,
			wantAfterCursorVisible:  true,
			wantAfterAnchorVisible:  true,
		},
		{
			name:                    "end of file",
			content:                 "abcd\nefgh\nijkl",
			width:                   10,
			height:                  20,
			before:                  []term.Event{key('G'), key('$'), key('v'), key('h'), key('h')},
			wantMode:                visualMode,
			wantBeforeCursor:        term.Coordinates{X: 1, Y: 2},
			wantBeforeAnchor:        term.Coordinates{X: 3, Y: 2},
			wantBeforeCursorVisible: true,
			wantBeforeAnchorVisible: true,
			wantAfterCursorVisible:  true,
			wantAfterAnchorVisible:  true,
		},
		{
			name:                    "selection start outside rendered view vertically",
			content:                 "line0\nline1\nline2\nline3\nline4\nline5",
			width:                   10,
			height:                  3,
			before:                  []term.Event{key('v'), key('j'), key('j'), key('j'), key('j')},
			wantMode:                visualMode,
			wantBeforeCursor:        term.Coordinates{Y: 4},
			wantBeforeAnchor:        term.Coordinates{},
			wantBeforeCursorVisible: true,
			wantBeforeAnchorVisible: false,
			wantAfterCursorVisible:  true,
			wantAfterAnchorVisible:  false,
		},
		{
			name:                    "selection start outside rendered view horizontally",
			content:                 "0123456789abcdef",
			width:                   4,
			height:                  2,
			before:                  []term.Event{key('v'), key('$')},
			wantMode:                visualMode,
			wantBeforeCursor:        term.Coordinates{X: 16},
			wantBeforeAnchor:        term.Coordinates{},
			wantBeforeCursorVisible: false,
			wantBeforeAnchorVisible: false,
			wantAfterCursorVisible:  true,
			wantAfterAnchorVisible:  false,
		},
		{
			name:                    "visual line mode different lines",
			content:                 "alpha\nbeta\ngamma\ndelta",
			width:                   10,
			height:                  20,
			before:                  []term.Event{key('V'), key('j'), key('j')},
			wantMode:                visualLineMode,
			wantBeforeCursor:        term.Coordinates{Y: 2},
			wantBeforeAnchor:        term.Coordinates{},
			wantBeforeCursorVisible: true,
			wantBeforeAnchorVisible: true,
			wantAfterCursorVisible:  true,
			wantAfterAnchorVisible:  true,
		},
		{
			name:                    "visual line mode selection start outside rendered view vertically",
			content:                 "alpha\nbeta\ngamma\ndelta\nepsilon\nzeta",
			width:                   10,
			height:                  3,
			before:                  []term.Event{key('V'), key('j'), key('j'), key('j'), key('j')},
			wantMode:                visualLineMode,
			wantBeforeCursor:        term.Coordinates{Y: 4},
			wantBeforeAnchor:        term.Coordinates{},
			wantBeforeCursorVisible: true,
			wantBeforeAnchorVisible: false,
			wantAfterCursorVisible:  true,
			wantAfterAnchorVisible:  false,
		},
	}

	for _, tc := range suite {
		t.Run(tc.name, func(t *testing.T) {
			vi := setupVi(t, tc.content, 2, WithWrap(false))
			vi.Resize(tc.width, tc.height)
			vi.Draw(term.NoopWriter{})

			handleViEvents(vi, tc.before)

			require.Equal(t, tc.wantMode, vi.mode())

			beforeCursor := vi.cursor.CursorAtScroll()
			require.Equal(t, tc.wantBeforeCursor, beforeCursor)

			beforeAnchor, ok := vi.cursor.SelectionFrom()
			require.True(t, ok)
			require.Equal(t, tc.wantBeforeAnchor, beforeAnchor)
			assert.Equal(t, tc.wantBeforeCursorVisible, viPositionVisible(vi, beforeCursor))
			assert.Equal(t, tc.wantBeforeAnchorVisible, viPositionVisible(vi, beforeAnchor))

			selectionBefore := vi.cursor.Selection()
			require.NotEmpty(t, selectionBefore)
			bufferBefore := vi.less.Buffer().String()
			wantAfterCursor, wantAfterAnchor := swappedVisualEnds(beforeAnchor, beforeCursor)

			vi.Handle(key('o'))

			assert.Equal(t, tc.wantMode, vi.mode())
			assert.Equal(t, bufferBefore, vi.less.Buffer().String())
			assert.Equal(t, wantAfterCursor, vi.cursor.CursorAtScroll())
			assert.Equal(t, selectionBefore, vi.cursor.Selection())

			afterAnchor, ok := vi.cursor.SelectionFrom()
			require.True(t, ok)
			assert.Equal(t, wantAfterAnchor, afterAnchor)
			assert.Equal(t, tc.wantAfterCursorVisible, viPositionVisible(vi, wantAfterCursor))
			assert.Equal(t, tc.wantAfterAnchorVisible, viPositionVisible(vi, afterAnchor))
		})
	}
}

func TestTillCharacterMotion(t *testing.T) {
	sample := `abcdabcdabcd`
	// Positions: a=0, b=1, c=2, d=3, a=4, b=5, c=6, d=7, a=8, b=9, c=10, d=11

	type testCase struct {
		name    string
		input   string
		expectX int
	}

	cases := []testCase{
		// t: till next character, cursor lands one before target
		{"t forward", "tc", 1},   // first 'c' at 2, land at 1
		{"t forward 2", "td", 2}, // first 'd' at 3, land at 2
		{"t no match", "tz", 0},  // not found, stay at 0
		{"t after moving right", "lltd", 2},

		// T: till prev character, cursor lands one after target
		{"T backward", "lllllllTa", 5},    // at col 7, prev 'a' at 4, land at 5
		{"T backward 2", "llllllllTb", 6}, // at col 8, prev 'b' at 5, land at 6
		{"T no match", "llTz", 2},         // not found, stays at 2
		{"T from end to c", "llllllllllTc", 7},

		// ; repeats t in same direction (till)
		{"t then semicolon", "tc;", 5}, // tc->1, ;->till next 'c' at 6, land at 5
		{"t then double semicolon", "tc;;", 9},

		// , repeats t in opposite direction (till)
		{"t then comma", "lllltc,", 3}, // at 4, tc->5 (before 'c' at 6), ,->till prev 'c' at 2, land at 3
		{"t then semicolon then comma", "tc;,", 3},

		// ; repeats T in same direction (backward till)
		{"T then semicolon", "llllllllTa;", 1}, // at 8, Ta->5, ;->till prev 'a' at 0, land at 1
		{"T then comma", "llllllllTa,", 7},

		// f then ; still works (regression)
		{"f then semicolon", "fc;", 6},   // fc->2, ;->next 'c' at 6
		{"f then comma", "llllllfc,", 6}, // at 6, fc->10, ,->prev 'c' at 6
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vi := setupVi(t, sample, 2, WithWrap(false))
			vi.Resize(20, 20)
			vi.Draw(term.NoopWriter{})

			for _, ch := range tc.input {
				vi.Handle(term.Event{Type: term.EventKey, Ch: ch})
			}
			vi.Draw(term.NoopWriter{})

			coords := vi.cursor.Coordinates()
			assert.Equal(t, tc.expectX, coords.X, "cursor X position")
		})
	}
}

func TestViDeleteAWord(t *testing.T) {
	cases := []handlertest.SequenceTestCase{
		{"jjjjjwdw",
			`                    
/*                  
 * Check if the curr
 * diff buffers.    
 */                 
 ▐                  
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              NORMAL`},
		{"jjjjjwcw",
			`                    
/*                  
 * Check if the curr
 * diff buffers.    
 */                 
  ▐                 
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              INSERT`},
		{"jjjjjwce",
			`                    
/*                  
 * Check if the curr
 * diff buffers.    
 */                 
  ▐                 
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              INSERT`},
		{"jjjjjwecb",
			`                    
/*                  
 * Check if the curr
 * diff buffers.    
 */                 
  ▐                 
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              INSERT`},
		{"jjjwwcw",
			`                    
/*                  
 * Check if the curr
 * ▐uffers.         
 */                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              INSERT`},
		{"jjjwwce",
			`                    
/*                  
 * Check if the curr
 * ▐buffers.        
 */                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              INSERT`},
	}

	newVi := func(t *testing.T) tui.Handler {
		return setupViIntegration(t, snippet, 2)
	}
	handlertest.TestHandlerIsolated(t, newVi, 20, 10, cases)
}

func TestViGoLeftEndWordMotions(t *testing.T) {
	const content = "one two-three, four"
	at := term.Coordinates{X: strings.Index(content, "four") + len("four") - 1, Y: 0}

	newVi := func(t *testing.T) *viHandlerImpl {
		t.Helper()
		vi := setupVi(t, content, 2)
		vi.Resize(80, 10)
		vi.setCursorAtScroll(at)
		vi.Draw(term.NoopWriter{})
		return vi
	}

	for _, tc := range []struct {
		name   string
		seq    string
		motion func(*viHandlerImpl) bool
	}{
		{
			name:   "ge moves to previous word end",
			seq:    "ge",
			motion: func(vi *viHandlerImpl) bool { return vi.cursor.MoveLeftEndWord() },
		},
		{
			name:   "gE moves to previous WORD end",
			seq:    "gE",
			motion: func(vi *viHandlerImpl) bool { return vi.cursor.MoveLeftEndWordGroup() },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expected := newVi(t)
			tc.motion(expected)
			require.NotEqual(t, at, expected.cursor.Coordinates())

			actual := newVi(t)
			for _, event := range tc.seq {
				_, handled := actual.Handle(term.Event{Type: term.EventKey, Ch: event})
				require.True(t, handled, "sequence %q failed on %q", tc.seq, string(event))
			}

			assert.Equal(t, expected.cursor.Coordinates(), actual.cursor.Coordinates())
			assert.Equal(t, normalMode, actual.mode())
		})
	}
}

func TestViOperatorGoLeftEndWordMotions(t *testing.T) {
	const content = "one two-three, four"
	at := term.Coordinates{X: strings.Index(content, "four") + len("four") - 1, Y: 0}

	newVi := func(t *testing.T) *viHandlerImpl {
		t.Helper()
		vi := setupVi(t, content, 2)
		vi.Resize(80, 10)
		vi.setCursorAtScroll(at)
		vi.Draw(term.NoopWriter{})
		return vi
	}

	for _, tc := range []struct {
		name          string
		seq           string
		motion        func(*viHandlerImpl) bool
		wantMode      viMode
		wantClipboard bool
	}{
		{
			name:     "delete ge",
			seq:      "dge",
			motion:   func(vi *viHandlerImpl) bool { return vi.cursor.MoveLeftEndWord() },
			wantMode: normalMode,
		},
		{
			name:     "delete gE",
			seq:      "dgE",
			motion:   func(vi *viHandlerImpl) bool { return vi.cursor.MoveLeftEndWordGroup() },
			wantMode: normalMode,
		},
		{
			name:     "change ge enters insert mode",
			seq:      "cge",
			motion:   func(vi *viHandlerImpl) bool { return vi.cursor.MoveLeftEndWord() },
			wantMode: insertMode,
		},
		{
			name:          "yank ge",
			seq:           "yge",
			motion:        func(vi *viHandlerImpl) bool { return vi.cursor.MoveLeftEndWord() },
			wantMode:      normalMode,
			wantClipboard: true,
		},
		{
			name:          "yank gE",
			seq:           "ygE",
			motion:        func(vi *viHandlerImpl) bool { return vi.cursor.MoveLeftEndWordGroup() },
			wantMode:      normalMode,
			wantClipboard: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expected := newVi(t)
			require.True(t, expected.cursor.Select())
			tc.motion(expected)
			require.NotEqual(t, at, expected.cursor.Coordinates())

			switch tc.seq[0] {
			case 'd', 'c':
				expected.cursor.DeleteSelection()
			case 'y':
				expected.copySelection()
			}

			switch tc.wantMode {
			case insertMode:
				expected.setInsertMode()
			default:
				expected.setNormalMode()
			}
			expected.doMoveToBounds()

			actual := newVi(t)
			for i, event := range tc.seq {
				_, handled := actual.Handle(term.Event{Type: term.EventKey, Ch: event})
				require.True(t, handled, "sequence %q failed on %q", tc.seq, string(event))
				if i == 1 {
					switch tc.seq[0] {
					case 'd', 'c':
						assert.Equal(t, deleteMode, actual.mode())
					case 'y':
						assert.Equal(t, yankMode, actual.mode())
					}
				}
			}

			assert.Equal(t, tc.wantMode, actual.mode())
			assert.Equal(t, expected.less.Buffer().String(), actual.less.Buffer().String())
			assert.Equal(t, expected.cursor.Coordinates(), actual.cursor.Coordinates())

			if tc.wantClipboard {
				expectedPaste, err := expected.config.clipboard.Paste(expected.config.defaultRegister)
				require.NoError(t, err)
				actualPaste, err := actual.config.clipboard.Paste(actual.config.defaultRegister)
				require.NoError(t, err)
				assert.Equal(t, expectedPaste.Text, actualPaste.Text)
			}
		})
	}
}

func TestViGoLeftEndWordMotionsAcrossLines(t *testing.T) {
	const content = "one\ntwo-three\nfour"
	at := term.Coordinates{X: len("four") - 1, Y: 2}

	newVi := func(t *testing.T) *viHandlerImpl {
		t.Helper()
		vi := setupVi(t, content, 2)
		vi.Resize(80, 10)
		vi.setCursorAtScroll(at)
		vi.Draw(term.NoopWriter{})
		return vi
	}

	for _, tc := range []struct {
		name   string
		seq    string
		motion func(*viHandlerImpl) bool
	}{
		{
			name:   "ge crosses to prior line word end",
			seq:    "ge",
			motion: func(vi *viHandlerImpl) bool { return vi.cursor.MoveLeftEndWord() },
		},
		{
			name:   "gE crosses to prior line WORD end",
			seq:    "gE",
			motion: func(vi *viHandlerImpl) bool { return vi.cursor.MoveLeftEndWordGroup() },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expected := newVi(t)
			tc.motion(expected)
			require.NotEqual(t, at, expected.cursor.Coordinates())

			actual := newVi(t)
			for _, event := range tc.seq {
				_, handled := actual.Handle(term.Event{Type: term.EventKey, Ch: event})
				require.True(t, handled, "sequence %q failed on %q", tc.seq, string(event))
			}

			assert.Equal(t, expected.cursor.Coordinates(), actual.cursor.Coordinates())
			assert.Equal(t, normalMode, actual.mode())
		})
	}
}

func TestViOperatorGoLeftEndWordMotionsAcrossLines(t *testing.T) {
	const content = "one\ntwo-three\nfour"
	at := term.Coordinates{X: len("four") - 1, Y: 2}

	newVi := func(t *testing.T) *viHandlerImpl {
		t.Helper()
		vi := setupVi(t, content, 2)
		vi.Resize(80, 10)
		vi.setCursorAtScroll(at)
		vi.Draw(term.NoopWriter{})
		return vi
	}

	for _, tc := range []struct {
		name     string
		seq      string
		motion   func(*viHandlerImpl) bool
		wantMode viMode
	}{
		{
			name:     "delete ge across lines",
			seq:      "dge",
			motion:   func(vi *viHandlerImpl) bool { return vi.cursor.MoveLeftEndWord() },
			wantMode: normalMode,
		},
		{
			name:     "change gE across lines",
			seq:      "cgE",
			motion:   func(vi *viHandlerImpl) bool { return vi.cursor.MoveLeftEndWordGroup() },
			wantMode: insertMode,
		},
		{
			name:     "yank ge across lines",
			seq:      "yge",
			motion:   func(vi *viHandlerImpl) bool { return vi.cursor.MoveLeftEndWord() },
			wantMode: normalMode,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expected := newVi(t)
			require.True(t, expected.cursor.Select())
			tc.motion(expected)
			require.NotEqual(t, at, expected.cursor.Coordinates())

			switch tc.seq[0] {
			case 'd', 'c':
				expected.cursor.DeleteSelection()
			case 'y':
				expected.copySelection()
			}

			if tc.wantMode == insertMode {
				expected.setInsertMode()
			} else {
				expected.setNormalMode()
			}
			expected.doMoveToBounds()

			actual := newVi(t)
			for _, event := range tc.seq {
				_, handled := actual.Handle(term.Event{Type: term.EventKey, Ch: event})
				require.True(t, handled, "sequence %q failed on %q", tc.seq, string(event))
			}

			assert.Equal(t, tc.wantMode, actual.mode())
			assert.Equal(t, expected.less.Buffer().String(), actual.less.Buffer().String())
			assert.Equal(t, expected.cursor.Coordinates(), actual.cursor.Coordinates())

			if tc.seq[0] == 'y' {
				expectedPaste, err := expected.config.clipboard.Paste(expected.config.defaultRegister)
				require.NoError(t, err)
				actualPaste, err := actual.config.clipboard.Paste(actual.config.defaultRegister)
				require.NoError(t, err)
				assert.Equal(t, expectedPaste.Text, actualPaste.Text)
			}
		})
	}
}

func TestViGoLeftEndWordSnapshots(t *testing.T) {
	const (
		content = "one two-three,four"
		width   = 24
		height  = 5
	)

	newVi := func(t *testing.T) tui.Handler {
		t.Helper()
		return setupViIntegration(t, content, 2)
	}

	t.Run("motions", func(t *testing.T) {
		cases := []handlertest.SequenceTestCase{
			{InputSequence: "$", Expected: "one two-three,fou▐      \n                        \n                        \n                        \n                  NORMAL"},
			{InputSequence: "ge", Expected: "one two-three▐four      \n                        \n                        \n                        \n                  NORMAL"},
			{InputSequence: "gE", Expected: "on▐ two-three,four      \n                        \n                        \n                        \n                  NORMAL"},
		}
		handlertest.RunHandlerSequence(t, newVi(t), width, height, cases)
	})

	t.Run("delete word motion", func(t *testing.T) {
		cases := []handlertest.SequenceTestCase{
			{InputSequence: "$", Expected: "one two-three,fou▐      \n                        \n                        \n                        \n                  NORMAL"},
			{InputSequence: "dge", Expected: "one two-thre▐           \n                        \n                        \n                        \n                  NORMAL"},
		}
		handlertest.RunHandlerSequence(t, newVi(t), width, height, cases)
	})

	t.Run("delete WORD motion", func(t *testing.T) {
		cases := []handlertest.SequenceTestCase{
			{InputSequence: "$", Expected: "one two-three,fou▐      \n                        \n                        \n                        \n                  NORMAL"},
			{InputSequence: "dgE", Expected: "o▐                      \n                        \n                        \n                        \n                  NORMAL"},
		}
		handlertest.RunHandlerSequence(t, newVi(t), width, height, cases)
	})

	t.Run("change word motion", func(t *testing.T) {
		cases := []handlertest.SequenceTestCase{
			{InputSequence: "$", Expected: "one two-three,fou▐      \n                        \n                        \n                        \n                  NORMAL"},
			{InputSequence: "cge", Expected: "one two-three▐          \n                        \n                        \n                        \n                  INSERT"},
		}
		handlertest.RunHandlerSequence(t, newVi(t), width, height, cases)
	})

	t.Run("change WORD motion", func(t *testing.T) {
		cases := []handlertest.SequenceTestCase{
			{InputSequence: "$", Expected: "one two-three,fou▐      \n                        \n                        \n                        \n                  NORMAL"},
			{InputSequence: "cgE", Expected: "on▐                     \n                        \n                        \n                        \n                  INSERT"},
		}
		handlertest.RunHandlerSequence(t, newVi(t), width, height, cases)
	})

	t.Run("yank paste word motion", func(t *testing.T) {
		cases := []handlertest.SequenceTestCase{
			{InputSequence: "$", Expected: "one two-three,fou▐      \n                        \n                        \n                        \n                  NORMAL"},
			{InputSequence: "yge", Expected: "one two-three,fou▐      \n                        \n                        \n                        \n                  NORMAL"},
			{InputSequence: "P", Expected: "one two-three,fou,four▐ \n                        \n                        \n                        \n                  NORMAL"},
		}
		handlertest.RunHandlerSequence(t, newVi(t), width, height, cases)
	})

	t.Run("yank paste WORD motion", func(t *testing.T) {
		cases := []handlertest.SequenceTestCase{
			{InputSequence: "$", Expected: "one two-three,fou▐      \n                        \n                        \n                        \n                  NORMAL"},
			{InputSequence: "ygE", Expected: "one two-three,fou▐      \n                        \n                        \n                        \n                  NORMAL"},
			{InputSequence: "P", Expected: "ree,foue two-three,four▐\n                        \n                        \n                        \n                  NORMAL"},
		}
		handlertest.RunHandlerSequence(t, newVi(t), width, height, cases)
	})
}

func TestViTextObjects(t *testing.T) {
	type viTextObjectCase struct {
		name          string
		content       string
		at            *term.Coordinates
		seq           string
		wantBuffer    string
		wantMode      viMode
		wantCursor    *term.Coordinates
		wantSelection string
		wantClipboard string
	}

	run := func(t *testing.T, tc viTextObjectCase) {
		t.Helper()
		vi := setupVi(t, tc.content, 2)
		vi.Resize(80, 10)
		if tc.at != nil {
			vi.setCursorAtScroll(*tc.at)
		}
		for _, event := range tc.seq {
			_, handled := vi.Handle(term.Event{Type: term.EventKey, Ch: event})
			require.True(t, handled, "sequence %q failed on %q", tc.seq, string(event))
		}
		if tc.wantBuffer != "" || tc.content == "" {
			assert.Equal(t, tc.wantBuffer, vi.less.Buffer().String())
		}
		assert.Equal(t, tc.wantMode, vi.mode())
		if tc.wantCursor != nil {
			assert.Equal(t, *tc.wantCursor, vi.cursor.CursorAtScroll())
		}
		if tc.wantSelection != "" {
			selection, ok := vi.Selection()
			require.True(t, ok)
			assert.Equal(t, tc.wantSelection, selection)
		}
		if tc.wantClipboard != "" {
			paste, err := vi.config.clipboard.Paste(vi.config.defaultRegister)
			require.NoError(t, err)
			assert.Equal(t, tc.wantClipboard, paste.Text)
		}
	}

	for _, tc := range []viTextObjectCase{
		{
			name:       "delete inner word",
			content:    "one two three",
			seq:        "wdiw",
			wantBuffer: "one  three",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 4, Y: 0},
		},
		{
			name:       "change a word enters insert mode",
			content:    "one two three",
			seq:        "wcaw",
			wantBuffer: "one three",
			wantMode:   insertMode,
			wantCursor: &term.Coordinates{X: 4, Y: 0},
		},
		{
			name:          "yank inner quotes",
			content:       `say "hello world" now`,
			seq:           "fhyi\"",
			wantBuffer:    `say "hello world" now`,
			wantMode:      normalMode,
			wantClipboard: "hello world",
		},
		{
			name:       "delete around parens",
			content:    "call(one, two)",
			seq:        "f(dab",
			wantBuffer: "call",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 3, Y: 0},
		},
		{
			name:       "delete around parens alias",
			content:    "call(one, two)",
			seq:        "f(da)",
			wantBuffer: "call",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 3, Y: 0},
		},
		{
			name:       "delete around braces alias",
			content:    "call{one}",
			seq:        "f{daB",
			wantBuffer: "call",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 3, Y: 0},
		},
		{
			name:       "delete around angle alias",
			content:    "a <b> c",
			seq:        "f<dat",
			wantBuffer: "a  c",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 2, Y: 0},
		},
		{
			name:       "delete inner paragraph",
			content:    "one\ntwo\n\nthree\nfour\n",
			at:         &term.Coordinates{Y: 3},
			seq:        "dip",
			wantBuffer: "one\ntwo\n\n",
			wantMode:   normalMode,
		},
		{
			name:       "delete inner sentence",
			content:    "One. Two! Three?",
			at:         &term.Coordinates{X: 6},
			seq:        "dis",
			wantBuffer: "One.  Three?",
			wantMode:   normalMode,
		},
		{
			name:          "visual inner word",
			content:       "one two three",
			seq:           "vwiw",
			wantBuffer:    "one two three",
			wantMode:      visualMode,
			wantSelection: "two",
		},
		{
			name:          "visual invalid text object key keeps prior selection and stays visual",
			content:       "one two three",
			seq:           "vwiq",
			wantBuffer:    "one two three",
			wantMode:      visualMode,
			wantSelection: "one t",
		},
		{
			name:       "delete invalid text object exits operator mode without mutating",
			content:    "one two",
			seq:        "diq",
			wantBuffer: "one two",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 0, Y: 0},
		},
		{
			name:       "operator can retarget from inner to around",
			content:    "one two",
			seq:        "daiw",
			wantBuffer: " two",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 0, Y: 0},
		},
		{
			name:       "operator can retarget from around to inner",
			content:    "one two",
			seq:        "diiw",
			wantBuffer: " two",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 0, Y: 0},
		},
		{
			name:       "change inner empty quote enters insert mode without mutation",
			content:    `say "" now`,
			seq:        `f"ci"`,
			wantBuffer: `say "" now`,
			wantMode:   insertMode,
			wantCursor: &term.Coordinates{X: 5, Y: 0},
		},
		{
			name:       "change around empty quote removes delimiters and enters insert",
			content:    `say "" now`,
			seq:        `f"ca"`,
			wantBuffer: "say  now",
			wantMode:   insertMode,
			wantCursor: &term.Coordinates{X: 4, Y: 0},
		},
		{
			name:       "change inner empty block enters insert without mutation",
			content:    "call()",
			seq:        "f(cib",
			wantBuffer: "call()",
			wantMode:   insertMode,
			wantCursor: &term.Coordinates{X: 5, Y: 0},
		},
		{
			name:       "change around empty block removes delimiters and enters insert",
			content:    "call()",
			seq:        "f(cab",
			wantBuffer: "call",
			wantMode:   insertMode,
			wantCursor: &term.Coordinates{X: 4, Y: 0},
		},
		{
			name:       "delete inner word from end of line trailing spaces",
			content:    "one two  ",
			at:         &term.Coordinates{X: len("one two  ")},
			seq:        "diw",
			wantBuffer: "one   ",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 4, Y: 0},
		},
		{
			name:       "delete around last word removes leading whitespace",
			content:    "one two",
			at:         &term.Coordinates{X: len("one two")},
			seq:        "daw",
			wantBuffer: "one",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 2, Y: 0},
		},
		{
			name:       "count delete two around words",
			content:    "one two three",
			seq:        "2daw",
			wantBuffer: "three",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 0, Y: 0},
		},
		{
			name:       "count delete two inner words",
			content:    "one two three",
			seq:        "2diw",
			wantBuffer: " three",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 0, Y: 0},
		},
		{
			name:       "count change two inner words enters insert",
			content:    "one two three",
			seq:        "2ciw",
			wantBuffer: " three",
			wantMode:   insertMode,
			wantCursor: &term.Coordinates{X: 0, Y: 0},
		},
		{
			name:          "count yank two inner words",
			content:       "one two three",
			seq:           "2yiw",
			wantBuffer:    "one two three",
			wantMode:      normalMode,
			wantClipboard: "one two",
		},
		{
			name:          "yank a word stores standard selection metadata",
			content:       "one two",
			seq:           "yaw",
			wantBuffer:    "one two",
			wantMode:      normalMode,
			wantClipboard: "one ",
		},
		{
			name:       "count delete three around words",
			content:    "one two three four",
			seq:        "3daw",
			wantBuffer: "four",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 0, Y: 0},
		},
		{
			name:          "count yank three inner WORDs",
			content:       "one two-three four five",
			seq:           "3yiW",
			wantBuffer:    "one two-three four five",
			wantMode:      normalMode,
			wantClipboard: "one two-three four",
		},
		{
			name:       "count change two around words from leading whitespace",
			content:    "  one two three",
			at:         &term.Coordinates{X: 0, Y: 0},
			seq:        "2caw",
			wantBuffer: "  three",
			wantMode:   insertMode,
			wantCursor: &term.Coordinates{X: 2, Y: 0},
		},
		{
			name:       "count delete two inner words from end of line",
			content:    "one two three",
			at:         &term.Coordinates{X: len("one two three"), Y: 0},
			seq:        "2diw",
			wantBuffer: "one two ",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 7, Y: 0},
		},
		{
			name:          "visual around word from end of line",
			content:       "one two",
			at:            &term.Coordinates{X: len("one two"), Y: 0},
			seq:           "vaw",
			wantBuffer:    "one two",
			wantMode:      visualMode,
			wantSelection: " two",
		},
		{
			name:       "delete multiline block",
			content:    "call(\none,\ntwo\n)",
			at:         &term.Coordinates{X: 2, Y: 1},
			seq:        "dab",
			wantBuffer: "call",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 3, Y: 0},
		},
		{
			name:       "inner quote does not cross lines and invalid sequence is harmless",
			content:    "say \"hello\nworld\" now",
			seq:        "f\"di\"",
			wantBuffer: "say \"hello\nworld\" now",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 4, Y: 0},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestViTextObjectYankMetadata(t *testing.T) {
	vi := setupVi(t, "one two", 2)
	vi.Resize(40, 5)
	for _, event := range "yaw" {
		_, handled := vi.Handle(term.Event{Type: term.EventKey, Ch: event})
		require.True(t, handled)
	}
	paste, err := vi.config.clipboard.Paste(vi.config.defaultRegister)
	require.NoError(t, err)
	assert.Equal(t, "one ", paste.Text)
	assert.Equal(t, text.StandardSelection, paste.Metadata)
}

func TestViTextObjectSnapshots(t *testing.T) {
	cases := []handlertest.SequenceTestCase{
		{
			InputSequence: "vwiwd",
			Expected: `one ▐three          
call()              
                    
                    
                    
                    
                    
                    
                    
              NORMAL`,
		},
		{
			InputSequence: "j0f(cab",
			Expected: `one two three       
call▐               
                    
                    
                    
                    
                    
                    
                    
              INSERT`,
		},
	}

	newVi := func(t *testing.T) tui.Handler {
		return setupViIntegration(t, "one two three\ncall()", 2)
	}
	handlertest.RunHandlerIsolated(t, newVi, 20, 10, cases)
}

func TestViCursorIsolated(t *testing.T) {
	cases := []handlertest.SequenceTestCase{
		{"jjddp",
			`                    
/*                  
 * diff buffers.    
▐* Check if the curr
 */                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              NORMAL`},
		// insert block one rune (for now until repeater captures all insert)
		{"`jjjIh<",
			`h                   
h/*                 
h * Check if the cur
h▐* diff buffers.   
 */                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              NORMAL`},
		{"/C>/>",
			`                    
/*                  
 * ▐heck if the curr
 * diff buffers.    
 */                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              NORMAL`},
		// TODO check yank paste after last line
		// the only thing from integration tests is that there's no
		// unix View that trims last EOL, this must in turn translatre in
		// some internal difference which renders this test failure
		/*{"Gyyp",
					`    curtab->tp_diff_
		    diff_redraw(TRUE
		    }
		  }
		  }
		  else
		  diff_buf_add(win->
		}
		▐
		              NORMAL`}, */
	}

	newVi := func(t *testing.T) tui.Handler {
		return setupViIntegration(t, snippet, 2)
	}
	handlertest.TestHandlerIsolated(t, newVi, 20, 10, cases)
}

func TestIntegrationScrollEvent(t *testing.T) {
	tsuite := []struct {
		desc      string
		cursorPos term.Coordinates
		ev        term.Event
	}{
		{"move to matching rune", term.Coordinates{Y: 7}, term.Event{Type: term.EventKey, Ch: '%'}},
		{"MoveEndLine", term.Coordinates{Y: 2}, term.Event{Type: term.EventKey, Ch: '$'}},
		{"MoveRightStartWord", term.Coordinates{X: 4, Y: 9}, term.Event{Type: term.EventKey, Ch: 'w'}},
		{"MoveLeftStartWord", term.Coordinates{X: 8, Y: 9}, term.Event{Type: term.EventKey, Ch: 'b'}},
		{"MoveLeftStartWordGroup", term.Coordinates{X: 8, Y: 9}, term.Event{Type: term.EventKey, Ch: 'B'}},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.desc, func(t *testing.T) {
			vi := setupVi(t, snippet, 4)
			vi.setCursorAtScroll(tcase.cursorPos)
			vi.Resize(4, 4)

			var called int
			vi.less.Scroll().Subscribe(component.FuncScrollSubscriber(func(from, to term.Coordinates) {
				called++
			}))

			_, ok := vi.Handle(tcase.ev)
			assert.True(t, ok)
			assert.Equal(t, 1, called)
		})
	}
}

func TestIntegrationMoveWordSpecialChars(t *testing.T) {
	t.Run("navigating special chars MoveRightEndWord and MoveLeftStartWord", func(t *testing.T) {
		const snippetSpecialChars = `aaa.aaa,aaa:aaa;aaa aaa)aaa"aaa'aaa(aaa{aaa}aaa[aaa` +
			`]aaa	aaa\aaa/aaa+aaa_aaa@aaa#aaa=aaa<aaa>aaa!aaa?aaa|` +
			`aaa^aaa&aaa*aaa%aaa.aaa-`

		vi := setupVi(t, snippetSpecialChars, 2)
		prevCoords := term.Coordinates{X: 0, Y: 0}
		vi.setCursorAtScroll(prevCoords)
		vi.Resize(200, 30)
		jumps := []rune{
			'a', '.', 'a', ',', 'a', ':', 'a', ';', 'a', 'a', ')', 'a', '"',
			'a', '\'', 'a', '(', 'a', '{', 'a', '}', 'a', '[', 'a',
			']', 'a', 'a', '\\', 'a', '/', 'a', '+', 'a', 'a', '@', 'a', '#', 'a',
			'=', 'a', '<', 'a', '>', 'a', '!', 'a', '?', 'a', '|',
			'a', '^', 'a', '&', 'a', '*', 'a', '%', 'a', '.', 'a', '-',
		}

		// forward
		for i := range jumps {
			_, ok := vi.Handle(term.Event{Type: term.EventKey, Ch: 'e'})
			require.True(t, ok)
			cell, ok := vi.cursor.Cell()
			require.True(t, ok)
			require.Equal(t, jumps[i], cell.Ch, "(forward) expected '%c', got '%c'", jumps[i], cell.Ch)
		}

		// backwards
		for i := len(jumps) - 1; i >= 1; i-- {
			_, ok := vi.Handle(term.Event{Type: term.EventKey, Ch: 'b'})
			require.True(t, ok)
			cell, ok := vi.cursor.Cell()
			require.True(t, ok)
			require.Equal(t, jumps[i-1], cell.Ch, "(backwards) expected '%c', got '%c'", jumps[i-1], cell.Ch)
		}
	})
	t.Run("navigating new lines MoveRightEndWord and MoveLeftStartWord", func(t *testing.T) {
		t.Skip("Broken and to be fixed by OX-365")

		const snippetNewLines = `ab#cde
fghi.j
$klmno
pqr@st`

		vi := setupVi(t, snippetNewLines, 2)
		prevCoords := term.Coordinates{X: 0, Y: 0}
		vi.setCursorAtScroll(prevCoords)
		vi.Resize(10, 10)

		jumps := []rune{
			'b', '#', 'e',
			'f', 'i', '.', 'j',
			'$', 'o',
			'p', 'r', '@', 't', // FIXME: Instead of t gets p, to be fixed in OX-365
		}

		// forward
		for i := range jumps {
			_, ok := vi.Handle(term.Event{Type: term.EventKey, Ch: 'e'})
			require.True(t, ok)
			cell, ok := vi.cursor.Cell()
			require.True(t, ok)
			require.Equal(t, jumps[i], cell.Ch, "(forward) expected '%c', got '%c'", jumps[i], cell.Ch)
		}

		revJumps := []rune{
			's', '@', 'p',
			'o', '$',
			'j', '.', 'f',
			'e', 'c', '#', 'a',
		}

		// backwards
		for i := range revJumps {
			_, ok := vi.Handle(term.Event{Type: term.EventKey, Ch: 'b'})
			require.True(t, ok)
			cell, ok := vi.cursor.Cell()
			require.True(t, ok)
			require.Equal(t, revJumps[i], cell.Ch, "(backwards) [index %v] expected '%c', got '%c'", i, revJumps[i], cell.Ch)
		}
	})
}

func TestIntegrationNewFile(t *testing.T) {
	vi := setupVi(t, "", 2)
	vi.Resize(4, 4)
	for _, ch := range "ihello\nworld" {
		vi.Handle(term.Event{Type: term.EventKey, Ch: ch})
	}
	assert.Equal(t, "hello\nworld", vi.less.Buffer().String())
}

func TestExitInsertMode(t *testing.T) {
	t.Run("escape and control-c exit insert mode into normal", func(t *testing.T) {
		vi := setupVi(t, "aaaa\nbbbb\ncccc\ndddd", 2)
		vi.Resize(4, 4)

		vi.setInsertMode()
		assert.Equal(t, vi.mode(), insertMode)

		vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyEsc})
		assert.Equal(t, vi.mode(), normalMode)

		vi.setInsertMode()
		assert.Equal(t, vi.mode(), insertMode)

		vi.Handle(term.Event{Type: term.EventKey, Ch: 'c', Mod: term.ModCtrl})
		assert.Equal(t, vi.mode(), normalMode)
	})
}

func TestNormalModeIMovesToFirstNonBlank(t *testing.T) {
	vi := setupVi(t, "    abc", 2)
	vi.Resize(10, 10)

	require.True(t, vi.setCursorAtScroll(term.Coordinates{X: 6, Y: 0}))

	exit, handled := vi.Handle(term.Event{Type: term.EventKey, Ch: 'I'})
	require.False(t, exit)
	require.True(t, handled)
	assert.Equal(t, insertMode, vi.mode())

	scroll := vi.cursor.ScrollCoordinates(vi.cursor.Coordinates())
	assert.Equal(t, term.Coordinates{X: 4, Y: 0}, scroll)

	exit, handled = vi.Handle(term.Event{Type: term.EventKey, Ch: 'X'})
	require.False(t, exit)
	require.True(t, handled)
	assert.Equal(t, "    Xabc", vi.less.Buffer().String())
}

func TestInsertModeCtrlShortcuts(t *testing.T) {
	type testCase struct {
		name       string
		content    string
		cursor     term.Coordinates
		key        rune
		want       string
		wantCursor term.Coordinates
	}

	suite := []testCase{
		{
			name:       "ctrl+h deletes character before cursor",
			content:    "abc",
			cursor:     term.Coordinates{X: 3, Y: 0},
			key:        'h',
			want:       "ab",
			wantCursor: term.Coordinates{X: 2, Y: 0},
		},
		{
			name:       "ctrl+w deletes previous word",
			content:    "one two",
			cursor:     term.Coordinates{X: 7, Y: 0},
			key:        'w',
			want:       "one ",
			wantCursor: term.Coordinates{X: 4, Y: 0},
		},
		{
			name:       "ctrl+j inserts newline at cursor",
			content:    "ab",
			cursor:     term.Coordinates{X: 1, Y: 0},
			key:        'j',
			want:       "a\nb",
			wantCursor: term.Coordinates{X: 0, Y: 1},
		},
		{
			name:       "ctrl+t indents current line",
			content:    "abc",
			cursor:     term.Coordinates{X: 1, Y: 0},
			key:        't',
			want:       "\tabc",
			wantCursor: term.Coordinates{X: 2, Y: 0},
		},
		{
			name:       "ctrl+d deindents current line",
			content:    "\tabc",
			cursor:     term.Coordinates{X: 1, Y: 0},
			key:        'd',
			want:       "abc",
			wantCursor: term.Coordinates{X: 0, Y: 0},
		},
	}

	for _, tc := range suite {
		t.Run(tc.name, func(t *testing.T) {
			vi := setupVi(t, tc.content, 2)
			vi.Resize(10, 10)

			require.True(t, vi.setCursorAtScroll(tc.cursor))
			vi.setInsertMode()

			exit, handled := vi.Handle(term.Event{Type: term.EventKey, Ch: tc.key, Mod: term.ModCtrl})
			require.False(t, exit)
			require.True(t, handled)
			assert.Equal(t, insertMode, vi.mode())
			assert.Equal(t, tc.want, vi.less.Buffer().String())

			scroll := vi.cursor.ScrollCoordinates(vi.cursor.Coordinates())
			assert.Equal(t, tc.wantCursor, scroll)
		})
	}
}

func TestExitVisualMode(t *testing.T) {
	t.Run("escape and control-c exit visual mode into normal", func(t *testing.T) {
		vi := setupVi(t, "aaaa\nbbbb\ncccc\ndddd", 2)
		vi.Resize(4, 4)

		vi.setVisualMode()
		assert.Equal(t, vi.mode(), visualMode)

		vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyEsc})
		assert.Equal(t, vi.mode(), normalMode)

		vi.setVisualMode()
		assert.Equal(t, vi.mode(), visualMode)

		vi.Handle(term.Event{Type: term.EventKey, Ch: 'c', Mod: term.ModCtrl})
		assert.Equal(t, vi.mode(), normalMode)
	})
}

func TestViCountChangeToVisualMode(t *testing.T) {
	codeSnippet := "abcdefghij\n1234567"
	// In a window width of 4 without wrapping::
	//
	//   abcd (efghij hidden right)
	//   1234 (567 hidden right)
	//
	// In a window width of 4 with wrapping::
	//
	//   abcd
	//   efgh
	//   ij
	//   1234
	//   567

	suite := []struct {
		name                   string
		scrollWidth            int
		cursorAt               term.Coordinates
		countDigits            []rune
		selection              string
		expectScrollCoords     term.Coordinates
		expectScrollCoordsWrap term.Coordinates
		expectWindowCoords     term.Coordinates
		expectWindowCoordsWrap term.Coordinates
	}{
		{
			name:                   "1v unitary selects only current char",
			scrollWidth:            4,
			cursorAt:               term.Coordinates{X: 1, Y: 0}, // 'b' in snippet
			countDigits:            []rune{'1'},
			selection:              "b",
			expectScrollCoords:     term.Coordinates{X: 1, Y: 0},
			expectScrollCoordsWrap: term.Coordinates{X: 1, Y: 0},
			expectWindowCoords:     term.Coordinates{X: 1, Y: 0},
			expectWindowCoordsWrap: term.Coordinates{X: 1, Y: 0},
		},
		{
			name:                   "2v count N and change from normal to visual mode moves cursor N cells right",
			scrollWidth:            4,
			cursorAt:               term.Coordinates{X: 1, Y: 0}, // 'b' in snippet
			countDigits:            []rune{'2'},
			selection:              "bc",
			expectScrollCoords:     term.Coordinates{X: 2, Y: 0},
			expectScrollCoordsWrap: term.Coordinates{X: 2, Y: 0},
			expectWindowCoords:     term.Coordinates{X: 2, Y: 0},
			expectWindowCoordsWrap: term.Coordinates{X: 2, Y: 0},
		},
		{
			name:        "11v narrow scroll count N and change from normal to visual exceeds line end but not entire content",
			scrollWidth: 4,
			cursorAt:    term.Coordinates{X: 2, Y: 0}, // 'c' in snippet
			countDigits: []rune{'1', '5'},
			selection:   "cdefghij", // does not beyond code line
			// cursor will be at last char if it doesn't have room rightwards (last char index: 9)
			expectScrollCoords:     term.Coordinates{X: 10, Y: 0},
			expectScrollCoordsWrap: term.Coordinates{X: 10, Y: 0},
			expectWindowCoords:     term.Coordinates{X: 4, Y: 0},
			expectWindowCoordsWrap: term.Coordinates{X: 2, Y: 2},
		},
		{
			name:        "11v wide scroll count N and change from normal to visual exceeds line end but not entire content",
			scrollWidth: 100,
			cursorAt:    term.Coordinates{X: 2, Y: 0}, // 'c' in snippet
			countDigits: []rune{'1', '5'},
			selection:   "cdefghij", // does not beyond code line
			// cursor will be at last char if it doesn't have room rightwards (last char index: 9)
			expectScrollCoords:     term.Coordinates{X: 10, Y: 0},
			expectScrollCoordsWrap: term.Coordinates{X: 10, Y: 0},
			expectWindowCoords:     term.Coordinates{X: 10, Y: 0},
			expectWindowCoordsWrap: term.Coordinates{X: 10, Y: 0},
		},
		{
			name:                   "3v count N and change from normal to visual in last column wraps row below",
			scrollWidth:            4,
			cursorAt:               term.Coordinates{X: 3, Y: 0}, // 'd' in snippet
			countDigits:            []rune{'3'},
			selection:              "def",
			expectScrollCoords:     term.Coordinates{X: 5, Y: 0},
			expectScrollCoordsWrap: term.Coordinates{X: 5, Y: 0},
			expectWindowCoords:     term.Coordinates{X: 3, Y: 0},
			expectWindowCoordsWrap: term.Coordinates{X: 1, Y: 1},
		},
		{
			name:        "99999999999999999999v narrow scroll count N and change from normal to visual",
			scrollWidth: 4,
			cursorAt:    term.Coordinates{X: 3, Y: 0}, // 'd' in snippet
			countDigits: []rune{'9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9'},
			selection:   "defghij",
			// cursor will be at last char if it doesn't have room rightwards (last char index: 9)
			expectScrollCoords:     term.Coordinates{X: 10, Y: 0},
			expectScrollCoordsWrap: term.Coordinates{X: 10, Y: 0},
			expectWindowCoords:     term.Coordinates{X: 4, Y: 0},
			expectWindowCoordsWrap: term.Coordinates{X: 2, Y: 2},
		},
		{
			name:        "99999999999999999999v wide scroll count N and change from normal to visual",
			scrollWidth: 100,
			cursorAt:    term.Coordinates{X: 3, Y: 0}, // 'd' in snippet
			countDigits: []rune{'9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9'},
			selection:   "defghij",
			// cursor will be at last char if it doesn't have room rightwards (last char index: 9)
			expectScrollCoords:     term.Coordinates{X: 10, Y: 0},
			expectScrollCoordsWrap: term.Coordinates{X: 10, Y: 0},
			expectWindowCoords:     term.Coordinates{X: 10, Y: 0},
			expectWindowCoordsWrap: term.Coordinates{X: 10, Y: 0},
		},
		{
			name:                   "stay within code line",
			scrollWidth:            4,
			cursorAt:               term.Coordinates{X: 0, Y: 0}, // 'a' in snippet
			countDigits:            []rune{'1', '0'},
			selection:              "abcdefghij",
			expectScrollCoords:     term.Coordinates{X: 9, Y: 0},
			expectScrollCoordsWrap: term.Coordinates{X: 9, Y: 0},
			expectWindowCoords:     term.Coordinates{X: 3, Y: 0},
			expectWindowCoordsWrap: term.Coordinates{X: 1, Y: 2},
		},
	}

	for _, tcase := range suite {
		for _, wrap := range []bool{false, true} {
			name := tcase.name
			if wrap {
				name += " (wrap)"
			}

			t.Run(name, func(t *testing.T) {
				vi := setupVi(t, codeSnippet, 2, WithWrap(wrap))
				vi.Resize(tcase.scrollWidth, 20)
				// In order to create the wraps a Draw  must be issued so [scroll.Draw]
				// can create them, otherwise it's the same as passing WithWrap(false).
				vi.Draw(term.NoopWriter{})

				vi.cursor.MoveToScroll(tcase.cursorAt)

				for _, countDigit := range tcase.countDigits {
					vi.Handle(term.Event{Type: term.EventKey, Ch: countDigit})
				}
				vi.Handle(term.Event{Type: term.EventKey, Ch: 'v'})

				windowCoords := vi.cursor.Coordinates()
				scrollCoords := vi.cursor.ScrollCoordinates(windowCoords)

				if wrap {
					assert.Equal(t, tcase.expectScrollCoordsWrap,
						scrollCoords, "wrong scroll coords (wrap)")
					assert.Equal(t, tcase.expectWindowCoordsWrap,
						windowCoords, "wrong window coords (wrap)")
				} else {
					assert.Equal(t, tcase.expectScrollCoords,
						scrollCoords, "wrong scroll coords")
					assert.Equal(t, tcase.expectWindowCoords,
						windowCoords, "wrong window coords")
				}
			})
		}
	}
}

func TestViCountChangeToLineVisualMode(t *testing.T) {
	codeSnippet := "abc\n123\ndef\n456\nghij\n7891"
	suite := []struct {
		name        string
		scrollWidth int
		cursorAt    term.Coordinates
		countDigits []rune
		selection   string
	}{
		{
			name:        "1V unitary selects current line",
			scrollWidth: 2,
			cursorAt:    term.Coordinates{X: 1, Y: 0}, // 'b' in snippet
			countDigits: []rune{'1'},
			selection:   "abc\n",
		},
		{
			name:        "2V unitary selects current line and line below",
			scrollWidth: 2,
			cursorAt:    term.Coordinates{X: 1, Y: 0}, // 'b' in snippet
			countDigits: []rune{'2'},
			selection:   "abc\n123\n",
		},
		{
			name:        "999V content overflow",
			scrollWidth: 2,
			cursorAt:    term.Coordinates{X: 1, Y: 3}, // '5' in snippet
			countDigits: []rune{'9', '9', '9'},
			selection:   "456\nghij\n7891\n",
		},
	}

	for _, tcase := range suite {
		for _, wrap := range []bool{true, false} {
			name := tcase.name
			if wrap {
				name += " (wrap)"
			}

			t.Run(name, func(t *testing.T) {
				vi := setupVi(t, codeSnippet, 2, WithWrap(wrap))
				vi.Resize(tcase.scrollWidth, 4)
				vi.cursor.MoveToScroll(tcase.cursorAt)

				for _, countDigit := range tcase.countDigits {
					vi.Handle(term.Event{Type: term.EventKey, Ch: countDigit})
				}
				vi.Handle(term.Event{Type: term.EventKey, Ch: 'V'})
				assert.Equal(t, tcase.selection, vi.cursor.Selection())
			})
		}
	}
}

func TestViJoinNoSpace(t *testing.T) {
	newVi := func(t *testing.T, content string, opts ...Option) *viHandlerImpl {
		t.Helper()
		vi := setupVi(t, content, 2, opts...)
		vi.Resize(40, 10)
		vi.Draw(term.NoopWriter{})
		return vi
	}

	runEvents := func(t *testing.T, vi *viHandlerImpl, seq string) {
		t.Helper()
		for _, ch := range seq {
			_, handled := vi.Handle(term.Event{Type: term.EventKey, Ch: ch})
			require.True(t, handled, "sequence %q failed on %q", seq, string(ch))
		}
	}

	t.Run("joins current line with next without adding a space", func(t *testing.T) {
		vi := newVi(t, "hello\nworld\nfoo")

		runEvents(t, vi, "gJ")

		assert.Equal(t, "helloworld\nfoo", vi.less.Buffer().String())
		assert.Equal(t, term.Coordinates{X: len("hello"), Y: 0}, vi.cursor.Coordinates())
		assert.Equal(t, normalMode, vi.mode())
		assert.Equal(t, 1, vi.count)
		assert.Empty(t, vi.countDigits)
	})

	t.Run("joins blank line without inserting padding", func(t *testing.T) {
		vi := newVi(t, "hello\n\nworld")

		runEvents(t, vi, "gJ")

		assert.Equal(t, "hello\nworld", vi.less.Buffer().String())
		assert.Equal(t, term.Coordinates{X: len("hello") - 1, Y: 0}, vi.cursor.Coordinates())
		assert.Equal(t, normalMode, vi.mode())
	})

	t.Run("from last line gJ is a no-op and exits g mode", func(t *testing.T) {
		vi := newVi(t, "hello\nworld\nfoo")
		vi.cursor.MoveLastLine()
		vi.Draw(term.NoopWriter{})

		before := vi.less.Buffer().String()
		beforeCoord := vi.cursor.Coordinates()

		runEvents(t, vi, "gJ")

		assert.Equal(t, before, vi.less.Buffer().String())
		assert.Equal(t, beforeCoord, vi.cursor.Coordinates())
		assert.Equal(t, normalMode, vi.mode())
	})

	t.Run("repeated gJ can join multiple lines", func(t *testing.T) {
		vi := newVi(t, "a\nb\nc\nd")

		runEvents(t, vi, "gJgJ")

		assert.Equal(t, "abc\nd", vi.less.Buffer().String())
		assert.Equal(t, term.Coordinates{X: 2, Y: 0}, vi.cursor.Coordinates())
		assert.Equal(t, normalMode, vi.mode())
	})

	t.Run("count before gJ is reset after command", func(t *testing.T) {
		vi := newVi(t, "abc\ndef\nghi")

		runEvents(t, vi, "2gJ")

		assert.Equal(t, "abcdef\nghi", vi.less.Buffer().String())
		assert.Equal(t, term.Coordinates{X: len("abc"), Y: 0}, vi.cursor.Coordinates())
		assert.Equal(t, normalMode, vi.mode())
		assert.Equal(t, 1, vi.count)
		assert.Empty(t, vi.countDigits)
	})

	t.Run("unsupported g sequence returns to normal mode", func(t *testing.T) {
		vi := newVi(t, "abc\ndef")

		_, handled := vi.Handle(term.Event{Type: term.EventKey, Ch: 'g'})
		require.True(t, handled)
		assert.Equal(t, gMode, vi.mode())

		_, handled = vi.Handle(term.Event{Type: term.EventKey, Ch: 'x'})
		assert.False(t, handled)
		assert.Equal(t, normalMode, vi.mode())
		assert.Equal(t, "abc\ndef", vi.less.Buffer().String())
	})
}

func TestVigg(t *testing.T) {
	code := `11111111111
222222
333333333333

 5
6
7777777777777777777777777777777777777777777777777777777777777777777777777777777777777777777
   88888888
9999
11111111
  222222222222222
3333333
   4444444444444444
`
	codeLong := code
	for range 200 {
		codeLong += code
	}

	suite := []struct {
		name         string
		fileText     string
		narrowWrap   bool // scroll width of 4 with wrapping enaled
		moveCursorFn func(*viHandlerImpl)
		events       string
		expectCoord  term.Coordinates
	}{
		{
			name:        "gg from 0,0",
			fileText:    code,
			narrowWrap:  true,
			events:      "gg",
			expectCoord: term.Coordinates{X: 0, Y: 0},
		},
		{
			name:         "gg from 1,0",
			fileText:     code,
			narrowWrap:   true,
			moveCursorFn: func(vi *viHandlerImpl) { vi.cursor.MoveRight() },
			events:       "gg",
			expectCoord:  term.Coordinates{X: 1, Y: 0},
		},
		{
			name:       "6gg from start of wrapped line",
			fileText:   code,
			narrowWrap: true,
			moveCursorFn: func(vi *viHandlerImpl) {
				vi.cursor.MoveDown()
				vi.cursor.MoveRightColumns(2)
			},
			events:      "6gg",
			expectCoord: term.Coordinates{X: 0, Y: 8}, // lonely 6 in `code`
		},
		{
			name:       "6gg from middle of wrapped line",
			fileText:   code,
			narrowWrap: true,
			moveCursorFn: func(vi *viHandlerImpl) {
				vi.cursor.MoveDown()
				vi.cursor.MoveRightColumns(2)
			},
			events:      "6gg",
			expectCoord: term.Coordinates{X: 0, Y: 8}, // lonely 6 in `code`
		},
		{
			name:       "6gg from end of wrapped line",
			fileText:   code,
			narrowWrap: true,
			moveCursorFn: func(vi *viHandlerImpl) {
				vi.cursor.MoveDown()
				vi.cursor.MoveEndLine()
			},
			events:      "6gg",
			expectCoord: term.Coordinates{X: 0, Y: 8}, // lonely 6 in `code`
		},
		{
			name:       "gg to same line called from wrapped lines below brings cursor to start of line",
			fileText:   code,
			narrowWrap: true,
			events:     "3gg",
			moveCursorFn: func(vi *viHandlerImpl) {
				// Move to wrapped line of 333s in `code`.
				vi.cursor.MoveDownLines(3)
				vi.cursor.MoveRightColumns(2)
			},
			expectCoord: term.Coordinates{X: 0, Y: 5}, // beginning of 333s
		},
		{
			name:     "gg from end of long file",
			fileText: codeLong,
			moveCursorFn: func(vi *viHandlerImpl) {
				vi.cursor.MoveLastLine()

			},
			events:      "gg",
			expectCoord: term.Coordinates{X: 0, Y: 0},
		},
		{
			name:     "gg from middle of long file",
			fileText: codeLong,
			moveCursorFn: func(vi *viHandlerImpl) {
				vi.cursor.MoveLastLine()
				scrollCoords := vi.cursor.ScrollCoordinates(vi.cursor.Coordinates())
				vi.cursor.MoveFirstLine()
				vi.cursor.MoveDownLines(scrollCoords.Y / 2)

			},
			events:      "gg",
			expectCoord: term.Coordinates{X: 0, Y: 0},
		},
		{
			name:        "999gg beyond limits of file",
			fileText:    code,
			events:      "999gg",
			expectCoord: term.Coordinates{X: 0, Y: 8},
		},
		{
			name:     "0g",
			fileText: code,
			moveCursorFn: func(vi *viHandlerImpl) {
				vi.cursor.MoveLastLine()
			},
			events:      "0gg",
			expectCoord: term.Coordinates{X: 0, Y: 0},
		},
		{
			name:     "00g",
			fileText: code,
			moveCursorFn: func(vi *viHandlerImpl) {
				vi.cursor.MoveLastLine()
			},
			events:      "00gg",
			expectCoord: term.Coordinates{X: 0, Y: 0},
		},
		{
			name:        "3gg in single char file",
			fileText:    "a",
			events:      "3gg",
			expectCoord: term.Coordinates{X: 0, Y: 0},
		},
		{
			name:        "3gg in empty file",
			fileText:    "",
			events:      "3gg",
			expectCoord: term.Coordinates{X: 0, Y: 0},
		},
		{
			name:        "3gg in file that's only new lines",
			fileText:    "\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n",
			events:      "3gg",
			expectCoord: term.Coordinates{X: 0, Y: 2},
		},
		{
			name:        "9999999999999999999999999999999999999999999999999999gg",
			fileText:    code,
			events:      "9999999999999999999999999999999999999999999999999999gg",
			expectCoord: term.Coordinates{X: 0, Y: 8},
		},
	}

	for _, tcase := range suite {
		t.Run(tcase.name, func(t *testing.T) {
			vi := setupVi(t, tcase.fileText, 2, WithWrap(tcase.narrowWrap))
			scrollWidth := 100
			if tcase.narrowWrap {
				scrollWidth = 4
			}
			vi.Resize(scrollWidth, 9)
			vi.Draw(term.NoopWriter{})
			if tcase.moveCursorFn != nil {
				tcase.moveCursorFn(vi)
			}
			for _, eventChar := range tcase.events {
				vi.Handle(term.Event{Type: term.EventKey, Ch: eventChar})
			}
			assert.Equal(t, tcase.expectCoord, vi.cursor.Coordinates())
		})
	}
}

func TestViggBeyondContent(t *testing.T) {
	t.Run("discrepancy between visual cursor and actual cursor", func(t *testing.T) {
		fileText := "a\nb\nc\nd"
		vi := setupVi(t, fileText, 2)
		vi.Resize(100, 30)
		vi.Draw(term.NoopWriter{})
		for _, eventChar := range "999gg" {
			vi.Handle(term.Event{Type: term.EventKey, Ch: eventChar})
		}
		assert.Equal(t, term.Coordinates{X: 0, Y: 3}, vi.cursor.Coordinates())
		vi.cursor.MoveUp()
		assert.Equal(t, term.Coordinates{X: 0, Y: 2}, vi.cursor.Coordinates())

	})
}

func TestSetCursorAtScrollBounds(t *testing.T) {
	fileText := "123\n456\n789\nd"
	vi := setupVi(t, fileText, 2)
	vi.Resize(8, 8)

	t.Run("vertical bounds", func(t *testing.T) {
		vi.setCursorAtScroll(term.Coordinates{X: 0, Y: 999})
		assert.Equal(t, term.Coordinates{X: 0, Y: 3}, vi.cursor.Coordinates())
	})
}

func TestResetCount(t *testing.T) {
	t.Run("numbers are accumulated into vi counter", func(t *testing.T) {
		var code strings.Builder
		for i := range 45 {
			code.WriteString(fmt.Sprintf("%v\n", i))
		}

		vi := setupVi(t, code.String(), 2)
		vi.Resize(5, 50)

		vi.Handle(term.Event{Type: term.EventKey, Ch: '1'})
		vi.Handle(term.Event{Type: term.EventKey, Ch: '1'})
		vi.Handle(term.Event{Type: term.EventKey, Ch: 'j'})

		assert.Equal(t, vi.cursor.Coordinates().Y, 11)
	})
	t.Run("when parsing numbers and the parsed int exceeds MaxInt count should become MaxInt and not be reset", func(t *testing.T) {
		vi := setupVi(t, "aaa\nbbb\nccc\nddd", 2)
		vi.Resize(10, 10)

		maxIntStr := strconv.Itoa(math.MaxInt)

		for _, maxIntChar := range maxIntStr {
			vi.Handle(term.Event{Type: term.EventKey, Ch: maxIntChar})
		}
		vi.Handle(term.Event{Type: term.EventKey, Ch: '9'})
		vi.Handle(term.Event{Type: term.EventKey, Ch: 'j'})

		assert.Equal(t, vi.cursor.Coordinates().Y, 3)
	})

	t.Run("single motion on vi initialized with scroll", func(t *testing.T) {
		vi := setupViWithScroll(t, "aaa\nbbb\nccc\nddd", 2)
		vi.Resize(10, 10)
		vi.Handle(term.Event{Type: term.EventKey, Ch: 'j'})
		assert.Equal(t, 1, vi.cursor.Coordinates().Y)
	})

	t.Run("huge horizontal counts clamp to line bounds", func(t *testing.T) {
		vi := setupVi(t, "abcdef", 2)
		vi.Resize(20, 5)
		vi.setCursorAtScroll(term.Coordinates{X: 3, Y: 0}) // 'd'

		maxIntStr := strconv.Itoa(math.MaxInt)
		for _, ch := range maxIntStr {
			_, handled := vi.Handle(term.Event{Type: term.EventKey, Ch: ch})
			require.True(t, handled)
		}

		_, handled := vi.Handle(term.Event{Type: term.EventKey, Ch: 'h'})
		require.True(t, handled)
		assert.Equal(t, term.Coordinates{X: 0, Y: 0}, vi.cursor.Coordinates())

		for _, ch := range maxIntStr {
			_, handled = vi.Handle(term.Event{Type: term.EventKey, Ch: ch})
			require.True(t, handled)
		}

		_, handled = vi.Handle(term.Event{Type: term.EventKey, Ch: 'l'})
		require.True(t, handled)
		assert.Equal(t, term.Coordinates{X: len("abcdef") - 1, Y: 0}, vi.cursor.Coordinates())

		vi = setupVi(t, "abcdef", 2)
		vi.Resize(20, 5)
		vi.setCursorAtScroll(term.Coordinates{X: 0, Y: 0})
		for _, ch := range maxIntStr {
			_, handled = vi.Handle(term.Event{Type: term.EventKey, Ch: ch})
			require.True(t, handled)
		}

		_, handled = vi.Handle(term.Event{Type: term.EventKey, Ch: 'l'})
		require.True(t, handled)
		assert.Equal(t, term.Coordinates{X: len("abcdef") - 1, Y: 0}, vi.cursor.Coordinates())
	})

	t.Run("huge vertical counts clamp to file bounds", func(t *testing.T) {
		vi := setupVi(t, "aaa\nbbb\nccc\nddd", 2)
		vi.Resize(20, 5)
		vi.setCursorAtScroll(term.Coordinates{X: 1, Y: 2})

		maxIntStr := strconv.Itoa(math.MaxInt)
		for _, ch := range maxIntStr {
			_, handled := vi.Handle(term.Event{Type: term.EventKey, Ch: ch})
			require.True(t, handled)
		}

		_, handled := vi.Handle(term.Event{Type: term.EventKey, Ch: 'k'})
		require.True(t, handled)
		assert.Equal(t, term.Coordinates{X: 1, Y: 0}, vi.cursor.Coordinates())

		for _, ch := range maxIntStr {
			_, handled = vi.Handle(term.Event{Type: term.EventKey, Ch: ch})
			require.True(t, handled)
		}

		_, handled = vi.Handle(term.Event{Type: term.EventKey, Ch: 'j'})
		require.True(t, handled)
		assert.Equal(t, term.Coordinates{X: 1, Y: 3}, vi.cursor.Coordinates())
	})
}

func TestNoModeHandlesNonCtrlModifiers(t *testing.T) {
	vi := setupVi(t, "a", 2)
	vi.Resize(4, 4)

	modes := []viMode{
		normalMode,
		insertMode,
		deleteMode,
		gMode,
		zMode,
		yankMode,
		visualMode,
		visualLineMode,
		visualBlockMode,
		replaceMode,
		replaceOneMode,
		searchMode,
	}
	modifiers := []term.Modifier{
		term.ModAlt, term.ModShift, term.ModMeta,
		term.ModCtrlShift, term.ModCtrlAlt, term.ModCtrlMeta,
		term.ModCtrlShiftAlt, term.ModCtrlShiftMeta, term.ModCtrlAltMeta,
		term.ModShiftMeta, term.ModAltMeta, term.ModAltShiftMeta,
		term.ModAltShift,
	}

	for _, mod := range modifiers {
		for _, mode := range modes {
			t.Run(fmt.Sprintf("handle %v in %v", mod, mode), func(t *testing.T) {
				vi.currMode = mode
				exit, handled := vi.Handle(term.Event{Type: term.EventKey, Mod: mod, Ch: 'a'})
				assert.False(t, exit)
				assert.False(t, handled)
			})
		}
	}
}

func TestCursorOutOfBounds(t *testing.T) {
	for _, wrap := range []bool{false, true} {
		for _, contentWindowOverflow := range []bool{false, true} {
			name := "cursor go beyond rows and move one up"
			if wrap {
				name += " (wrap)"
			}
			if contentWindowOverflow {
				name += " (content overflow)"
			}

			t.Run(name, func(t *testing.T) {
				vi := setupVi(t, "aaaa\nbbbb\ncccc\ndddd\neeee", 2, WithWrap(wrap))

				windowWidth := 2
				if contentWindowOverflow {
					vi.Resize(windowWidth, 3)
				} else {
					vi.Resize(windowWidth, 10)
				}

				beyondRowsCoords := term.Coordinates{Y: 99}
				ok := vi.setCursorAtScroll(beyondRowsCoords)
				require.True(t, ok)

				scrollCoords := vi.cursor.ScrollCoordinates(vi.cursor.Coordinates())
				assert.Equal(t, term.Coordinates{X: 0, Y: 4}, scrollCoords)

				vi.Handle(term.Event{Type: term.EventKey, Ch: 'k'})
				scrollCoords = vi.cursor.ScrollCoordinates(vi.cursor.Coordinates())
				assert.Equal(t, term.Coordinates{X: 0, Y: 3}, scrollCoords,
					"the cursor is trapped at the last line")
			})
		}
	}
}

func TestMoveCursorArrowKeys(t *testing.T) {
	t.Run("arrow keys can be used to move cursor", func(t *testing.T) {
		vi := setupVi(t, "aaaa\nbbbb\ncccc\ndddd", 2)
		vi.Resize(4, 4)

		modes := []viMode{
			normalMode,
			insertMode,
			visualMode,
		}

		for _, mod := range modes {
			vi.currMode = mod

			coords, _, _ := vi.Cursor()
			require.Equal(t, 0, coords.X)
			require.Equal(t, 0, coords.Y)

			vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowRight})
			coords, _, _ = vi.Cursor()
			require.Equal(t, 1, coords.X)
			require.Equal(t, 0, coords.Y)

			vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
			coords, _, _ = vi.Cursor()
			require.Equal(t, 1, coords.X)
			require.Equal(t, 1, coords.Y)

			vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowLeft})
			coords, _, _ = vi.Cursor()
			require.Equal(t, 0, coords.X)
			require.Equal(t, 1, coords.Y)

			vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowUp})
			coords, _, _ = vi.Cursor()
			require.Equal(t, 0, coords.X)
			require.Equal(t, 0, coords.Y)
		}

	})
}

func TestSetNormalModeClearing(t *testing.T) {
	vi := setupVi(t, "aaaa\nbbbb\ncccc\ndddd", 2)
	vi.Resize(4, 4)

	vi.Handle(term.Event{Type: term.EventKey, Ch: '/'})
	assert.Equal(t, searchMode, vi.mode())

	vi.setNormalMode()
	assert.Equal(t, normalMode, vi.mode())

	assert.Equal(t, thandler.LessNormalMode, vi.less.Mode())
}

func TestViRegisters(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	type registerCase struct {
		name              string
		content           string
		seq               string
		externalClipboard *clipboard.Data
		wantBuffer        *string
		wantExternal      *string
		wantRegisters     map[rune]string
	}

	run := func(t *testing.T, vi *viHandlerImpl, seq string) {
		t.Helper()
		for _, event := range seq {
			ev := term.Event{Type: term.EventKey, Ch: event}
			switch event {
			case '#':
				ev = term.Event{Type: term.EventKey, Key: term.KeyEsc}
			case '>':
				ev = term.Event{Type: term.EventKey, Key: term.KeyEnter}
			}
			_, handled := vi.Handle(ev)
			require.True(t, handled, "sequence %q failed on %q", seq, string(event))
		}
	}

	for _, tc := range []registerCase{
		{
			name:    "default yank populates unnamed and last-yank registers",
			content: "one\ntwo\n",
			seq:     "yy",
			wantRegisters: map[rune]string{
				unnamedRegister:  "one\n",
				lastYankRegister: "one\n",
			},
		},
		{
			name:    "named yank populates named unnamed and last-yank registers",
			content: "one\ntwo\n",
			seq:     "\"ayy",
			wantRegisters: map[rune]string{
				'a':              "one\n",
				unnamedRegister:  "one\n",
				lastYankRegister: "one\n",
			},
		},
		{
			name:       "named register paste uses named text and preserves unnamed paste source",
			content:    "one\ntwo\n",
			seq:        "\"ayyjyy\"ap",
			wantBuffer: strPtr("one\ntwo\none\n"),
			wantRegisters: map[rune]string{
				'a':              "one\n",
				unnamedRegister:  "two\n",
				lastYankRegister: "two\n",
			},
		},
		{
			name:       "named delete populates named and unnamed but does not replace last-yank",
			content:    "one\ntwo\nthree\n",
			seq:        "yyj\"bdd",
			wantBuffer: strPtr("one\nthree\n"),
			wantRegisters: map[rune]string{
				'b':              "two\n",
				unnamedRegister:  "two\n",
				lastYankRegister: "one\n",
			},
		},
		{
			name:       "deletes into named register and pastes it later",
			content:    "one\ntwo\nthree\n",
			seq:        "\"add\"ap",
			wantBuffer: strPtr("two\none\nthree\n"),
			wantRegisters: map[rune]string{
				'a':             "one\n",
				unnamedRegister: "one\n",
			},
		},
		{
			name:       "black-hole operator delete preserves unnamed and last-yank registers",
			content:    "one\ntwo\nthree\n",
			seq:        "yyj\"_ddp",
			wantBuffer: strPtr("one\nthree\none\n"),
			wantRegisters: map[rune]string{
				unnamedRegister:   "one\n",
				lastYankRegister:  "one\n",
				blackHoleRegister: "",
			},
		},
		{
			name:       "last-yank register survives delete and can be pasted explicitly",
			content:    "one\ntwo\nthree\n",
			seq:        "yyjdd\"0p",
			wantBuffer: strPtr("one\nthree\none\n"),
			wantRegisters: map[rune]string{
				unnamedRegister:  "two\n",
				lastYankRegister: "one\n",
			},
		},
		{
			name:    "visual named yank populates named unnamed and last-yank registers",
			content: "abcdef\n",
			seq:     "vll\"ay",
			wantRegisters: map[rune]string{
				'a':              "abc",
				unnamedRegister:  "abc",
				lastYankRegister: "abc",
			},
		},
		{
			name:       "visual named delete populates named unnamed and small-delete registers",
			content:    "abcdef\n",
			seq:        "vll\"ad",
			wantBuffer: strPtr("def\n"),
			wantRegisters: map[rune]string{
				'a':             "abc",
				unnamedRegister: "abc",
				'-':             "abc",
			},
		},
		{
			name:    "search register stores last slash search",
			content: "one\ntwo\n",
			seq:     "/two>",
			wantRegisters: map[rune]string{
				'/': "two",
			},
		},
		{
			name:    "dot register stores last inserted text",
			content: "one\n",
			seq:     "iXYZ#",
			wantRegisters: map[rune]string{
				'.': "XYZ",
			},
		},
		{
			name:       "small-delete register can be pasted explicitly",
			content:    "abcdef\n",
			seq:        "vllx\"-P",
			wantBuffer: strPtr("abcdef\n"),
			wantRegisters: map[rune]string{
				'-':             "abc",
				unnamedRegister: "abc",
			},
		},
		{
			name:              "clipboard register pastes from configured clipboard",
			content:           "one\n",
			externalClipboard: &clipboard.Data{Text: "clip\n", Metadata: text.LineSelection},
			seq:               "\"+p",
			wantBuffer:        strPtr("one\nclip\n"),
		},
		{
			name:              "clipboard register yanks to configured clipboard",
			content:           "one\ntwo\n",
			externalClipboard: &clipboard.Data{Text: "clip\n", Metadata: text.LineSelection},
			seq:               "\"+yy",
			wantExternal:      strPtr("one\n"),
			wantRegisters: map[rune]string{
				unnamedRegister:  "one\n",
				lastYankRegister: "one\n",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clip := new(mockClip)
			if tc.externalClipboard != nil {
				clip.data = *tc.externalClipboard
			}

			vi := setupVi(t, tc.content, 2, WithClipboard(registerset.New(clip)))
			vi.Resize(80, 24)
			run(t, vi, tc.seq)

			if tc.wantBuffer != nil {
				assert.Equal(t, *tc.wantBuffer, vi.less.Buffer().String())
			}
			if tc.wantExternal != nil {
				assert.Equal(t, *tc.wantExternal, clip.data.Text)
			}
			for name, want := range tc.wantRegisters {
				data, err := vi.readRegister(name)
				require.NoError(t, err)
				assert.Equalf(t, want, data.Text, "register %q", string(name))
			}
		})
	}
}

func TestViParagraphMotions(t *testing.T) {
	type paragraphCase struct {
		name          string
		content       string
		at            *term.Coordinates
		seq           string
		wantBuffer    string
		wantMode      viMode
		wantCursor    *term.Coordinates
		wantClipboard string
	}

	run := func(t *testing.T, tc paragraphCase) {
		t.Helper()
		vi := setupVi(t, tc.content, 2)
		vi.Resize(80, 24)
		if tc.at != nil {
			vi.setCursorAtScroll(*tc.at)
		}
		for _, event := range tc.seq {
			_, handled := vi.Handle(term.Event{Type: term.EventKey, Ch: event})
			require.True(t, handled, "sequence %q failed on %q", tc.seq, string(event))
		}
		if tc.wantBuffer != "" || tc.content == "" {
			assert.Equal(t, tc.wantBuffer, vi.less.Buffer().String())
		}
		assert.Equal(t, tc.wantMode, vi.mode())
		if tc.wantCursor != nil {
			assert.Equal(t, *tc.wantCursor, vi.cursor.CursorAtScroll())
		}
		if tc.wantClipboard != "" {
			paste, err := vi.config.clipboard.Paste(vi.config.defaultRegister)
			require.NoError(t, err)
			assert.Equal(t, tc.wantClipboard, paste.Text)
		}
	}

	content := "aaa\nbbb\n\nccc\nddd\neee\n\nfff\nggg"
	// Line layout:
	//   0: aaa
	//   1: bbb
	//   2: (empty)
	//   3: ccc
	//   4: ddd
	//   5: eee
	//   6: (empty)
	//   7: fff
	//   8: ggg

	for _, tc := range []paragraphCase{
		// --- basic } motion ---
		{
			name:       "} from first line moves to first blank line",
			content:    content,
			seq:        "}",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 0, Y: 2},
		},
		{
			name:       "} from blank line skips to next blank line",
			content:    content,
			at:         &term.Coordinates{Y: 2},
			seq:        "}",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 0, Y: 6},
		},
		{
			name:       "} from last paragraph goes to last line",
			content:    content,
			at:         &term.Coordinates{Y: 7},
			seq:        "}",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 0, Y: 8},
		},
		{
			name:       "} from last line stays",
			content:    content,
			at:         &term.Coordinates{Y: 8},
			seq:        "}",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 0, Y: 8},
		},
		// --- basic { motion ---
		{
			name:       "{ from last line moves to last blank line",
			content:    content,
			at:         &term.Coordinates{Y: 8},
			seq:        "{",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 0, Y: 6},
		},
		{
			name:       "{ from blank line skips to prev blank line",
			content:    content,
			at:         &term.Coordinates{Y: 6},
			seq:        "{",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 0, Y: 2},
		},
		{
			name:       "{ from first paragraph goes to first line",
			content:    content,
			at:         &term.Coordinates{Y: 1},
			seq:        "{",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 0, Y: 0},
		},
		{
			name:       "{ from first line stays",
			content:    content,
			seq:        "{",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 0, Y: 0},
		},
		// --- count support ---
		{
			name:       "2} jumps two paragraphs forward",
			content:    content,
			seq:        "2}",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 0, Y: 6},
		},
		{
			name:       "2{ jumps two paragraphs backward",
			content:    content,
			at:         &term.Coordinates{Y: 8},
			seq:        "2{",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 0, Y: 2},
		},
		// --- operator combos ---
		{
			name:       "d} deletes to next paragraph boundary",
			content:    content,
			seq:        "d}",
			wantBuffer: "ccc\nddd\neee\n\nfff\nggg",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 0, Y: 0},
		},
		{
			name:       "d{ deletes backward to paragraph boundary",
			content:    content,
			at:         &term.Coordinates{Y: 4},
			seq:        "d{",
			wantBuffer: "aaa\nbbb\neee\n\nfff\nggg",
			wantMode:   normalMode,
			wantCursor: &term.Coordinates{X: 0, Y: 2},
		},
		{
			name:          "y} yanks to next paragraph boundary",
			content:       content,
			seq:           "y}",
			wantMode:      normalMode,
			wantClipboard: "aaa\nbbb\n\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func setupVi(
	t *testing.T, text string, tabspaces int, opts ...Option,
) *viHandlerImpl {
	buf := cell.NewBuffer()
	buf.Init()
	_, err := buf.ReadFrom(strings.NewReader(text))
	require.NoError(t, err)

	config := defaultviHandlerImplConfig()
	config.tabspaces = tabspaces
	for _, o := range opts {
		o(&config)
	}

	vi := new(viHandlerImpl)
	vi.init(buf, config)

	return vi
}

func setupViWithScroll(
	t *testing.T, text string, tabspaces int, opts ...Option,
) *viHandlerImpl {
	buf := cell.NewBuffer()
	buf.Init()
	_, err := buf.ReadFrom(strings.NewReader(text))
	require.NoError(t, err)

	scroll := component.NewScroll(buf)
	scroll.SetTabspaces(tabspaces)

	vi := new(viHandlerImpl)
	vi.initWithScroll(scroll, opts...)

	return vi
}

func setupViIntegration(
	t *testing.T, copy string, tabspaces int, opts ...Option,
) tui.Handler {
	buf := cell.NewBuffer()
	buf.Init()
	_, err := buf.ReadFrom(strings.NewReader(copy))
	require.NoError(t, err)

	opts = append(opts, WithTabspaces(tabspaces))

	var mu sync.Mutex
	cfg := text.StatusBarConfig{
		Publisher: &texttest.TestEditor{},
		ScheduleNextTick: func(cb func()) bool {
			mu.Lock()
			defer mu.Unlock()
			cb()
			return true
		},
	}
	vi := New(buf, uri, opts...)
	bar := text.WithStatusBar(vi, buf, vi.less.Scroll(),
		false, false, cfg)
	vi.setStatusBar(bar)

	return handler.Sync(&mu, bar)
}

func TestPasteVisualMode(t *testing.T) {
	name := "standard select"
	for _, wrap := range []bool{false, true} {
		if wrap {
			name += " (wrap)"
		}

		t.Run(name, func(t *testing.T) {
			vi := setupVi(t, "ABC0123456789", 2)
			if wrap {
				vi.Resize(4, 10)
			} else {
				vi.Resize(10, 10)
			}

			events := "vllyvlld"
			for _, event := range events {
				vi.Handle(term.Event{Type: term.EventKey, Ch: event})
			}

			require.Equal(t, "0123456789", vi.less.Buffer().String())
			paste, err := vi.config.clipboard.Paste(vi.config.defaultRegister)
			require.NoError(t, err)
			require.Equal(t, "ABC", paste.Text)

			events = "lllvll"
			for _, event := range events {
				vi.Handle(term.Event{Type: term.EventKey, Ch: event})
			}
			require.Equal(t, "345", vi.cursor.Selection())

			vi.Handle(term.Event{Type: term.EventKey, Ch: 'p'})

			// after pasting in visual mode vim goes to normal mode again
			assert.Equal(t, normalMode, vi.mode())

			assert.Equal(t, "012ABC6789", vi.less.Buffer().String())

			cell, ok := vi.cursor.Cell()
			require.True(t, ok)
			require.Equal(t, '6', cell.Ch)
		})
	}

	name = "line select"
	for _, wrap := range []bool{false, true} {
		if wrap {
			name += " (wrap)"
		}

		t.Run(name, func(t *testing.T) {
			vi := setupVi(t, "ABC0123456\n789\n", 2)
			if wrap {
				vi.Resize(4, 10)
			} else {
				vi.Resize(10, 10)
			}

			events := "vllyvlld"
			for _, event := range events {
				vi.Handle(term.Event{Type: term.EventKey, Ch: event})
			}

			require.Equal(t, "0123456\n789\n", vi.less.Buffer().String())
			paste, err := vi.config.clipboard.Paste(vi.config.defaultRegister)
			require.NoError(t, err)
			require.Equal(t, "ABC", paste.Text)

			events = "V"
			for _, event := range events {
				vi.Handle(term.Event{Type: term.EventKey, Ch: event})
			}

			if wrap {
				// FIXME: Is "0123456\n"
				// At the moment Cursor.SelectLine does not honor wrap lines and honors
				// logical lines. This will be changed by PR #127 "Add directional vi
				// (d)elete and (y)ank  (OX-222).
				// require.Equal(t, "0123", vi.cursor.Selection())
			} else {
				require.Equal(t, "0123456\n", vi.cursor.Selection())
			}

			vi.Handle(term.Event{Type: term.EventKey, Ch: 'p'})

			// after pasting in visual mode vim goes to normal mode again
			assert.Equal(t, normalMode, vi.mode())

			if wrap {
				// FIXME: Is "ABC789\n"
				// At the moment Cursor.SelectLine does not honor wrap lines and honors
				// logical lines. This will be changed by PR #127 "Add directional vi
				// (d)elete and (y)ank  (OX-222).
				//assert.Equal(t, "ABC456\n789\n", vi.less.Buffer().String())
			} else {
				assert.Equal(t, "ABC\n789\n", vi.less.Buffer().String())
			}

			cell, ok := vi.cursor.Cell()
			require.True(t, ok)
			require.Equal(t, 'A', cell.Ch,
				"current cell char is not 'A' but '%c'", cell.Ch)
		})
	}

	name = "block select"
	for _, wrap := range []bool{false, true} {
		if wrap {
			name += " (wrap)"
		}

		t.Run(name, func(t *testing.T) {
			vi := setupVi(t, "ABC\nDEF\n0123456\n789\n", 2)
			if wrap {
				vi.Resize(4, 10)
			} else {
				vi.Resize(10, 10)
			}

			vi.Handle(term.Event{Type: term.EventKey, Ch: 'v', Mod: term.ModCtrl})
			for _, event := range "lljyVj\"_d" {
				vi.Handle(term.Event{Type: term.EventKey, Ch: event})
			}

			require.Equal(t, "0123456\n789\n", vi.less.Buffer().String())
			paste, err := vi.config.clipboard.Paste(vi.config.defaultRegister)
			require.NoError(t, err)
			require.Equal(t, "ABC\nDEF", paste.Text)

			vi.Handle(term.Event{Type: term.EventKey, Ch: 'l'})
			vi.Handle(term.Event{Type: term.EventKey, Ch: 'v', Mod: term.ModCtrl})
			vi.Handle(term.Event{Type: term.EventKey, Ch: 'j'})

			if wrap {
				require.Equal(t, "1\n8", vi.cursor.Selection())
			} else {
				require.Equal(t, "1\n8", vi.cursor.Selection())
			}

			vi.Handle(term.Event{Type: term.EventKey, Ch: 'p'})

			if wrap {
				assert.Equal(t, "0ABC23456\n7DEF9\n", vi.less.Buffer().String())
			} else {
				assert.Equal(t, "0ABC23456\n7DEF9\n", vi.less.Buffer().String())
			}

			cell, ok := vi.cursor.Cell()
			require.True(t, ok)
			require.Equal(t, 'A', cell.Ch,
				"current cell char is not 'A' but '%c'", cell.Ch)
		})
	}
}

func TestViGoPasteLeavesCursorAfterText(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		cursorAt      term.Coordinates
		clipboardText string
		clipboardMode text.SelectMode
		input         string
		wantBuffer    string
		wantPosition  term.Coordinates
		wantCell      rune
		wantCount     int
		wantCountText string
	}{
		{
			name:          "gp pastes characterwise text after cursor",
			content:       "abc\ndef\nxyz",
			cursorAt:      term.Coordinates{X: 0, Y: 0},
			clipboardText: "a",
			clipboardMode: text.StandardSelection,
			input:         "gp",
			wantBuffer:    "aabc\ndef\nxyz",
			wantPosition:  term.Coordinates{X: 2, Y: 0},
			wantCell:      'b',
			wantCount:     1,
			wantCountText: "",
		},
		{
			name:          "gP pastes characterwise text before cursor",
			content:       "abc\ndef\nxyz",
			cursorAt:      term.Coordinates{X: 1, Y: 0},
			clipboardText: "a",
			clipboardMode: text.StandardSelection,
			input:         "gP",
			wantBuffer:    "aabc\ndef\nxyz",
			wantPosition:  term.Coordinates{X: 2, Y: 0},
			wantCell:      'b',
			wantCount:     1,
			wantCountText: "",
		},
		{
			name:          "gp pastes multiline characterwise text after cursor",
			content:       "abc\ndef\nxyz",
			cursorAt:      term.Coordinates{X: 0, Y: 0},
			clipboardText: "abc",
			clipboardMode: text.StandardSelection,
			input:         "gp",
			wantBuffer:    "aabcbc\ndef\nxyz",
			wantPosition:  term.Coordinates{X: 4, Y: 0},
			wantCell:      'b',
			wantCount:     1,
			wantCountText: "",
		},
		{
			name:          "gP pastes multiline characterwise text before cursor",
			content:       "abc\ndef\nxyz",
			cursorAt:      term.Coordinates{X: 3, Y: 0},
			clipboardText: "abc",
			clipboardMode: text.StandardSelection,
			input:         "gP",
			wantBuffer:    "ababcc\ndef\nxyz",
			wantPosition:  term.Coordinates{X: 5, Y: 0},
			wantCell:      'c',
			wantCount:     1,
			wantCountText: "",
		},
		{
			name:          "gp pastes line after cursor line",
			content:       "abc\ndef\nxyz",
			cursorAt:      term.Coordinates{X: 0, Y: 1},
			clipboardText: "abc\n",
			clipboardMode: text.LineSelection,
			input:         "gp",
			wantBuffer:    "abc\ndef\nabc\nxyz",
			wantPosition:  term.Coordinates{X: 0, Y: 3},
			wantCell:      'x',
			wantCount:     1,
			wantCountText: "",
		},
		{
			name:          "gP pastes line before cursor line",
			content:       "abc\ndef\nxyz",
			cursorAt:      term.Coordinates{X: 0, Y: 1},
			clipboardText: "abc\n",
			clipboardMode: text.LineSelection,
			input:         "gP",
			wantBuffer:    "abc\nabc\ndef\nxyz",
			wantPosition:  term.Coordinates{X: 0, Y: 2},
			wantCell:      'd',
			wantCount:     1,
			wantCountText: "",
		},
		{
			name:          "gp on last line appends linewise paste after buffer end",
			content:       "abc\ndef\nxyz",
			cursorAt:      term.Coordinates{X: 0, Y: 2},
			clipboardText: "abc\n",
			clipboardMode: text.LineSelection,
			input:         "gp",
			wantBuffer:    "abc\ndef\nxyz\nabc\n",
			wantPosition:  term.Coordinates{X: 0, Y: 3},
			wantCell:      'a',
			wantCount:     1,
			wantCountText: "",
		},
		{
			name:          "gP on last line inserts linewise paste before current line",
			content:       "abc\ndef\nxyz",
			cursorAt:      term.Coordinates{X: 0, Y: 2},
			clipboardText: "abc\n",
			clipboardMode: text.LineSelection,
			input:         "gP",
			wantBuffer:    "abc\ndef\nabc\nxyz",
			wantPosition:  term.Coordinates{X: 0, Y: 3},
			wantCell:      'x',
			wantCount:     1,
			wantCountText: "",
		},
		{
			name:          "count before gp is reset after command",
			content:       "abc\ndef\nxyz",
			cursorAt:      term.Coordinates{X: 0, Y: 1},
			clipboardText: "abc\n",
			clipboardMode: text.LineSelection,
			input:         "2gp",
			wantBuffer:    "abc\ndef\nabc\nabc\nxyz",
			wantPosition:  term.Coordinates{X: 0, Y: 4},
			wantCell:      'x',
			wantCount:     1,
			wantCountText: "",
		},
	}

	for _, tcase := range tests {
		t.Run(tcase.name, func(t *testing.T) {
			vi := setupVi(t, tcase.content, 2)
			vi.Resize(20, 10)
			vi.setCursorAtScroll(tcase.cursorAt)
			err := vi.config.clipboard.Copy(vi.config.defaultRegister, clipboard.Data{
				Text:     tcase.clipboardText,
				Metadata: tcase.clipboardMode,
			})
			require.NoError(t, err)

			for _, event := range tcase.input {
				_, handled := vi.Handle(term.Event{Type: term.EventKey, Ch: event})
				require.True(t, handled, "sequence %q failed on %q", tcase.input, string(event))
			}

			assert.Equal(t, tcase.wantBuffer, vi.less.Buffer().String())
			assert.Equal(t, normalMode, vi.mode())
			assert.Equal(t, tcase.wantPosition, vi.cursorAtScroll())
			assert.Equal(t, tcase.wantCount, vi.count)
			assert.Equal(t, tcase.wantCountText, vi.countDigits)

			cell, ok := vi.cursor.Cell()
			require.True(t, ok)
			assert.Equal(t, tcase.wantCell, cell.Ch)
		})
	}
}

func TestVisualBlockInsert(t *testing.T) {
	fileContent := "aaaaaa\nbbbbbb\ncccccc\ndddddd"

	suite := []struct {
		name          string
		moveCursorFn  func(*viHandlerImpl)
		selectKeys    string // keys after Ctrl-V to define block; default "jj"
		inputSequence string
		expect        string
	}{
		{
			name:          "no backspace",
			inputSequence: "01234",
			expect:        "01234aaaaaa\n01234bbbbbb\n01234cccccc\ndddddd",
		},
		{
			name:          "backspace",
			inputSequence: "01234^xy",
			expect:        "0123xyaaaaaa\n0123xybbbbbb\n0123xycccccc\ndddddd",
		},
		{
			name:          "many backspace",
			inputSequence: "01234^^^xy",
			expect:        "01xyaaaaaa\n01xybbbbbb\n01xycccccc\ndddddd",
		},
		{
			name:          "more backspaces than characters in row",
			inputSequence: "ABC^^^^^^^",
			expect:        "aaaaaa\nbbbbbb\ncccccc\ndddddd",
		},
		{
			name:          "backspace only",
			inputSequence: "^",
			expect:        "aaaaaa\nbbbbbb\ncccccc\ndddddd",
		},
		{
			name:          "many backspace only",
			inputSequence: "^^^",
			expect:        "aaaaaa\nbbbbbb\ncccccc\ndddddd",
		},
		{
			name: "wider block",
			moveCursorFn: func(vi *viHandlerImpl) {
				// move cursor to column 2
				vi.Handle(term.Event{Type: term.EventKey, Ch: 'l'})
				vi.Handle(term.Event{Type: term.EventKey, Ch: 'l'})
			},
			selectKeys:    "lljj", // select cols 2-4, rows 0-2
			inputSequence: "XY",
			expect:        "aaXYaaaa\nbbXYbbbb\nccXYcccc\ndddddd",
		},
		{
			name: "reversed horizontal selection",
			moveCursorFn: func(vi *viHandlerImpl) {
				// start at column 3
				vi.Handle(term.Event{Type: term.EventKey, Ch: 'l'})
				vi.Handle(term.Event{Type: term.EventKey, Ch: 'l'})
				vi.Handle(term.Event{Type: term.EventKey, Ch: 'l'})
			},
			selectKeys:    "hhjj", // select leftward from col 3 to col 1, rows 0-2
			inputSequence: "XY",
			// I inserts at left edge (col 1)
			expect: "aXYaaaaa\nbXYbbbbb\ncXYccccc\ndddddd",
		},
		{
			name:          "reversed vertical selection",
			selectKeys:    "kk", // select upward: start at row 2, go to row 0
			inputSequence: "XY",
			moveCursorFn: func(vi *viHandlerImpl) {
				// start at row 2
				vi.Handle(term.Event{Type: term.EventKey, Ch: 'j'})
				vi.Handle(term.Event{Type: term.EventKey, Ch: 'j'})
			},
			expect: "XYaaaaaa\nXYbbbbbb\nXYcccccc\ndddddd",
		},
	}

	for _, tcase := range suite {
		t.Run(tcase.name, func(t *testing.T) {
			vi := setupVi(t, fileContent, 2)
			vi.Resize(20, 10)

			if tcase.moveCursorFn != nil {
				tcase.moveCursorFn(vi)
			}

			selectKeys := tcase.selectKeys
			if selectKeys == "" {
				selectKeys = "jj"
			}

			vi.Handle(term.Event{Type: term.EventKey, Ch: 'v', Mod: term.ModCtrl})
			for _, ch := range selectKeys {
				vi.Handle(term.Event{Type: term.EventKey, Ch: ch})
			}
			vi.Handle(term.Event{Type: term.EventKey, Ch: 'I'})

			for _, ch := range tcase.inputSequence {
				// following same convetions as handler.handlertest.SequenceTestCase
				if ch == '^' {
					vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyBackspace})
				} else {
					vi.Handle(term.Event{Type: term.EventKey, Ch: ch})
				}

			}

			vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyEsc})
			assert.Equal(t, tcase.expect,
				vi.less.Buffer().String())

		})
	}
}

func TestVisualBlockAppend(t *testing.T) {
	fileContent := "aaaaaa\nbbbbbb\ncccccc\ndddddd"

	suite := []struct {
		name          string
		moveCursorFn  func(*viHandlerImpl)
		selectKeys    string // keys after Ctrl-V; default "jj"
		inputSequence string
		expect        string
	}{
		{
			name:          "no backspace",
			inputSequence: "01234",
			// block is col 0 only, rows 0-2; A appends at col 1
			expect: "a01234aaaaa\nb01234bbbbb\nc01234ccccc\ndddddd",
		},
		{
			name:          "backspace",
			inputSequence: "01234^xy",
			expect:        "a0123xyaaaaa\nb0123xybbbbb\nc0123xyccccc\ndddddd",
		},
		{
			name:          "many backspace",
			inputSequence: "01234^^^xy",
			expect:        "a01xyaaaaa\nb01xybbbbb\nc01xyccccc\ndddddd",
		},
		{
			name:          "more backspaces than characters",
			inputSequence: "ABC^^^^^^^",
			// backspacing past insertion deletes existing chars at append position
			expect: "aaaaa\nbbbbb\nccccc\ndddddd",
		},
		{
			name:          "backspace only",
			inputSequence: "^",
			// single backspace from append position deletes char at col 0
			expect: "aaaaa\nbbbbb\nccccc\ndddddd",
		},
		{
			name: "wider block",
			moveCursorFn: func(vi *viHandlerImpl) {
				vi.Handle(term.Event{Type: term.EventKey, Ch: 'l'})
				vi.Handle(term.Event{Type: term.EventKey, Ch: 'l'})
			},
			selectKeys:    "lljj", // cols 2-4, rows 0-2; A appends at col 5
			inputSequence: "XY",
			expect:        "aaaaaXYa\nbbbbbXYb\ncccccXYc\ndddddd",
		},
		{
			name: "reversed horizontal selection",
			moveCursorFn: func(vi *viHandlerImpl) {
				vi.Handle(term.Event{Type: term.EventKey, Ch: 'l'})
				vi.Handle(term.Event{Type: term.EventKey, Ch: 'l'})
				vi.Handle(term.Event{Type: term.EventKey, Ch: 'l'})
			},
			selectKeys:    "hhjj", // anchor col 3, cursor col 1; block cols 1-3, A at col 4
			inputSequence: "XY",
			expect:        "aaaaXYaa\nbbbbXYbb\nccccXYcc\ndddddd",
		},
		{
			name:          "reversed vertical selection",
			selectKeys:    "kk",
			inputSequence: "XY",
			moveCursorFn: func(vi *viHandlerImpl) {
				vi.Handle(term.Event{Type: term.EventKey, Ch: 'j'})
				vi.Handle(term.Event{Type: term.EventKey, Ch: 'j'})
			},
			expect: "aXYaaaaa\nbXYbbbbb\ncXYccccc\ndddddd",
		},
	}

	for _, tcase := range suite {
		t.Run(tcase.name, func(t *testing.T) {
			vi := setupVi(t, fileContent, 2)
			vi.Resize(20, 10)

			if tcase.moveCursorFn != nil {
				tcase.moveCursorFn(vi)
			}

			selectKeys := tcase.selectKeys
			if selectKeys == "" {
				selectKeys = "jj"
			}

			vi.Handle(term.Event{Type: term.EventKey, Ch: 'v', Mod: term.ModCtrl})
			for _, ch := range selectKeys {
				vi.Handle(term.Event{Type: term.EventKey, Ch: ch})
			}
			vi.Handle(term.Event{Type: term.EventKey, Ch: 'A'})

			for _, ch := range tcase.inputSequence {
				if ch == '^' {
					vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyBackspace})
				} else {
					vi.Handle(term.Event{Type: term.EventKey, Ch: ch})
				}
			}

			vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyEsc})
			assert.Equal(t, tcase.expect,
				vi.less.Buffer().String())
		})
	}
}

func TestVisualBlockChange(t *testing.T) {
	fileContent := "aaaaaa\nbbbbbb\ncccccc\ndddddd"

	suite := []struct {
		name          string
		moveCursorFn  func(*viHandlerImpl)
		selectKeys    string // keys after Ctrl-V; default "jj"
		changeKey     rune   // 'c' or 's'; default 'c'
		inputSequence string
		expect        string
	}{
		{
			name:          "single column change",
			inputSequence: "X",
			// block is col 0, rows 0-2; delete col 0, insert "X"
			expect: "Xaaaaa\nXbbbbb\nXccccc\ndddddd",
		},
		{
			name: "wider block change",
			moveCursorFn: func(vi *viHandlerImpl) {
				vi.Handle(term.Event{Type: term.EventKey, Ch: 'l'})
				vi.Handle(term.Event{Type: term.EventKey, Ch: 'l'})
			},
			selectKeys:    "lljj", // cols 2-4, rows 0-2
			inputSequence: "XY",
			expect:        "aaXYa\nbbXYb\nccXYc\ndddddd",
		},
		{
			name:          "change with backspace",
			selectKeys:    "lljj",
			inputSequence: "ABCD^xy",
			// block is cols 0-2, rows 0-2; delete 3 cols, type ABCDxy with 1 BS
			expect: "ABCxyaaa\nABCxybbb\nABCxyccc\ndddddd",
		},
		{
			name:      "substitute single column",
			changeKey: 's',
			// block is col 0, rows 0-2; delete col 0, insert "Z"
			inputSequence: "Z",
			expect:        "Zaaaaa\nZbbbbb\nZccccc\ndddddd",
		},
		{
			name: "reversed horizontal change",
			moveCursorFn: func(vi *viHandlerImpl) {
				vi.Handle(term.Event{Type: term.EventKey, Ch: 'l'})
				vi.Handle(term.Event{Type: term.EventKey, Ch: 'l'})
				vi.Handle(term.Event{Type: term.EventKey, Ch: 'l'})
			},
			selectKeys:    "hhjj", // anchor col 3, cursor col 1; block cols 1-3
			inputSequence: "XY",
			expect:        "aXYaa\nbXYbb\ncXYcc\ndddddd",
		},
	}

	for _, tcase := range suite {
		t.Run(tcase.name, func(t *testing.T) {
			vi := setupVi(t, fileContent, 2)
			vi.Resize(20, 10)

			if tcase.moveCursorFn != nil {
				tcase.moveCursorFn(vi)
			}

			selectKeys := tcase.selectKeys
			if selectKeys == "" {
				selectKeys = "jj"
			}
			changeKey := tcase.changeKey
			if changeKey == 0 {
				changeKey = 'c'
			}

			vi.Handle(term.Event{Type: term.EventKey, Ch: 'v', Mod: term.ModCtrl})
			for _, ch := range selectKeys {
				vi.Handle(term.Event{Type: term.EventKey, Ch: ch})
			}
			vi.Handle(term.Event{Type: term.EventKey, Ch: changeKey})

			for _, ch := range tcase.inputSequence {
				if ch == '^' {
					vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyBackspace})
				} else {
					vi.Handle(term.Event{Type: term.EventKey, Ch: ch})
				}
			}

			vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyEsc})
			assert.Equal(t, tcase.expect,
				vi.less.Buffer().String())
		})
	}
}

func TestVisualBlockSwapCorner(t *testing.T) {
	type testCase struct {
		name                    string
		content                 string
		width                   int
		height                  int
		before                  []term.Event
		wantBeforeCursor        term.Coordinates
		wantBeforeAnchor        term.Coordinates
		wantBeforeCursorVisible bool
		wantBeforeAnchorVisible bool
		wantAfterCursorVisible  bool
		wantAfterAnchorVisible  bool
	}

	key := func(ch rune) term.Event {
		return term.Event{Type: term.EventKey, Ch: ch}
	}
	ctrl := func(ch rune) term.Event {
		return term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: ch}
	}

	suite := []testCase{
		{
			name:                    "different lines middle columns swaps opposite corner",
			content:                 "abcd\nefgh\nijkl",
			width:                   10,
			height:                  20,
			before:                  []term.Event{key('l'), ctrl('v'), key('j'), key('l')},
			wantBeforeCursor:        term.Coordinates{X: 2, Y: 1},
			wantBeforeAnchor:        term.Coordinates{X: 1},
			wantBeforeCursorVisible: true,
			wantBeforeAnchorVisible: true,
			wantAfterCursorVisible:  true,
			wantAfterAnchorVisible:  true,
		},
		{
			name:                    "same line block selection behaves like end swap",
			content:                 "abcdef",
			width:                   10,
			height:                  20,
			before:                  []term.Event{key('l'), ctrl('v'), key('l'), key('l')},
			wantBeforeCursor:        term.Coordinates{X: 3},
			wantBeforeAnchor:        term.Coordinates{X: 1},
			wantBeforeCursorVisible: true,
			wantBeforeAnchorVisible: true,
			wantAfterCursorVisible:  true,
			wantAfterAnchorVisible:  true,
		},
		{
			name:                    "same column block selection behaves like end swap",
			content:                 "abcd\nefgh\nijkl",
			width:                   10,
			height:                  20,
			before:                  []term.Event{key('l'), ctrl('v'), key('j'), key('j')},
			wantBeforeCursor:        term.Coordinates{X: 1, Y: 2},
			wantBeforeAnchor:        term.Coordinates{X: 1},
			wantBeforeCursorVisible: true,
			wantBeforeAnchorVisible: true,
			wantAfterCursorVisible:  true,
			wantAfterAnchorVisible:  true,
		},
		{
			name:                    "beginning of line to end of line across rows",
			content:                 "abcd\nefgh",
			width:                   10,
			height:                  20,
			before:                  []term.Event{ctrl('v'), key('j'), key('$')},
			wantBeforeCursor:        term.Coordinates{X: 4, Y: 1},
			wantBeforeAnchor:        term.Coordinates{},
			wantBeforeCursorVisible: true,
			wantBeforeAnchorVisible: true,
			wantAfterCursorVisible:  true,
			wantAfterAnchorVisible:  true,
		},
		{
			name:                    "start of file block selection",
			content:                 "abcd\nefgh\nijkl",
			width:                   10,
			height:                  20,
			before:                  []term.Event{ctrl('v'), key('j'), key('l')},
			wantBeforeCursor:        term.Coordinates{X: 1, Y: 1},
			wantBeforeAnchor:        term.Coordinates{},
			wantBeforeCursorVisible: true,
			wantBeforeAnchorVisible: true,
			wantAfterCursorVisible:  true,
			wantAfterAnchorVisible:  true,
		},
		{
			name:                    "end of file block selection",
			content:                 "abcd\nefgh\nijkl",
			width:                   10,
			height:                  20,
			before:                  []term.Event{key('G'), key('$'), ctrl('v'), key('k'), key('h')},
			wantBeforeCursor:        term.Coordinates{X: 2, Y: 1},
			wantBeforeAnchor:        term.Coordinates{X: 3, Y: 2},
			wantBeforeCursorVisible: true,
			wantBeforeAnchorVisible: true,
			wantAfterCursorVisible:  true,
			wantAfterAnchorVisible:  true,
		},
		{
			name:                    "selection start is above rendered view after swap",
			content:                 "aaaa\nbbbb\ncccc\ndddd\neeee\nffff",
			width:                   10,
			height:                  3,
			before:                  []term.Event{key('G'), key('l'), ctrl('v'), key('k'), key('k'), key('k')},
			wantBeforeCursor:        term.Coordinates{X: 1, Y: 2},
			wantBeforeAnchor:        term.Coordinates{X: 1, Y: 5},
			wantBeforeCursorVisible: true,
			wantBeforeAnchorVisible: false,
			wantAfterCursorVisible:  true,
			wantAfterAnchorVisible:  false,
		},
		{
			name:                    "selection end is below rendered view before swap and anchor becomes offscreen after swap",
			content:                 "aaaa\nbbbb\ncccc\ndddd\neeee\nffff",
			width:                   10,
			height:                  3,
			before:                  []term.Event{key('l'), ctrl('v'), key('j'), key('j'), key('j')},
			wantBeforeCursor:        term.Coordinates{X: 1, Y: 3},
			wantBeforeAnchor:        term.Coordinates{X: 1},
			wantBeforeCursorVisible: true,
			wantBeforeAnchorVisible: false,
			wantAfterCursorVisible:  true,
			wantAfterAnchorVisible:  false,
		},
		{
			name:                    "selection start is left of rendered view after horizontal scroll swap",
			content:                 "0123456789abcdef\n0123456789abcdef",
			width:                   4,
			height:                  2,
			before:                  []term.Event{key('$'), ctrl('v'), key('k'), key('k')},
			wantBeforeCursor:        term.Coordinates{X: 15},
			wantBeforeAnchor:        term.Coordinates{X: 15},
			wantBeforeCursorVisible: true,
			wantBeforeAnchorVisible: true,
			wantAfterCursorVisible:  true,
			wantAfterAnchorVisible:  true,
		},
		{
			name:                    "selection start left of rendered view in wide block after swap",
			content:                 "0123456789abcdef\n0123456789abcdef",
			width:                   4,
			height:                  2,
			before:                  []term.Event{key('$'), ctrl('v'), key('j'), key('h'), key('h')},
			wantBeforeCursor:        term.Coordinates{X: 13, Y: 1},
			wantBeforeAnchor:        term.Coordinates{X: 15},
			wantBeforeCursorVisible: true,
			wantBeforeAnchorVisible: true,
			wantAfterCursorVisible:  true,
			wantAfterAnchorVisible:  true,
		},
		{
			name:                    "selection end is right of rendered view before swap and anchor becomes offscreen after swap",
			content:                 "0123456789abcdef\n0123456789abcdef",
			width:                   4,
			height:                  2,
			before:                  []term.Event{ctrl('v'), key('j'), key('$')},
			wantBeforeCursor:        term.Coordinates{X: 16, Y: 1},
			wantBeforeAnchor:        term.Coordinates{},
			wantBeforeCursorVisible: false,
			wantBeforeAnchorVisible: false,
			wantAfterCursorVisible:  true,
			wantAfterAnchorVisible:  false,
		},
	}

	for _, tc := range suite {
		t.Run(tc.name, func(t *testing.T) {
			vi := setupVi(t, tc.content, 2, WithWrap(false))
			vi.Resize(tc.width, tc.height)
			vi.Draw(term.NoopWriter{})

			handleViEvents(vi, tc.before)

			require.Equal(t, visualBlockMode, vi.mode())

			beforeCursor := vi.cursor.CursorAtScroll()
			require.Equal(t, tc.wantBeforeCursor, beforeCursor)

			beforeAnchor, ok := vi.cursor.SelectionFrom()
			require.True(t, ok)
			require.Equal(t, tc.wantBeforeAnchor, beforeAnchor)
			assert.Equal(t, tc.wantBeforeCursorVisible, viPositionVisible(vi, beforeCursor))
			assert.Equal(t, tc.wantBeforeAnchorVisible, viPositionVisible(vi, beforeAnchor))

			selectionBefore := vi.cursor.Selection()
			require.NotEmpty(t, selectionBefore)
			bufferBefore := vi.less.Buffer().String()
			wantAfterCursor, wantAfterAnchor := swappedBlockCorners(beforeAnchor, beforeCursor)

			vi.Handle(key('O'))

			assert.Equal(t, visualBlockMode, vi.mode())
			assert.Equal(t, bufferBefore, vi.less.Buffer().String())
			assert.Equal(t, wantAfterCursor, vi.cursor.CursorAtScroll())
			assert.Equal(t, selectionBefore, vi.cursor.Selection())

			afterAnchor, ok := vi.cursor.SelectionFrom()
			require.True(t, ok)
			assert.Equal(t, wantAfterAnchor, afterAnchor)
			assert.Equal(t, tc.wantAfterCursorVisible, viPositionVisible(vi, wantAfterCursor))
			assert.Equal(t, tc.wantAfterAnchorVisible, viPositionVisible(vi, afterAnchor))
		})
	}
}

func TestNormalPageScrolls(t *testing.T) {
	fileContent := "a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"

	suite := []struct {
		name          string
		setCursor     term.Coordinates
		inputSequence term.Event
		expect        string
	}{
		{
			name:          "ctrl-e scrolls down, keeps cursor fixed",
			setCursor:     term.Coordinates{Y: 2},
			inputSequence: term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'e'},
			expect:        "b\nX\nd\ne",
		},
		{
			name:          "ctrl-e scrolls down, moves cursor if oob",
			setCursor:     term.Coordinates{},
			inputSequence: term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'e'},
			expect:        "X\nc\nd\ne",
		},
		{
			name:          "ctrl-y scrolls up, moves cursor if oob",
			setCursor:     term.Coordinates{Y: 5},
			inputSequence: term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'y'},
			expect:        "b\nc\nd\nX",
		},
		{
			name:          "ctrl-b move screen up one page, cursor to last line",
			setCursor:     term.Coordinates{Y: 7},
			inputSequence: term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'b'},
			expect:        "c\nd\ne\nX",
		},
		{
			name:          "ctrl-f move screen down one page, cursor to first line",
			setCursor:     term.Coordinates{Y: 1},
			inputSequence: term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'f'},
			expect:        "X\ne\nf\ng",
		},
		{
			name:          "ctrl-u move screen up half page, cursor to last line",
			setCursor:     term.Coordinates{Y: 7},
			inputSequence: term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'u'},
			expect:        "c\nd\ne\nX",
		},
		{
			name:          "ctrl-d move screen down half page, cursor to first line",
			setCursor:     term.Coordinates{Y: 1},
			inputSequence: term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'd'},
			expect:        "X\ne\nf\ng",
		},
	}

	for _, tcase := range suite {
		t.Run(tcase.name, func(t *testing.T) {
			vi := setupVi(t, fileContent, 2)
			vi.Resize(1, 4)

			vi.setCursorAtScroll(tcase.setCursor)
			_, ok := vi.Handle(tcase.inputSequence)
			require.True(t, ok)

			w := term.NewStringWriter(1, 4)
			vi.Draw(w)
			c, _, ok := vi.Cursor()
			require.True(t, ok)
			w.SetCell(c, term.Cell{Width: 1, Ch: 'X'})
			w.Flush()
			assert.Equal(t, tcase.expect, w.String())

		})
	}
}

func TestCentering(t *testing.T) {
	fileContent := "a\nb\nc\nd\ne\n	f\ng\nh\ni\nj\nk"

	suite := []struct {
		name          string
		setCursor     term.Coordinates
		inputSequence string
		expect        string
	}{
		{
			name:          "zz centers the view around the cursor if there's enough offset available",
			setCursor:     term.Coordinates{Y: 5},
			inputSequence: "zz",
			expect:        "d   \ne   \n Xf \ng   ",
		},
		{
			name:          "zz does nothing with last line",
			setCursor:     term.Coordinates{Y: 10},
			inputSequence: "zz",
			expect:        "h   \ni   \nj   \nX   ",
		},
		{
			name:          "zz does nothing with first line",
			setCursor:     term.Coordinates{},
			inputSequence: "zz",
			expect:        "X   \nb   \nc   \nd   ",
		},
		{
			name:          "z. centers the view around the cursor and moves to first non blank",
			setCursor:     term.Coordinates{Y: 5},
			inputSequence: "z.",
			expect:        "d   \ne   \n  X \ng   ",
		},
		{
			name:          "zt repositions cursor at the top of the view",
			setCursor:     term.Coordinates{Y: 5},
			inputSequence: "zt",
			expect:        " Xf \ng   \nh   \ni   ",
		},
		{
			name:          "zb repositions cursor at the bottom of the view",
			setCursor:     term.Coordinates{Y: 5},
			inputSequence: "zb",
			expect:        "c   \nd   \ne   \n Xf ",
		},
	}

	for _, tcase := range suite {
		t.Run(tcase.name, func(t *testing.T) {
			vi := setupVi(t, fileContent, 2)
			vi.Resize(4, 4)

			vi.setCursorAtScroll(tcase.setCursor)
			for _, ch := range tcase.inputSequence {
				vi.Handle(term.Event{Type: term.EventKey, Ch: ch})
			}

			w := term.NewStringWriter(4, 4)
			vi.Draw(w)
			c, _, ok := vi.Cursor()
			require.True(t, ok)
			w.SetCell(c, term.Cell{Width: 1, Ch: 'X'})
			w.Flush()
			assert.Equal(t, tcase.expect, w.String())

		})
	}
}

func TestScreenRelativeMotions(t *testing.T) {
	fileContent := "a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"

	suite := []struct {
		name          string
		inputSequence string
		expected      string
	}{
		{
			name:          "H moves cursor to top visible line",
			inputSequence: "H",
			expected:      "▐\ne\nf\ng",
		},
		{
			name:          "M moves cursor to middle visible line",
			inputSequence: "M",
			expected:      "d\ne\n▐\ng",
		},
		{
			name:          "L moves cursor to bottom visible line",
			inputSequence: "L",
			expected:      "d\ne\nf\n▐",
		},
		{
			name:          "count H moves cursor to nth visible line from top",
			inputSequence: "3H",
			expected:      "d\ne\n▐\ng",
		},
		{
			name:          "count L moves cursor to nth visible line from bottom",
			inputSequence: "2L",
			expected:      "d\ne\n▐\ng",
		},
	}

	for _, tcase := range suite {
		t.Run(tcase.name, func(t *testing.T) {
			vi := setupVi(t, fileContent, 2)
			vi.Resize(1, 4)
			ok := vi.less.Scroll().SetOffset(term.Coordinates{Y: 3})
			require.True(t, ok)
			vi.setCursorAtScroll(term.Coordinates{Y: 5})

			handlertest.RunHandlerSequence(t, vi, 1, 4, []handlertest.SequenceTestCase{{
				InputSequence: tcase.inputSequence,
				Expected:      tcase.expected,
			}})
		})
	}
}

func TestPasteBatching(t *testing.T) {
	t.Run("insert mode batches paste into single edit", func(t *testing.T) {
		vi := setupVi(t, "hello\nworld", 2)
		vi.Resize(40, 10)

		// Enter insert mode
		vi.Handle(term.Event{Type: term.EventKey, Ch: 'i'})
		require.Equal(t, insertMode, vi.mode())

		// Track edits via a subscriber
		editCount := 0
		vi.less.Buffer().Subscribe(&editCounter{count: &editCount})

		// Send paste sequence: PasteStart → characters → PasteEnd
		_, handled := vi.Handle(term.Event{Type: term.EventPasteStart})
		assert.True(t, handled, "PasteStart should be handled")

		pasteText := "PASTED"
		for _, ch := range pasteText {
			_, handled = vi.Handle(term.Event{Type: term.EventKey, Ch: ch})
			assert.True(t, handled, "characters during paste should be handled")
		}

		// No edits should have happened yet (characters are buffered)
		assert.Equal(t, 0, editCount, "no edits should happen during paste buffering")

		_, handled = vi.Handle(term.Event{Type: term.EventPasteEnd})
		assert.True(t, handled, "PasteEnd should be handled")

		// Exactly one edit should have happened
		assert.Equal(t, 1, editCount, "paste should result in exactly one edit")

		// Verify the buffer content
		assert.Equal(t, "PASTEDhello\nworld", vi.less.Buffer().String())
	})

	t.Run("normal mode paste is consumed without inserting", func(t *testing.T) {
		vi := setupVi(t, "hello\nworld", 2)
		vi.Resize(40, 10)

		require.Equal(t, normalMode, vi.mode())

		_, handled := vi.Handle(term.Event{Type: term.EventPasteStart})
		assert.True(t, handled)

		for _, ch := range "PASTED" {
			_, handled = vi.Handle(term.Event{Type: term.EventKey, Ch: ch})
			assert.True(t, handled)
		}

		_, handled = vi.Handle(term.Event{Type: term.EventPasteEnd})
		assert.True(t, handled)

		// Buffer should be unchanged (paste in normal mode does not insert)
		assert.Equal(t, "hello\nworld", vi.less.Buffer().String())
	})

	t.Run("paste with newlines", func(t *testing.T) {
		vi := setupVi(t, "AB", 2)
		vi.Resize(40, 10)

		// Enter insert mode
		vi.Handle(term.Event{Type: term.EventKey, Ch: 'i'})

		vi.Handle(term.Event{Type: term.EventPasteStart})
		for _, ch := range "line1\nline2\n" {
			vi.Handle(term.Event{Type: term.EventKey, Ch: ch})
		}
		vi.Handle(term.Event{Type: term.EventPasteEnd})

		assert.Equal(t, "line1\nline2\nAB", vi.less.Buffer().String())
	})
}

// editCounter counts the number of OnDidEdit calls.
type editCounter struct {
	count *int
}

func (c *editCounter) OnWillEdit(_ context.Context, _, _ term.Coordinates, _ string) {}
func (c *editCounter) OnDidEdit(_ context.Context, _, _ term.Coordinates, _ string) {
	*c.count++
}

func TestScreenRelativeMotionsWrap(t *testing.T) {
	fileContent := "123456789\nab\nc"

	suite := []struct {
		name          string
		inputSequence string
		expected      string
	}{
		{
			name:          "count H uses wrapped screen rows",
			inputSequence: "2H",
			expected:      "123\n4▐6\n789\nab ",
		},
		{
			name:          "count L uses wrapped screen rows",
			inputSequence: "2L",
			expected:      "123\n456\n▐89\nab ",
		},
	}

	for _, tcase := range suite {
		t.Run(tcase.name, func(t *testing.T) {
			vi := setupVi(t, fileContent, 2, WithWrap(true))
			vi.Resize(3, 4)
			vi.setCursorAtScroll(term.Coordinates{X: 1, Y: 1})

			handlertest.RunHandlerSequence(t, vi, 3, 4, []handlertest.SequenceTestCase{{
				InputSequence: tcase.inputSequence,
				Expected:      tcase.expected,
			}})
		})
	}
}
