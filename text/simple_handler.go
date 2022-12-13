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
}

func newSimpleEditor(
	buf *cell.Buffer, resource workspaceapi.URI, wrap bool,
) *simpleEditorHandler {
	ret := new(simpleEditorHandler)
	ret.init(buf, resource, wrap)
	return ret
}

func (h *simpleEditorHandler) init(
	buf *cell.Buffer, resource workspaceapi.URI, wrap bool,
) {
	h.buf = buf
	h.resource = resource
	h.less.InitWithBuffer(buf, handler.LessConfig{
		Wrap: wrap,
		// TODO expose via configuration
		// Debug:   vi.config.debug,
		// ResAttr: vi.config.resAttr,
	})
	h.cursor.Init(h.less.Scroll())
}

// Resize satisfies tui.Component
func (h *simpleEditorHandler) Resize(width, height int) {
	h.less.Resize(width, height)
}

// Draw satisfies tui.Component
func (h *simpleEditorHandler) Draw(w term.Writer) {
	h.less.Draw(w)
	locs, _ := h.cursor.Locations()
	for _, loc := range locs {
		h.less.SetMessage(loc.Message)
		return
	}
}

func (h *simpleEditorHandler) Handle(ev term.Event) (exit, handled bool) {
	switch ev.Type {
	case term.EventMouse:
		/* TODO */
	default:
	}

	handled = true
	switch ev.Key {
	case term.KeyArrowLeft:
		if ev.Mod == term.ModAlt {
			h.cursor.MoveLeftStartWord()
		} else {
			h.cursor.MoveLeft()
		}
	case term.KeyArrowRight:
		if ev.Mod == term.ModAlt {
			h.cursor.MoveRightStartWord()
		} else {
			h.cursor.MoveRight()
		}
	case term.KeyArrowUp:
		h.cursor.MoveUp()
	case term.KeyArrowDown:
		h.cursor.MoveDown()
	case term.KeyEnter:
		h.cursor.Insert('\n')
	case term.KeySpace:
		h.cursor.Insert(' ')
	case term.KeyTab:
		h.cursor.Insert('\t')
	case term.KeyBackspace, term.KeyBackspace2:
		if h.cursor.Selection() != "" {
			handled = h.cursor.DeleteSelection()
		} else {
			handled = h.cursor.Backspace()
		}
	case term.KeyCtrlA:
		h.cursor.MoveStartLine()
	case term.KeyCtrlE:
		h.cursor.MoveEndLine()
	case term.KeyCtrlZ:
		h.cursor.Undo()
	case term.KeyCtrlR:
		h.cursor.Redo()
	case term.KeyCtrlF:
		ev.Key = 0
		ev.Ch = '/'
		h.less.Handle(ev)
	default:
		if ev.Ch != 0 {
			h.cursor.Insert(ev.Ch)
		} else {
			handled = false
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
