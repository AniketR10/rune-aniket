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

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/internal/cell"
)

// Publisher implements pub/sub functionality for Editor implementations.
type Publisher struct {
	subs map[textapi.EventType][]EventHandler
}

type cursorPublisher struct {
	parent *Publisher
	uri    workspaceapi.URI
	buf    *cell.Buffer
	cursor *Cursor
	Handler
}

func (p *cursorPublisher) SelectionBounds() (from, to term.Coordinates, ok bool) {
	return p.cursor.SelectionBounds()
}

// NewPublisher allocates storage for a new Publisher and initializes it.
func NewPublisher() *Publisher {
	ret := new(Publisher)
	ret.Init()
	return ret
}

// Init initializes this Publisher.
func (p *Publisher) Init() {
	p.subs = make(map[textapi.EventType][]EventHandler)
}

// PublishEdit publishes EventTypeOpen and EventTypeFocus events to subscribers
// and wraps root with a Handler that dispatches EventTypeCursor events.
// It also subscribes to scroll changes to dispatch EventTypeScroll, and
// subscribes to buffer updates to dispatch EventTypeEdit.
//
// cursor MUST be non-nil. Editors that do not own a *Cursor (e.g.
// text/exoeditor, which hosts an external TUI editor inside a vte and
// therefore has no Rune-side cursor at all) must use
// PublishExternalEdit instead.
func (p *Publisher) PublishEdit(
	resource workspaceapi.URI, buf *cell.Buffer, root Handler, cursor *Cursor,
) Handler {
	if cursor == nil {
		panic("text.Publisher.PublishEdit: cursor is nil; " +
			"use PublishExternalEdit for cursor-less editors")
	}
	h := &cursorPublisher{
		buf:     buf,
		parent:  p,
		uri:     resource,
		cursor:  cursor,
		Handler: root,
	}

	ctx := context.Background()

	p.dispatchEvent(ctx, textapi.Event{
		Type:     textapi.EventTypeOpen,
		URI:      resource,
		Resource: h,
		Content:  buf.String(),
	})

	// if simple is the final tui.Handler, then the underlying handler
	// is always in focus. Consumers of this Editor should not
	// delegate SubscribeEditor to this handler if there's some other
	// focus mechanism in place.
	p.dispatchEvent(ctx, textapi.Event{
		Type:     textapi.EventTypeFocus,
		URI:      resource,
		Resource: h,
	})

	bsub := CellSubscriber(resource, h, p)

	// NOTE: should probably find a way to unsubscribe, since
	// a cell.Buffer can outlive this Publisher
	buf.Subscribe(bsub)

	csub := ScrollSubscriber(resource, h, p)
	cursor.SubscribeScroll(csub)

	return h
}

// PublishExternalEdit publishes EventTypeOpen, EventTypeFocus and
// EventTypeEdit (via buffer subscription) for an editor that does NOT
// own a *Cursor. Scroll, cursor and selection events are not
// published because there is no Rune-side cursor to observe; the
// external editor process (and its host vte) owns the cursor and
// scrolling instead.
//
// The returned handler is root unchanged: callers must wire the
// handler into the tab system directly. This avoids the
// cursorPublisher wrapper, which dereferences the cursor on every
// keypress.
func (p *Publisher) PublishExternalEdit(
	resource workspaceapi.URI, buf *cell.Buffer, root Handler,
) Handler {
	ctx := context.Background()

	p.dispatchEvent(ctx, textapi.Event{
		Type:     textapi.EventTypeOpen,
		URI:      resource,
		Resource: root,
		Content:  buf.String(),
	})
	p.dispatchEvent(ctx, textapi.Event{
		Type:     textapi.EventTypeFocus,
		URI:      resource,
		Resource: root,
	})

	bsub := CellSubscriber(resource, root, p)
	buf.Subscribe(bsub)
	return root
}

// SubscribeEvents subsribes sub to ev.
func (p *Publisher) SubscribeEvents(evs []textapi.EventType, sub EventHandler) {
	for _, ev := range evs {
		if _, ok := p.subs[ev]; !ok {
			p.subs[ev] = []EventHandler{sub}
		} else {
			p.subs[ev] = append(p.subs[ev], sub)
		}
	}
	p.log(log.TraceLevel, "subscribe subscriber sub=%p to evs %+v: "+
		"subscribers=%+v", sub, evs, p.subs)
}

