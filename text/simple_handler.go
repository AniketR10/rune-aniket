package text

import (
	"unstable.build/go-tui"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/term"
)

type simpleEditorHandler struct {
	buf      *cell.Buffer
	less     handler.Less
	resource workspaceapi.URI
	cursor   Cursor
	height   int
	mouse    *Mouse
}

// NewSimpleHandler returns a modeless, simple-to-use text.Handler.
func NewSimpleHandler(
	buf *cell.Buffer, resource workspaceapi.URI,
	wrap, commandBar bool,
) Handler {
	ret := new(simpleEditorHandler)
	ret.Init(buf, resource, wrap, commandBar)
	return ret
}

func (h *simpleEditorHandler) Init(
	buf *cell.Buffer, resource workspaceapi.URI,
	wrap, commandBar bool,
) {
	h.buf = buf
	h.resource = resource
	h.less.InitWithBuffer(buf, handler.LessConfig{
		Wrap:  wrap,
		NoBar: !commandBar,
	})
	h.cursor.Init(h.less.Scroll())
	h.mouse = NewMouse(CursorMouseDelegate(&h.cursor))
}

// Resize satisfies tui.Component
func (h *simpleEditorHandler) Resize(width, height int) {
	h.height = height
	h.less.Resize(width, height)

	// scroll offset might > max new offset after resize
	// this must be done here because cursor doesn't
	// have a hook on Resize, and scroll cannot
	// have access to a cursor.
	pos := h.cursor.CursorAtScroll()
	h.less.Scroll().SeekTo(h.less.Scroll().Offset())
	h.cursor.MoveToScroll(pos)
}

// Draw satisfies tui.Component
func (h *simpleEditorHandler) Draw(w term.Writer) {
	h.less.Draw(w)
	locs, _ := h.cursor.Locations()
	for _, loc := range locs {
		h.less.Notify(loc.Message)
		return
	}
}

func (h *simpleEditorHandler) Handle(ev term.Event) (exit, handled bool) {
	if ev.Type == term.EventMouse {
		return h.mouse.Handle(ev)
	}

	switch ev.Key {
	case term.KeyArrowLeft:
		if ev.Mod == term.ModAlt {
			handled = h.cursor.MoveLeftStartWord()
		} else {
			handled = h.cursor.MoveLeft()
		}
	case term.KeyArrowRight:
		if ev.Mod == term.ModAlt {
			handled = h.cursor.MoveRightStartWord()
		} else {
			handled = h.cursor.MoveRight()
		}
	case term.KeyArrowUp:
		handled = h.cursor.MoveUp()
	case term.KeyArrowDown:
		handled = h.cursor.MoveDown()
	case term.KeyEnter:
		h.cursor.Insert('\n')
		handled = true
	case term.KeySpace:
		h.cursor.Insert(' ')
		handled = true
	case term.KeyTab:
		h.cursor.Insert('\t')
		handled = true
	case term.KeyBackspace, term.KeyBackspace2:
		if h.cursor.Selection() != "" {
			handled = h.cursor.DeleteSelection()
		} else {
			handled = h.cursor.Backspace()
		}
	case term.KeyCtrlA:
		handled = h.cursor.MoveStartLine()
	case term.KeyCtrlE:
		handled = h.cursor.MoveEndLine()
	case term.KeyCtrlZ:
		handled = h.cursor.Undo()
	case term.KeyCtrlR:
		handled = h.cursor.Redo()
	case term.KeyCtrlF:
		ev.Key = 0
		ev.Ch = '/'
		_, handled = h.less.Handle(ev)
	default:
		if ev.Ch != 0 {
			h.cursor.Insert(ev.Ch)
			handled = true
		}
	}
	return
}

// Cursor satisfies tui.Handler
func (h *simpleEditorHandler) Cursor() (pos term.Coordinates, show bool) {
	return h.cursor.Coordinates(), true
}

// Man satisfies tui.Handler
func (h *simpleEditorHandler) Man() tui.Manual {
	return tui.Manual{}
}

// Close satisfies editor.Handler.
func (h *simpleEditorHandler) Close() error {
	return nil
}

// Resource satisfies editor.Handler.
func (h *simpleEditorHandler) Resource() workspaceapi.URI {
	return h.resource
}

// SetWrap satisfies editor.Handler.
func (h *simpleEditorHandler) SetWrap(wrap bool) {
	h.less.Scroll().Wrap = wrap
}
func (t *simpleEditorHandler) ShowCommandBar(show bool) {
	t.less.ShowCommandBar(show)
}
