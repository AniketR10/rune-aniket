package text

import (
	"context"

	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
)

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

// CellSubscriber returns a cell.Subscriber which forwards Insert/Delete events to evHandler
func CellSubscriber(uri workspaceapi.URI, h Handler, evHandler EventHandler) cell.Subscriber {
	return &cellSubscriber{uri: uri, h: h, eh: evHandler}
}

type scrollSubscriber struct {
	uri workspaceapi.URI
	h   Handler
	eh  EventHandler
}

func (s scrollSubscriber) OnWillSeek(from term.Coordinates) {
	/* no-op */
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

// ScrollSubscriber returns a component.ScrollSubscriber which forwarsd Scroll events to evHandler
func ScrollSubscriber(resource workspaceapi.URI, h Handler, evHandler EventHandler) component.ScrollSubscriber {
	return scrollSubscriber{uri: resource, h: h, eh: evHandler}
}