// UnsubscribeEvents unsubscribes sub from all events.
func (p *Publisher) UnsubscribeEvents(sub EventHandler) (ret bool) {
	final := make(map[textapi.EventType][]EventHandler)
	for ev, subs := range p.subs {
		final[ev] = make([]EventHandler, 0, len(subs))
		for _, s := range subs {
			if s != sub {
				final[ev] = append(final[ev], s)
			} else {
				ret = true
			}
		}
	}
	p.subs = final
	p.log(log.TraceLevel, "unsubscribing subscriber sub=%p: "+
		"unsubscribed called. subscribers=%+v", sub, p.subs)
	return
}

func (p *Publisher) dispatchEvent(ctx context.Context, ev textapi.Event) {
	subs, ok := p.subs[ev.Type]
	if !ok {
		return
	}

	// call Handle in batch, as it might be an rpc, and removing from
	// edSubsribers after each call could introduce race conditions.
	exits := make([]bool, len(subs))
	for i, h := range subs {
		exits[i] = h.Handle(ctx, ev)
		p.log(log.TraceLevel, "dispatched to subscriber sub=%p, "+
			"event type %d: exit=%t", h, ev.Type, exits[i])
	}

	p.subs[ev.Type] = make([]EventHandler, 0, len(subs))
	for i, sub := range subs {
		if !exits[i] {
			p.subs[ev.Type] = append(p.subs[ev.Type], sub)
		}
	}
}

// Handle handles ev by dispatching to subscribers.
func (p *Publisher) Handle(ctx context.Context, ev textapi.Event) bool {
	p.dispatchEvent(ctx, ev)
	return false
}

// RecordCursorChange records a cursor change between this function call and
// dispatchEvent being called. If there is a cursor position change,
// then an EventTypeCursor is dispatched to subscribers.
func (p *Publisher) RecordCursorChange(h Handler) (dispatchEvent func()) {
	handler := h.(*cursorPublisher)
	cursor0 := handler.cursor.Coordinates()
	cursorAtScroll0 := handler.cursor.CursorAtScroll()
	selection0 := handler.cursor.Selection()

	return func() {
		cursor1 := handler.cursor.Coordinates()
		cursorAtScroll1 := handler.cursor.CursorAtScroll()
		selection1 := handler.cursor.Selection()

		if cursor0 != cursor1 || cursorAtScroll0 != cursorAtScroll1 {
			p.dispatchEvent(context.Background(), textapi.Event{
				Type:     textapi.EventTypeCursor,
				URI:      handler.uri,
				Resource: h,
				Start:    cursor1,
				From:     cursorAtScroll1,
			})
		}

		selectionFrom, _ := handler.cursor.SelectionFrom()
		if selection0 != selection1 {
			// select coordinates are right exclusive, but cursor is not
			cursorAtScroll1.X++
			p.dispatchEvent(context.Background(), textapi.Event{
				Type:     textapi.EventTypeSelection,
				URI:      handler.uri,
				Resource: h,
				Start:    selectionFrom,
				End:      cursorAtScroll1,
				Content:  selection1,
			})
		}

		p.log(log.TraceLevel, "record cursor change for %p: cursor before: %+v, "+
			"cursor after: %+v, cursorAtScroll before: %+v, cursorAtScroll after: %+v, "+
			"selection before: %s, selection after: %s",
			h, cursor0, cursor1, cursorAtScroll0, cursorAtScroll1,
			selection0, selection1)
	}
}

// Handle dispatchs cursor events if cursor has changed after
// underlying handler has processed ev.
func (p *cursorPublisher) Handle(ev term.Event) (bool, bool) {
	dispatch := p.parent.RecordCursorChange(p)
	defer dispatch()

	return p.Handler.Handle(ev)
}

// helper for internal tests
func (p *cursorPublisher) CursorReference() *Cursor {
	return p.cursor
}

func (p *cursorPublisher) SetCursorAtScroll(pos term.Coordinates) bool {
	dispatch := p.parent.RecordCursorChange(p)
	defer dispatch()

	return p.Handler.SetCursorAtScroll(pos)
}

func (p *cursorPublisher) MoveToNextLocation(ID string) bool {
	dispatch := p.parent.RecordCursorChange(p)
	defer dispatch()

	return p.Handler.MoveToNextLocation(ID)
}

func (p *cursorPublisher) MoveToPrevLocation(ID string) bool {
	dispatch := p.parent.RecordCursorChange(p)
	defer dispatch()

	return p.Handler.MoveToPrevLocation(ID)
}

func (p *Publisher) log(level log.Level, msg string, args ...any) {
	if !log.IsLevelEnabled(level) {
		return
	}
	log.WithFields(log.Fields{
		logging.KeyClass: "text.Publisher",
	}).Logf(level, msg, args...)
}
