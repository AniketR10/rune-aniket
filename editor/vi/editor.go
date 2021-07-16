package vi

import (
	"errors"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/editor"
	"github.com/ernestrc/go-tui/term"
)

type viEditor struct {
	opts []Option
	subs map[editor.EventType][]editor.EventHandler
}

type viEditorCursorPublisher struct {
	parent *viEditor
	name   string
	tui.Handler
}

// Editor returns a Vi editor.Editor.
func Editor(opts ...Option) editor.Editor {
	subs := make(map[editor.EventType][]editor.EventHandler)
	return &viEditor{opts: opts, subs: subs}
}

func (e *viEditor) Handle(ev editor.Event) bool {
	e.dispatchEvent(ev)
	return false
}

func (e *viEditor) dispatchEvent(ev editor.Event) {
	subs, ok := e.subs[ev.Type]
	if !ok {
		return
	}

	remain := make([]editor.EventHandler, 0, len(subs))
	for _, sub := range subs {
		exit := sub.Handle(ev)
		if !exit {
			remain = append(remain, sub)
		}
	}
	e.subs[ev.Type] = remain
}

func (e *viEditor) Edit(name string, buf *cell.Buffer) (editor.Handler, error) {
	root := New(buf, e.opts...)
	h := &viEditorCursorPublisher{parent: e, name: name, Handler: root}

	e.dispatchEvent(editor.Event{
		Type:         editor.EventTypeOpen,
		ResourceName: name,
		Resource:     h,
		Content:      buf.String(),
	})

	// if vi is the final tui.Handler, then the underlying handler
	// is always in focus. Consumers of this editor.Editor should not
	// delegate SubscribeEditor to this handler if there's some other
	// focus mechanism in place.
	e.dispatchEvent(editor.Event{
		Type:         editor.EventTypeFocus,
		ResourceName: name,
		Resource:     h,
	})

	bsub := editor.CellSubscriber(name, h, e)
	buf.Subscribe(bsub)

	csub := editor.ScrollSubscriber(name, h, e)
	root.cursor.SubscribeScroll(csub)

	return h, nil
}

// SubscribeCommand is not supported.
func (e *viEditor) SubscribeCommand(cmd string, h editor.CommandHandler) error {
	return errors.New("not supported")
}

// Editor is not supported
func (e *viEditor) Editor(name string) (editor.Handler, error) {
	// NOTE: it would be dead code
	return nil, errors.New("not supported")
}

// SubscribeEditorEvents subsribes sub to ev. Note that this Editor is only capable
// of dispatching EventTypeOpen, EventTypeInsert and EventTypeDelete EventType events.
func (e *viEditor) SubscribeEditorEvents(ev editor.EventType, sub editor.EventHandler) error {
	if _, ok := e.subs[ev]; !ok {
		e.subs[ev] = []editor.EventHandler{sub}
		return nil
	}
	e.subs[ev] = append(e.subs[ev], sub)
	return nil
}

func getViFromHandler(h tui.Handler) *Vi {
	return h.(*viEditorCursorPublisher).Handler.(*Vi)
}

func (e viEditor) SetLocationList(h editor.Handler, ID string, loc editor.LocationList) error {
	getViFromHandler(h).SetLocationList(ID, loc)
	return nil
}

func (e *viEditor) MoveToNextLocation(h editor.Handler, ID string) error {
	getViFromHandler(h).MoveToNextLocation(ID)
	return nil
}

func (e *viEditor) MoveToPrevLocation(h editor.Handler, ID string) error {
	getViFromHandler(h).MoveToPrevLocation(ID)
	return nil
}

func (e *viEditor) Reader(h editor.Handler) editor.Reader {
	return editor.CellReader(getViFromHandler(h).less.Buffer().Reader())
}

func (e *viEditor) Writer(h editor.Handler) editor.Writer {
	return editor.CellWriter(getViFromHandler(h).less.Buffer().Writer())
}

func (e *viEditor) SetCursor(h editor.Handler, pos term.Coordinates) error {
	ok := getViFromHandler(h).SetCursorAtScroll(pos)
	if !ok {
		return errors.New("SetCursor: invalid cursor position")
	}
	return nil
}

func (e *viEditor) Cursor(h editor.Handler) (term.Coordinates, error) {
	pos := getViFromHandler(h).CursorAtScroll()
	return pos, nil
}

func (p *viEditorCursorPublisher) Handle(ev term.Event) (bool, bool) {
	cursor0, _ := p.Handler.Cursor()
	cursorAtScroll0 := p.Handler.(*Vi).cursor.CursorAtScroll()

	exit, handled := p.Handler.Handle(ev)
	cursor1, _ := p.Handler.Cursor()
	cursorAtScroll1 := p.Handler.(*Vi).cursor.CursorAtScroll()

	if cursor0 != cursor1 || cursorAtScroll0 != cursorAtScroll1 {
		p.parent.dispatchEvent(editor.Event{
			Type:         editor.EventTypeCursor,
			ResourceName: p.name,
			Resource:     p,
			Start:        cursor1,
			From:         cursorAtScroll1,
		})
	}
	return exit, handled
}
