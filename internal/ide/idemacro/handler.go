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

// Package idemacro records key events into clipboard registers.
package idemacro

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/internal/text"
)

var _ text.CommandHandler = (*Recorder)(nil)

// New returns a handler that records all key events passed to Handle into the
// register selected with its subscribed command.
func New(
	clip clipboard.Register,
	notifications browserapi.Notifications,
	commandPrompt term.KeyComb,
) *Recorder {
	return &Recorder{
		clip:             clip,
		notifications:    notifications,
		commandPromptKey: commandPrompt.String(),
	}
}

// Recorder records key events into clipboard registers.
type Recorder struct {
	clip             clipboard.Register
	notifications    browserapi.Notifications
	registerID       string
	recording        bool
	keys             []string
	inEvent          bool
	currentKey       int
	currentKeyStr    string
	commandPromptKey string
}

// Start begins recording into the given register ID and reports errors via notifications.
func (h *Recorder) Start(registerID string) {
	if err := h.start(registerID); err != nil {
		h.notify(browserapi.LevelError, "%v", err)
	}
}

// Stop ends the current recording and reports errors via notifications.
func (h *Recorder) Stop() {
	h.discardCurrentEvent()
	if err := h.finish(); err != nil {
		h.notify(browserapi.LevelError, "%v", err)
	}
}

// IsRecording reports whether a recording is currently active.
func (h *Recorder) IsRecording() bool {
	return h.recording
}

// RegisterID reports the active recording register, or an empty string if idle.
func (h *Recorder) RegisterID() string {
	if !h.recording {
		return ""
	}
	return h.registerID
}

// BeginEvent records ev and marks it as the event currently being dispatched.
func (h *Recorder) BeginEvent(ev term.Event) {
	h.inEvent = true
	h.currentKey = -1
	h.currentKeyStr = ""
	if ev.Type != term.EventKey {
		return
	}
	h.currentKeyStr = ev.KeyComb().String()
	if !h.recording {
		return
	}
	h.currentKey = len(h.keys)
	h.handle(ev)
}

// EndEvent clears the event currently being dispatched.
func (h *Recorder) EndEvent() {
	h.inEvent = false
	h.currentKey = -1
	h.currentKeyStr = ""
}

// HandleCommand starts or stops macro recording for the requested register.
func (h *Recorder) HandleCommand(ctx context.Context, cmd textapi.Command) error {
	if len(cmd.Args) > 1 {
		return errors.New("expected zero arguments to stop recording or one register ID to start recording")
	}
	if h.recording {
		triggerKey := h.currentKeyStr
		h.discardCurrentEvent()
		if !h.discardLastCommand(cmd) {
			h.discardLastKey(triggerKey)
		}
		if len(cmd.Args) == 0 {
			return h.finish()
		}
		if cmd.Args[0] != h.registerID {
			return fmt.Errorf("already recording register %q", h.registerID)
		}
		return h.finish()
	}
	if len(cmd.Args) == 0 {
		return errors.New("expected one argument with the register ID")
	}
	return h.start(cmd.Args[0])
}

// Complete implements text.CommandHandler.
func (h *Recorder) Complete(ctx context.Context, cmd textapi.Command) (
	iterator.Iterator[string], string, error,
) {
	return iterator.Empty[string](), "", nil
}

func (h *Recorder) discardLastKey(key string) {
	if key == "" || len(h.keys) == 0 {
		return
	}
	if h.keys[len(h.keys)-1] != key {
		return
	}
	h.keys = h.keys[:len(h.keys)-1]
}

func (h *Recorder) discardCurrentEvent() {
	if !h.inEvent || h.currentKey < 0 || h.currentKey >= len(h.keys) {
		return
	}
	copy(h.keys[h.currentKey:], h.keys[h.currentKey+1:])
	h.keys = h.keys[:len(h.keys)-1]
	h.currentKey = -1
}

func (h *Recorder) discardLastCommand(cmd textapi.Command) bool {
	if h.discardLastCommandSuffix(cmd, true) {
		return true
	}
	return h.discardLastCommandSuffix(cmd, false)
}

func (h *Recorder) discardLastCommandSuffix(cmd textapi.Command, includeEnter bool) bool {
	suffix, err := commandKeys(cmd, includeEnter)
	if err != nil || len(h.keys) < len(suffix) {
		return false
	}
	start := len(h.keys) - len(suffix)
	for i, key := range suffix {
		if h.keys[start+i] != key {
			return false
		}
	}
	if start == 0 {
		return false
	}
	if h.keys[start-1] != h.commandPromptKey {
		return false
	}
	start--
	h.keys = h.keys[:start]
	return true
}

func commandKeys(cmd textapi.Command, includeEnter bool) ([]string, error) {
	var builder strings.Builder
	builder.WriteString(cmd.Name)
	for _, arg := range cmd.Args {
		builder.WriteString("<space>")
		builder.WriteString(arg)
	}
	if includeEnter {
		builder.WriteString("<enter>")
	}
	keys, err := term.ParseKeys(builder.String())
	if err != nil {
		return nil, err
	}
	ret := make([]string, len(keys))
	for i, key := range keys {
		ret[i] = key.String()
	}
	return ret, nil
}

func (h *Recorder) handle(ev term.Event) (exit, handled bool) {
	if !h.recording || ev.Type != term.EventKey {
		return false, false
	}
	key := ev.KeyComb()
	h.keys = append(h.keys, key.String())
	return false, false
}

func (h *Recorder) stop() error {
	if !h.recording {
		return errors.New("not currently recording")
	}
	h.recording = false
	text := strings.Join(h.keys, "")
	h.keys = h.keys[:0]
	if err := h.clip.Copy(h.registerID, clipboard.Data{Text: text}); err != nil {
		return err
	}
	h.notify(browserapi.LevelSuccess,
		"Recorded macro into register %q: %s", h.registerID, text)
	return nil
}

func (h *Recorder) start(registerID string) error {
	if registerID == "" {
		return errors.New("expected one argument with the register ID")
	}
	if h.recording {
		return fmt.Errorf("already recording register %q", h.registerID)
	}
	h.recording = true
	h.registerID = registerID
	h.keys = h.keys[:0]
	h.notify(browserapi.LevelInfo, "Started recording macro into register %q", h.registerID)
	return nil
}

func (h *Recorder) finish() error {
	return h.stop()
}

func (h *Recorder) notify(level browserapi.NotificationLevel, msg string, args ...any) {
	if h.notifications == nil {
		return
	}
	_, _ = h.notifications.Notify(level, msg, args...)
}
