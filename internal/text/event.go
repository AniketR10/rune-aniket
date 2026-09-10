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

package text

import (
	"context"

	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/internal/cell"
	"unstable.build/rune/internal/component"
)

// ScrollSubscriber returns a component.ScrollSubscriber which forwarsd Scroll events to evHandler
func ScrollSubscriber(
	resource workspaceapi.URI, h Handler, evHandler EventHandler,
) component.ScrollSubscriber {
	return scrollSubscriber{uri: resource, h: h, eh: evHandler}
}

// CellSubscriber returns a cell.Subscriber which forwards Insert/Delete events to evHandler
func CellSubscriber(
	uri workspaceapi.URI, h Handler, evHandler EventHandler,
) cell.Subscriber {
	return &cellSubscriber{uri: uri, h: h, eh: evHandler}
}

type cellSubscriber struct {
	uri workspaceapi.URI
	h   Handler
	eh  EventHandler

	onWillEditStr   string
	onWillEditStart term.Coordinates
	onWillEditEnd   term.Coordinates
}

func (s *cellSubscriber) OnWillEdit(
	ctx context.Context, start, end term.Coordinates, str string,
) {
	s.onWillEditStr = str
	s.onWillEditStart = start
	s.onWillEditEnd = end
}

func (s *cellSubscriber) OnDidEdit(
	ctx context.Context, from, to term.Coordinates, old string,
) {
	s.eh.Handle(ctx, textapi.Event{
		Type:     textapi.EventTypeEdit,
		Resource: s.h,
		URI:      s.uri,
		From:     from,
		To:       to,
		Start:    s.onWillEditStart,
		End:      s.onWillEditEnd,
		Content:  s.onWillEditStr,
	})
}

type scrollSubscriber struct {
	uri workspaceapi.URI
	h   Handler
	eh  EventHandler
}

func (s scrollSubscriber) OnWillSeek(from term.Coordinates) {
	/* no-op */
}

func (s scrollSubscriber) OnWillHide(start, end int) {
}

func (s scrollSubscriber) OnWillVisible(start int) {
}

func (s scrollSubscriber) OnDidSeek(from, to term.Coordinates) {
	s.eh.Handle(context.Background(), textapi.Event{
		Type:     textapi.EventTypeScroll,
		Resource: s.h,
		URI:      s.uri,
		Start:    to,
		From:     from,
	})
}

func (s scrollSubscriber) OnDidHide(
	start, end int,
) {
	s.eh.Handle(context.Background(), textapi.Event{
		Type:     textapi.EventTypeHidden,
		Resource: s.h,
		URI:      s.uri,
		Start:    term.Coordinates{Y: start},
		End:      term.Coordinates{Y: end},
	})
}

func (s scrollSubscriber) OnDidVisible(
	start int,
) {
	s.eh.Handle(context.Background(), textapi.Event{
		Type:     textapi.EventTypeVisible,
		Resource: s.h,
		URI:      s.uri,
		Start:    term.Coordinates{Y: start},
	})
}
