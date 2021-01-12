package vi

import (
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/editor"
)

type viEditor struct {
	opts []Option
}

// Editor returns a Vi editor.Editor.
func Editor(opts ...Option) editor.Editor {
	return &viEditor{opts: opts}
}

func (e *viEditor) Edit(name string, buf *cell.Buffer) (editor.Handler, error) {
	return New(buf, e.opts...), nil
}

/*
func (e *viEditor) Edit(name string, buf *cell.Buffer) (tui.Handler, error) {
	h := New(buf, e.opts...)
	if subs, ok := e.subs[editor.EventTypeOpen]; ok {
		ev := editor.Event{
			Type:            editor.EventTypeOpen,
			ResourceName:    name,
			ResourceHandler: h,
		}
		for _, sub := range subs {
			sub.Handle(ev)
		}
	}
	return h, nil
}

func (e *viEditor) Subscribe(ev editor.EventType, sub editor.Subscriber) error {
	if _, ok := e.subs[ev]; !ok {
		e.subs[ev] = []editor.Subscriber{sub}
		return nil
	}
	e.subs[ev] = append(e.subs[ev], sub)
	return nil
}

func (e viEditor) SetLocationList(h tui.Handler, loc editor.LocationList) error {
	h.(*Vi).SetLocationList(loc)
	return nil
}

func (e *viEditor) Reader(h tui.Handler) (cell.Reader, error) {
	return h.(*Vi).less.Buffer().Reader(), nil
}

func (e *viEditor) Writer(h tui.Handler) (cell.Writer, error) {
	return h.(*Vi).less.Buffer().Writer(), nil
}
*/
