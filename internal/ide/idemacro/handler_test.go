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

package idemacro

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"github.com/unstablebuild/rune-go-sdk/term"
)

var testCommandPrompt = term.KeyComb{Ch: ':'}

func TestHandlerRecordsKeysIntoRegister(t *testing.T) {
	clip := clipboard.NewInMemory()
	h := New(clip, nil, testCommandPrompt)
	ctx := context.Background()

	require.NoError(t, h.HandleCommand(ctx, textapi.Command{
		Name: "record",
		Args: []string{"a"},
	}))
	h.BeginEvent(term.Event{Type: term.EventMouse, Key: term.MouseLeft})
	h.EndEvent()
	handleKeys(t, h, "i<space>:stop-recording<space>a<enter>")
	require.NoError(t, h.HandleCommand(ctx, textapi.Command{
		Name: "stop-recording",
		Args: []string{"a"},
	}))

	data, err := clip.Paste("a")
	require.NoError(t, err)
	require.Equal(t, "i<space>", data.Text)
}

func TestHandlerStopsCurrentRecordingWithoutRegisterArgument(t *testing.T) {
	clip := clipboard.NewInMemory()
	h := New(clip, nil, testCommandPrompt)
	ctx := context.Background()

	require.NoError(t, h.HandleCommand(ctx, textapi.Command{
		Name: "record",
		Args: []string{"q"},
	}))
	handleKeys(t, h, "x:record<enter>")
	require.NoError(t, h.HandleCommand(ctx, textapi.Command{Name: "record"}))

	data, err := clip.Paste("q")
	require.NoError(t, err)
	require.Equal(t, "x", data.Text)
}

func TestHandlerDiscardsConfiguredCommandPromptKey(t *testing.T) {
	clip := clipboard.NewInMemory()
	commandPrompt := term.KeyComb{Mod: term.ModCtrl, Key: term.KeySpace}
	h := New(clip, nil, commandPrompt)
	ctx := context.Background()

	require.NoError(t, h.HandleCommand(ctx, textapi.Command{
		Name: "record",
		Args: []string{"q"},
	}))
	handleKeys(t, h, "x<c-space>record<enter>")
	require.NoError(t, h.HandleCommand(ctx, textapi.Command{Name: "record"}))

	data, err := clip.Paste("q")
	require.NoError(t, err)
	require.Equal(t, "x", data.Text)
}

func TestHandlerDoesNotDiscardRecordedCommandTextWhenStoppedByBinding(t *testing.T) {
	clip := clipboard.NewInMemory()
	h := New(clip, nil, testCommandPrompt)
	ctx := context.Background()

	require.NoError(t, h.HandleCommand(ctx, textapi.Command{
		Name: "record",
		Args: []string{"q"},
	}))
	handleKeys(t, h, "record<enter>")
	require.NoError(t, h.HandleCommand(ctx, textapi.Command{Name: "record"}))

	data, err := clip.Paste("q")
	require.NoError(t, err)
	require.Equal(t, "record<enter>", data.Text)
}

func TestHandlerDiscardsCurrentKeyWhenRecordingStopsFromBinding(t *testing.T) {
	clip := clipboard.NewInMemory()
	h := New(clip, nil, testCommandPrompt)
	ctx := context.Background()

	require.NoError(t, h.HandleCommand(ctx, textapi.Command{
		Name: "record",
		Args: []string{"q"},
	}))
	handleKeys(t, h, "i")
	h.BeginEvent(term.Event{Type: term.EventKey, Ch: 'q'})
	require.NoError(t, h.HandleCommand(ctx, textapi.Command{Name: "record"}))
	h.EndEvent()

	data, err := clip.Paste("q")
	require.NoError(t, err)
	require.Equal(t, "i", data.Text)
}

func TestHandlerDiscardsPreviousTriggerKeyWhenPartialSequenceIsReissued(t *testing.T) {
	clip := clipboard.NewInMemory()
	h := New(clip, nil, testCommandPrompt)
	ctx := context.Background()

	require.NoError(t, h.HandleCommand(ctx, textapi.Command{
		Name: "record",
		Args: []string{"q"},
	}))
	handleKeys(t, h, "i")
	// The first q is the user's original key press: the IDE sequencer records it
	// as a partial sequence but does not dispatch the record command yet.
	h.BeginEvent(term.Event{Type: term.EventKey, Ch: 'q'})
	h.EndEvent()
	// The second q is the IDE's reissued event after the sequence timeout; it is
	// the event that dispatches record with no args.
	h.BeginEvent(term.Event{Type: term.EventKey, Ch: 'q'})
	require.NoError(t, h.HandleCommand(ctx, textapi.Command{Name: "record"}))
	h.EndEvent()

	data, err := clip.Paste("q")
	require.NoError(t, err)
	require.Equal(t, "i", data.Text)
}

func TestHandlerNotifiesWhenRecordingStartsAndStops(t *testing.T) {
	clip := clipboard.NewInMemory()
	notis := new(stubNotifications)
	h := New(clip, notis, testCommandPrompt)
	ctx := context.Background()

	require.NoError(t, h.HandleCommand(ctx, textapi.Command{
		Name: "record",
		Args: []string{"a"},
	}))
	handleKeys(t, h, "x")
	require.NoError(t, h.HandleCommand(ctx, textapi.Command{
		Name: "record",
		Args: []string{"a"},
	}))

	require.Equal(t, []stubNotification{
		{level: browserapi.LevelInfo, msg: `Started recording macro into register "a"`},
		{level: browserapi.LevelSuccess, msg: `Recorded macro into register "a": x`},
	}, notis.notifications)
}

func TestPlayerRejectsSelfReferentialRegister(t *testing.T) {
	clip := clipboard.NewInMemory()
	require.NoError(t, clip.Copy("q", clipboard.Data{Text: "@q"}))

	var events []term.Event
	player := NewPlayer(clip, nil, func(ev term.Event) bool {
		events = append(events, ev)
		return true
	})

	require.NoError(t, player.Play("q", 1))
	require.Len(t, events, 3)

	// Simulate the vi handler consuming @q from the published events. This
	// second Play call should be rejected because q is still being replayed.
	err := player.Play("q", 1)
	require.Error(t, err)
	require.ErrorContains(t, err, `already replaying register "q"`)
	require.Len(t, events, 3, "recursive playback should not publish more events")
}

func handleKeys(t *testing.T, h *Recorder, seq string) {
	t.Helper()
	keys, err := term.ParseKeys(seq)
	require.NoError(t, err)
	for _, key := range keys {
		h.BeginEvent(term.Event{
			Type: term.EventKey,
			Ch:   key.Ch,
			Mod:  key.Mod,
			Key:  key.Key,
		})
		h.EndEvent()
	}
}

type stubNotification struct {
	level browserapi.NotificationLevel
	msg   string
}

type stubNotifications struct {
	notifications []stubNotification
}

func (s *stubNotifications) Notify(
	level browserapi.NotificationLevel, msg string, args ...any,
) (string, error) {
	s.notifications = append(s.notifications, stubNotification{
		level: level,
		msg:   fmt.Sprintf(msg, args...),
	})
	return "", nil
}

func (s *stubNotifications) NotifyOnce(
	level browserapi.NotificationLevel, msg string, args ...any,
) (string, error) {
	return s.Notify(level, msg, args...)
}

func (s *stubNotifications) UpdateNotificationProgress(
	id, message string, progress, total int64,
) error {
	return nil
}
