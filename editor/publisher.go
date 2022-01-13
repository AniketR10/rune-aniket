package editor

import (
	"context"

	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
)

// Publisher implements pub/sub functionality for Editor implementations.
type Publisher struct {
	subs map[EventType][]EventHandler
}

type cursorPublisher struct {
	parent *Publisher
	name   string
	buf    *cell.Buffer
	cursor *Cursor
	Handler
}

// NewPublisher allocates storage for a new Publisher and initializes it.
func NewPublisher() *Publisher {
	ret := new(Publisher)
	ret.Init()
	return ret
}

// Init initializes this Publisher.
func (p *Publisher) Init() {
	p.subs = make(map[EventType][]EventHandler)
}

// PublishEdit publishes EventTypeOpen and EventTypeFocus events to subscribers
// and wraps root with a Handler that dispatches EventTypeCursor events.
// It also subscribes to scroll changes to dispatch EventTypeScroll, and
// subscribes to buffer updates to dispatch EventTypeDelete and EventTypeInsert.
func (p *Publisher) PublishEdit(
	name string, buf *cell.Buffer, root Handler, cursor *Cursor,
) Handler {
	h := &cursorPublisher{
		buf:     buf,
		parent:  p,
		name:    name,
		cursor:  cursor,
		Handler: root,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p.dispatchEvent(ctx, Event{
		Type:         EventTypeOpen,
		ResourceName: name,
		Resource:     h,
		Content:      buf.String(),
	})

	// if simple is the final tui.Handler, then the underlying handler
	// is always in focus. Consumers of this Editor should not
	// delegate SubscribeEditor to this handler if there's some other
	// focus mechanism in place.
	p.dispatchEvent(ctx, Event{
		Type:         EventTypeFocus,
		ResourceName: name,
		Resource:     h,
	})

	bsub := CellSubscriber(name, h, p)
	buf.Subscribe(bsub)

	csub := ScrollSubscriber(name, h, p)
	cursor.SubscribeScroll(csub)

	return h
}

// Handler returns the underlying handler passed to PublishEdit.
func (p *Publisher) Handler(h Handler) Handler {
	return h.(*cursorPublisher).Handler
}

// SubscribeEditorEvents subsribes sub to ev.
func (p *Publisher) SubscribeEditorEvents(evs []EventType, sub EventHandler) {
	for _, ev := range evs {
		if _, ok := p.subs[ev]; !ok {
			p.subs[ev] = []EventHandler{sub}
		} else {
			p.subs[ev] = append(p.subs[ev], sub)
		}
	}
}

func (p *Publisher) dispatchEvent(ctx context.Context, ev Event) {
	subs, ok := p.subs[ev.Type]
	if !ok {
		return
	}

	remain := make([]EventHandler, 0, len(subs))
	for _, sub := range subs {
		exit := sub.Handle(ctx, ev)
		if !exit {
			remain = append(remain, sub)
		}
	}
	p.subs[ev.Type] = remain
}

// Handle handles ev by dispatching to subscribers.
func (p *Publisher) Handle(ctx context.Context, ev Event) bool {
	p.dispatchEvent(ctx, ev)
	return false
}

// Handle dispatchs cursor events if cursor has changed after
// underlying handler has processed ev.
func (p *cursorPublisher) Handle(ev term.Event) (bool, bool) {
	cursor0, _ := p.Handler.Cursor()
	cursorAtScroll0 := p.cursor.CursorAtScroll()

	exit, handled := p.Handler.Handle(ev)
	cursor1, _ := p.Handler.Cursor()
	cursorAtScroll1 := p.cursor.CursorAtScroll()

	if cursor0 != cursor1 || cursorAtScroll0 != cursorAtScroll1 {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		p.parent.dispatchEvent(ctx, Event{
			Type:         EventTypeCursor,
			ResourceName: p.name,
			Resource:     p,
			Start:        cursor1,
			From:         cursorAtScroll1,
		})
	}
	return exit, handled
}
