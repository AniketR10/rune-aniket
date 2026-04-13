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
