package text

import (
	"context"
	"errors"

	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/workspace"
)

type testEditor struct {
	uri  workspace.URI
	buf  *cell.Buffer
	subs map[EventType][]EventHandler
}

// NopEditor returns a editor suitable for testing.
// It doesn't return real tui.Handler upon Edit but mimics
// editor.Simple's subscription logic.
func NopEditor() Editor {
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

type TestEditorHandler struct {
	browser.TestHandler
	LocationList LocationList
	parent       *testEditor
	uri          workspace.URI
}

func (e *TestEditorHandler) Resource() workspace.URI {
	return e.uri
}

func (e *testEditor) Edit(resource workspace.URI, buf *cell.Buffer) (Handler, error) {
	e.uri = resource
	e.buf = buf

	h := &TestEditorHandler{
		uri:         resource,
		parent:      e,
		TestHandler: *browser.NewTestHandler(),
	}
	e.dispatchEvent(context.Background(), Event{
		Type:     EventTypeOpen,
		URI:      resource,
		Resource: h,
		Content:  buf.String(),
	})

	subs := CellSubscriber(resource, h, e)
	buf.Subscribe(subs)
	return h, nil
}

func (e *testEditor) SetLocationList(h Handler, id string, loc LocationList) error {
	h.(*TestEditorHandler).LocationList = loc
	return nil
}

func (e *TestEditorHandler) Handle(ev term.Event) (bool, bool) {
	e.parent.dispatchEvent(context.Background(), Event{
		Type:     EventTypeCursor,
		URI:      e.uri,
		Resource: e,
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
	h.(*TestEditorHandler).CursorPos = pos
	return nil
}

func (e *testEditor) Cursor(h Handler) (term.Coordinates, error) {
	return h.(*TestEditorHandler).CursorPos, nil
}

func (e *testEditor) CellEditor(h Handler) CellEditor {
	return NewCellEditor(e.buf.Editor())
}

func (e *testEditor) CellView(h Handler) CellView {
	return NewCellView(e.buf.View())
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

func (e *testEditor) Editor(file workspace.URI) (Handler, error) {
	return nil, errors.New("nope")
}
