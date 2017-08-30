package fractal

import (
	"fmt"

	"termbox"
)

type ViMode uint8

const (
	Normal ViMode = iota
	Insert
	Visual
	VisualLine
	VisualBlock
)

// Vi basic edit window
type Vi struct {
	Less
	mode   ViMode
	cursor cursorHelper
	// TODO clipboard Clipboard
}

func (vi *Vi) setNormalMode() {
	vi.mode = Normal
}

func (vi *Vi) setInsertMode() {
	vi.mode = Insert
}

func (vi *Vi) setVisualMode() {
	vi.mode = Visual
}
func (vi *Vi) setVisualLineMode() {
	vi.mode = VisualLine
}
func (vi *Vi) setVisualBlockMode() {
	vi.mode = VisualBlock
}

func (vi *Vi) handleNormal(ev termbox.Event) (bool, error) {
	switch ev.Type {
	case termbox.EventKey:
		switch ev.Ch {
		case 'O':
			// vi.SeekUp()
			vi.setInsertMode()
		case 'o':
			// vi.SeekDown()
			vi.setInsertMode()
		case 'i':
			vi.setInsertMode()
		case 'I':
			// vi.SeekStartLine()
			vi.setInsertMode()
		case 'a':
			// vi.SeekRight()
			vi.setInsertMode()
		case 'A':
			// vi.SeekEndLine()
			vi.setInsertMode()
		case 'x':
			// TODO vi.RemoveChar(vi.cursor.X, vi.cursor.Y)
			vi.setInsertMode()
		case 's':
			// TODO vi.RemoveChar(vi.cursor.X, vi.cursor.Y)
			vi.setInsertMode()
		}
	default:
		return vi.Less.Handle(ev)
	}

	return false, nil
}

func (vi *Vi) handleInsert(ev termbox.Event) (bool, error) {
	return false, nil
}

func (vi *Vi) handleVisual(ev termbox.Event) (bool, error) {
	return false, nil
}

func (vi *Vi) handleVisualLine(ev termbox.Event) (bool, error) {
	return false, nil
}

func (vi *Vi) handleVisualBlock(ev termbox.Event) (bool, error) {
	return false, nil
}

func NewVi() *Vi {
	vi := new(Vi)
	vi.Init()
	return vi
}

func (vi *Vi) Init() {
	vi.Less.Init(nil)
	vi.setNormalMode()
}

func (vi *Vi) InitWithScroll(scroll *Scroll) {
	vi.Less.InitWithScroll(scroll, nil)
	vi.setNormalMode()
}

func (vi *Vi) SetScroll(scroll *Scroll) (orig *Scroll) {
	vi.setNormalMode()
	return vi.Less.SetScroll(scroll)
}

func (vi *Vi) Handle(ev termbox.Event) (bool, error) {
	switch vi.mode {
	case Normal:
		return vi.handleNormal(ev)
	case Insert:
		return vi.handleInsert(ev)
	case Visual:
		return vi.handleVisual(ev)
	case VisualLine:
		return vi.handleVisualLine(ev)
	case VisualBlock:
		return vi.handleVisualBlock(ev)
	default:
		panic(fmt.Sprintf("unknown mode: %d", vi.mode))
	}
}
