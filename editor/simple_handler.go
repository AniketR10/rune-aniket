package editor

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/term"
)

type simpleEditorHandler struct {
	buf    *cell.Buffer
	less   handler.Less
	cursor Cursor
}

func newSimpleEditor(buf *cell.Buffer) *simpleEditorHandler {
	ret := new(simpleEditorHandler)
	ret.init(buf)
	return ret
}

func (h *simpleEditorHandler) init(buf *cell.Buffer) {
	h.buf = buf
	h.less.InitWithBuffer(buf, handler.LessConfig{
		Wrap: true,
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
			h.cursor.DeleteSelection()
		} else {
			h.cursor.Backspace()
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
