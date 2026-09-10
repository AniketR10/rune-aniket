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

package gui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func newEchoTestGUI(t *testing.T) *GUI {
	t.Helper()
	mock := &mockHandler{
		assertDraw:  func(term.Writer) {},
		assertEvent: func(term.Event) (bool, bool) { return false, true },
	}
	gui, _ := newTestGUI(t, mock)
	return gui
}

func overrideEchoBudget(t *testing.T, budget time.Duration) {
	t.Helper()
	orig := echoWaitBudget
	echoWaitBudget = budget
	t.Cleanup(func() { echoWaitBudget = orig })
}

func publishKey(t *testing.T, g *GUI) {
	t.Helper()
	require.True(t, g.PublishEvent(term.Event{
		Type: term.EventKey,
		Ch:   'a',
		Raw:  []byte("a"),
	}))
}

// armEcho establishes the key-then-interrupt pattern that marks the
// focused handler as asynchronously echoing.
func armEcho(t *testing.T, g *GUI) {
	t.Helper()
	publishKey(t, g)
	require.NoError(t, g.Update())
	require.True(t, g.PublishEvent(term.Event{Type: term.EventInterrupt}))
	require.NoError(t, g.Update())
	require.True(t, g.echoLikely)
}

func TestUpdateEchoWaitNotArmedByDefault(t *testing.T) {
	overrideEchoBudget(t, 250*time.Millisecond)
	g := newEchoTestGUI(t)

	publishKey(t, g)
	start := time.Now()
	require.NoError(t, g.Update())

	assert.Less(t, time.Since(start), 100*time.Millisecond,
		"an unarmed keystroke must not pay the echo wait")
	assert.False(t, g.echoLikely)
}

func TestUpdateArmsEchoAfterKeyThenInterrupt(t *testing.T) {
	g := newEchoTestGUI(t)
	armEcho(t, g)
}

func TestUpdateFoldsEchoInterruptSameTick(t *testing.T) {
	overrideEchoBudget(t, 500*time.Millisecond)
	g := newEchoTestGUI(t)
	armEcho(t, g)

	publishKey(t, g)
	go func() {
		time.Sleep(20 * time.Millisecond)
		g.PublishEvent(term.Event{Type: term.EventInterrupt})
	}()
	start := time.Now()
	require.NoError(t, g.Update())
	elapsed := time.Since(start)

	assert.False(t, g.interruptPending.Load(),
		"the echo interrupt must be folded into the same tick")
	assert.GreaterOrEqual(t, elapsed, 20*time.Millisecond,
		"Update must have waited for the echo")
	assert.Less(t, elapsed, 400*time.Millisecond,
		"the wait must end when the echo arrives, not at the budget")
	assert.True(t, g.echoLikely, "a successful fold must keep the wait armed")
	assert.True(t, g.NeedsRender())
}

func TestUpdateEchoWaitDisarmsAfterTimeout(t *testing.T) {
	overrideEchoBudget(t, 30*time.Millisecond)
	g := newEchoTestGUI(t)
	armEcho(t, g)

	publishKey(t, g)
	start := time.Now()
	require.NoError(t, g.Update())
	assert.GreaterOrEqual(t, time.Since(start), 30*time.Millisecond,
		"an armed single keystroke must wait out the budget when no echo comes")
	assert.False(t, g.echoLikely, "a timed-out wait must disarm")

	publishKey(t, g)
	start = time.Now()
	require.NoError(t, g.Update())
	assert.Less(t, time.Since(start), 20*time.Millisecond,
		"a disarmed keystroke must not wait")
}

func TestUpdateEchoWaitSkipsKeyBursts(t *testing.T) {
	overrideEchoBudget(t, 250*time.Millisecond)
	g := newEchoTestGUI(t)
	armEcho(t, g)

	publishKey(t, g)
	publishKey(t, g)
	start := time.Now()
	require.NoError(t, g.Update())

	assert.Less(t, time.Since(start), 100*time.Millisecond,
		"bursts must never pay the echo wait")
	assert.True(t, g.echoLikely, "a skipped burst must not disarm")
}

func TestUpdateEchoWaitSkipsWhenInterruptAlreadyProcessed(t *testing.T) {
	overrideEchoBudget(t, 250*time.Millisecond)
	g := newEchoTestGUI(t)
	armEcho(t, g)

	publishKey(t, g)
	require.True(t, g.PublishEvent(term.Event{Type: term.EventInterrupt}))
	start := time.Now()
	require.NoError(t, g.Update())

	assert.Less(t, time.Since(start), 100*time.Millisecond,
		"a tick that already folded an interrupt must not wait for another")
	assert.True(t, g.echoLikely)
}
