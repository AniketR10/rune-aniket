package text

import (
	"context"
	"errors"

	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
)

type testEditor struct {
	name string
	buf  *cell.Buffer
	subs map[EventType][]EventHandler
}

// Mock returns a editor suitable for testing.
// It doesn't return real tui.Handler upon Edit but mimics
// editor.Simple's subscription logic.
func Mock() Editor {
	return &testEditor{}
}

func (e *testEditor) Handle(ctx context.Context, ev Event) bool {
	e.dispatchEvent(ctx, ev)
	return false
}

func (e *testEditor) dispatchEvent(ctx context.Context, ev Event) {
	if len(e.subs) == 0 {
		return
	}
	subs, ok := e.subs[ev.Type]
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
	e.subs[ev.Type] = remain
}

type testEditorHandler struct {
	browser.TestHandler
	locationList LocationList
	parent       *testEditor
	name         string
}

func (e *testEditorHandler) Name() string {
	return e.name
}

func (e *testEditor) Edit(name string, buf *cell.Buffer) (Handler, error) {
	e.name = name
	e.buf = buf

	h := &testEditorHandler{
		name:        name,
		parent:      e,
		TestHandler: *browser.NewTestHandler(),
	}
	e.dispatchEvent(context.Background(), Event{
		Type:         EventTypeOpen,
		ResourceName: name,
		Resource:     h,
		Content:      buf.String(),
	})

	subs := CellSubscriber(name, h, e)
	buf.Subscribe(subs)
	return h, nil
}

func (e *testEditor) SetLocationList(h Handler, id string, loc LocationList) error {
	h.(*testEditorHandler).locationList = loc
	return nil
}

func (e *testEditorHandler) Handle(ev term.Event) (bool, bool) {
	e.parent.dispatchEvent(context.Background(), Event{
		Type:         EventTypeCursor,
		ResourceName: e.name,
		Resource:     e,
	})
	return e.TestHandler.Handle(ev)
}

func (e *testEditor) MoveToNextLocation(h Handler, ID string) error {
	return nil
}

func (e *testEditor) MoveToPrevLocation(h Handler, ID string) error {
	return nil
}

func (e *testEditor) SetCursor(h Handler, pos term.Coordinates) error {
	h.(*testEditorHandler).CursorPos = pos
	return nil
}

func (e *testEditor) Cursor(h Handler) (term.Coordinates, error) {
	return h.(*testEditorHandler).CursorPos, nil
}

func (e *testEditor) Writer(h Handler) Writer {
	return CellWriter(e.buf.Writer())
}

func (e *testEditor) Reader(h Handler) Reader {
	return CellReader(e.buf.Reader())
}

func (e *testEditor) SubscribeCommand(cmd string, h CommandHandler) error {
	return nil
}

func (e *testEditor) SubscribeEditorEvents(
	evs []EventType, sub EventHandler,
) error {
	for _, ev := range evs {
		if e.subs == nil {
			e.subs = make(map[EventType][]EventHandler)
		}
		if _, ok := e.subs[ev]; !ok {
			e.subs[ev] = []EventHandler{sub}
		} else {
			e.subs[ev] = append(e.subs[ev], sub)
		}
	}
	return nil
}

func (e *testEditor) Editor(name string) (Handler, error) {
	return nil, errors.New("nope")
}
