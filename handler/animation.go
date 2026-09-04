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

package handler

import (
	"context"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/rune/cell"
	tcomponent "unstable.build/rune/component"
)

// AnimationPlayer returns a tui.Handler that wraps a component.Animation
// into a tui.Handler that plays the animation and stops with the space key.
// It also handles Escape, in which case the animation is stopped and Handle
// returns exit=true.
func AnimationPlayer(a *component.Animation) tui.Handler {
	return &player{a: a}
}

var _ tui.Handler = (*player)(nil)

type player struct {
	a           *component.Animation
	width       int
	height      int
	pause       bool
	pausedFrame tui.Component
}

func (p *player) Resize(width, height int) {
	p.width = width
	p.height = height
	p.a.Resize(width, height)
}

func (p *player) Draw(w term.Writer) {
	if !p.pause {
		p.a.Draw(w)
		return
	}

	if p.pausedFrame != nil {
		p.pausedFrame.Draw(w)
		return
	}

	p.cachePausedFrame(w.Context())
	p.pausedFrame.Draw(w)
}

func (p *player) cachePausedFrame(ctx context.Context) {
	var bw cell.BufferWriter
	var buf cell.Buffer
	bw.Init(ctx, p.width, p.height)
	p.a.Draw(&bw)
	bw.ToBuffer(&buf)

	scroll := new(tcomponent.Scroll)
	scroll.InitPerformance(&buf)
	p.pausedFrame = scroll
	p.pausedFrame.Resize(p.width, p.height)
}

func (p *player) Handle(ev term.Event) (exit, handled bool) {
	if ev.Type != term.EventKey || ev.Mod != 0 {
		return
	}

	handled = true
	switch ev.Key {
	case term.KeySpace:
		p.pause = !p.pause
		if !p.pause {
			p.pausedFrame = nil
		}
	case term.KeyEsc:
		p.cachePausedFrame(context.Background())
		p.pause = true
		exit = true
		_ = p.a.Close()
	}
	return
}

func (p *player) Cursor() (pos term.Coordinates, style term.CursorStyle, show bool) {
	return
}

func (p *player) Selection() (string, bool) {
	return "", false
}
