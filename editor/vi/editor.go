package vi

import (
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/editor"
)

type viEditor struct {
	opts []Option
	subs map[editor.EventType][]editor.EventHandler
}

// Editor returns a Vi editor.Editor.
func Editor(opts ...Option) editor.Editor {
	subs := make(map[editor.EventType][]editor.EventHandler)
	return &viEditor{opts: opts, subs: subs}
}

func (e *viEditor) Edit(name string, buf *cell.Buffer) (editor.Handler, error) {
	h := New(buf, e.opts...)
	subs, ok := e.subs[editor.EventTypeOpen]
	if !ok {
		return h, nil
	}

	ev := editor.Event{
		Type:         editor.EventTypeOpen,
		ResourceName: name,
		Resource:     h,
	}
	remain := make([]editor.EventHandler, 0, len(subs))
	for _, sub := range subs {
		exit := sub.Handle(ev)
		if !exit {
			remain = append(remain, sub)
		}
	}
	e.subs[editor.EventTypeOpen] = remain
	return h, nil
}

// SubscribeEditor subsribes sub to ev. Note that this Editor is only capable
// of dispatching EventTypeOpen EventType events.
func (e *viEditor) SubscribeEditor(ev editor.EventType, sub editor.EventHandler) error {
	if _, ok := e.subs[ev]; !ok {
		e.subs[ev] = []editor.EventHandler{sub}
		return nil
	}
	e.subs[ev] = append(e.subs[ev], sub)
	return nil
}

func (e viEditor) SetLocationList(h editor.Handler, loc editor.LocationList) error {
	h.(*Vi).SetLocationList(loc)
	return nil
}

func (e *viEditor) Reader(h editor.Handler) editor.Reader {
	return editor.CellReader(h.(*Vi).less.Buffer().Reader())
}

func (e *viEditor) Writer(h editor.Handler) editor.Writer {
	return editor.CellWriter(h.(*Vi).less.Buffer().Writer())
}
