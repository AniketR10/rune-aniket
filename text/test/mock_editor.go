package test

import (
	"context"
	"errors"

	"github.com/sirupsen/logrus"
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	browsertest "unstable.build/go-tui/browser/test"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
)

type testEditor struct {
	uri  workspaceapi.URI
	buf  *cell.Buffer
	subs map[textapi.EventType][]text.EventHandler
	cb   func()
}

// NopEditor returns a editor suitable for testing.
// It doesn't return real tui.Handler upon Edit but mimics
// editor.Simple's subscription logic.
func NopEditor() text.Editor {
	return &testEditor{}
}

// NopEditorWithCallback returns a text.Editor that calls cb when
// SubscribeEditorEvents is called.
func NopEditorWithCallback(cb func()) text.Editor {
	return &testEditor{cb: cb}
}

func (e *testEditor) Handle(ctx context.Context, ev textapi.Event) bool {
	e.dispatchEvent(ctx, ev)
	return false
}

func (e *testEditor) dispatchEvent(ctx context.Context, ev textapi.Event) {
	logrus.Infof("dispathing event %#v: subs=%v", ev, e.subs)
	if len(e.subs) == 0 {
		return
	}
	subs, ok := e.subs[ev.Type]
	if !ok {
		return
	}

	remain := make([]text.EventHandler, 0, len(subs))
	for _, sub := range subs {
		exit := sub.Handle(ctx, ev)
		if !exit {
			remain = append(remain, sub)
		}
	}
	e.subs[ev.Type] = remain
}

type TestEditorHandler struct {
	browsertest.TestHandler
	LocationList text.LocationList
	parent       *testEditor
	uri          workspaceapi.URI
}

func (e *TestEditorHandler) Resource() workspaceapi.URI {
	return e.uri
}

func (e *testEditor) Edit(resource workspaceapi.URI, buf *cell.Buffer) (text.Handler, error) {
	e.uri = resource
	e.buf = buf

	h := &TestEditorHandler{
		uri:         resource,
		parent:      e,
		TestHandler: *browsertest.NewTestHandler(),
	}
	e.dispatchEvent(context.Background(), textapi.Event{
		Type:     textapi.EventTypeOpen,
		URI:      resource,
		Resource: h,
		Content:  buf.String(),
	})

	subs := text.CellSubscriber(resource, h, e)
	buf.Subscribe(subs)
	return h, nil
}

func (e *testEditor) SetLocationList(
	h text.Handler, pri textapi.LocationPriority, id string, loc text.LocationList,
) error {
	h.(*TestEditorHandler).LocationList = loc
	return nil
}

func (e *TestEditorHandler) Handle(ev term.Event) (bool, bool) {
	e.parent.dispatchEvent(context.Background(), textapi.Event{
		Type:     textapi.EventTypeCursor,
		URI:      e.uri,
		Resource: e,
	})
	return e.TestHandler.Handle(ev)
}

func (e *testEditor) MoveToNextLocation(h text.Handler, ID string) error {
	return nil
}

func (e *testEditor) MoveToPrevLocation(h text.Handler, ID string) error {
	return nil
}

func (e *testEditor) SetCursor(h text.Handler, pos term.Coordinates) error {
	h.(*TestEditorHandler).CursorPos = pos
	return nil
}

func (e *testEditor) Cursor(h text.Handler) (term.Coordinates, error) {
	return h.(*TestEditorHandler).CursorPos, nil
}

func (e *testEditor) CellEditor(h text.Handler) text.CellEditor {
	return text.NewCellEditor(e.buf.Editor())
}

func (e *testEditor) CellView(h text.Handler) text.CellView {
	return text.NewCellView(e.buf.View())
}

func (e *testEditor) SubscribeCommand(cmd string, h text.CommandHandler) error {
	return nil
}

func (e *testEditor) SetDefaultAttributes(h text.Handler, attr term.Attributes) error {
	h.(*TestEditorHandler).Attributes.Fg = attr.Fg
	h.(*TestEditorHandler).Attributes.Bg = attr.Bg
	return nil
}

func (e *testEditor) SubscribeEditorEvents(
	evs []textapi.EventType, sub text.EventHandler,
) error {
	for _, ev := range evs {
		if e.subs == nil {
			e.subs = make(map[textapi.EventType][]text.EventHandler)
		}
		if _, ok := e.subs[ev]; !ok {
			e.subs[ev] = []text.EventHandler{sub}
		} else {
			e.subs[ev] = append(e.subs[ev], sub)
		}
	}
	if e.cb != nil {
		e.cb()
	}
	return nil
}

func (e *testEditor) Editor(file workspaceapi.URI) (text.Handler, error) {
	return nil, errors.New("nope")
}
