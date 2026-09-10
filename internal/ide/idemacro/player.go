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
	"fmt"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// Player replays macro registers into the event loop.
type Player struct {
	clip         clipboard.Register
	recorder     *Recorder
	publishEvent func(term.Event) bool
	mu           sync.Mutex
	active       map[string]struct{}
}

// NewPlayer creates a Player that reads registers from clip, checks
// recorder for active-recording guards, and publishes keys via publishEvent.
func NewPlayer(
	clip clipboard.Register,
	recorder *Recorder,
	publishEvent func(term.Event) bool,
) *Player {
	return &Player{
		clip:         clip,
		recorder:     recorder,
		publishEvent: publishEvent,
		active:       make(map[string]struct{}),
	}
}

// Play reads the given register and publishes its key sequence count times.
func (p *Player) Play(registerID string, count int) error {
	if count < 1 {
		count = 1
	}
	if p.recorder != nil && p.recorder.IsRecording() && p.recorder.RegisterID() == registerID {
		return fmt.Errorf("cannot play register %q while it is actively being recorded", registerID)
	}

	data, err := p.clip.Paste(registerID)
	if err != nil {
		return fmt.Errorf("paste register %q: %w", registerID, err)
	}

	if data.Text == "" {
		return nil
	}

	keys, err := term.ParseKeys(data.Text)
	if err != nil {
		return fmt.Errorf("parse register %q: %w", registerID, err)
	}
	if len(keys) == 0 {
		return nil
	}
	if err := p.beginPlayback(registerID); err != nil {
		return err
	}
	return p.publishKeys(registerID, keys, count)
}

func (p *Player) beginPlayback(registerID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.active == nil {
		p.active = make(map[string]struct{})
	}
	if _, ok := p.active[registerID]; ok {
		return fmt.Errorf("already replaying register %q", registerID)
	}
	p.active[registerID] = struct{}{}
	return nil
}

func (p *Player) finishPlayback(registerID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.active, registerID)
}

// IsPlaying reports whether any replayed register is still being drained
// through the event loop.
func (p *Player) IsPlaying() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.active) > 0
}

func (p *Player) publishKeys(registerID string, keys []term.KeyComb, count int) error {
	ok := true
	for range count {
		for _, key := range keys {
			ok = ok && p.publishEvent(term.Event{
				Type: term.EventKey,
				Ch:   key.Ch,
				Mod:  key.Mod,
				Key:  key.Key,
			})
		}
	}
	releaseOK := p.publishEvent(term.Event{Type: term.EventInterrupt, UserFunc: func() {
		p.finishPlayback(registerID)
	}})
	if !releaseOK {
		p.finishPlayback(registerID)
	}

	if !ok || !releaseOK {
		return fmt.Errorf("could not publish all events for register %q", registerID)
	}
	return nil
}
