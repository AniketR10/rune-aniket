package handler

import (
	"fmt"

	"termbox"

	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/component"
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
	component.Scroll
	mode   ViMode
	cursor cursorHelper
	// TODO clipboard fractal.Clipboard
}

func (vi *Vi) SetNormalMode() {
	vi.mode = Normal
}

func (vi *Vi) SetInsertMode() {
	vi.mode = Insert
}

func (vi *Vi) SetVisualMode() {
	vi.mode = Visual
}
func (vi *Vi) SetVisualLineMode() {
	vi.mode = VisualLine
}
func (vi *Vi) SetVisualBlockMode() {
	vi.mode = VisualBlock
}

func (vi *Vi) handleNormal(ev *termbox.Event) (bool, error) {
	switch ev.Type {
	case termbox.EventKey:
		switch ev.Key {
		case termbox.KeyArrowDown:
			vi.MoveDown()
		case termbox.KeyArrowUp:
			vi.MoveUp()
		case termbox.KeyArrowLeft:
			vi.MoveLeft()
		case termbox.KeyArrowRight:
			vi.MoveRight()
		default:
			switch ev.Ch {
			case 'O':
				vi.MoveUp()
				vi.SetInsertMode()
			case 'o':
				vi.MoveDown()
				vi.SetInsertMode()
			case 'i':
				vi.SetInsertMode()
			case 'I':
				vi.MoveStartLine()
				vi.SetInsertMode()
			case 'a':
				vi.MoveRight()
				vi.SetInsertMode()
			case 'A':
				vi.MoveEndLine()
				vi.SetInsertMode()
			case 'x':
				// vi.RemoveChar(vi.cursor.X, vi.cursor.Y)
			case 's':
				// vi.RemoveChar(vi.cursor.X, vi.cursor.Y)
				vi.SetInsertMode()
			case 'N':
				vi.MovePrevResult()
			case 'n':
				vi.MoveNextResult()
			case '0':
				vi.MoveStartLine()
			case '$':
				vi.MoveEndLine()
			case 'g':
				vi.MoveStartFile()
			case 'G':
				vi.MoveEndFile()
			case 'j':
				vi.MoveDown()
			case 'k':
				vi.MoveUp()
			case 'h':
				vi.MoveLeft()
			case 'l':
				vi.MoveRight()
			}
		}
	}

	return false, nil
}

func (vi *Vi) handleInsert(ev *termbox.Event) (bool, error) {
	return false, nil
}

func (vi *Vi) handleVisual(ev *termbox.Event) (bool, error) {
	return false, nil
}

func (vi *Vi) handleVisualLine(ev *termbox.Event) (bool, error) {
	return false, nil
}

func (vi *Vi) handleVisualBlock(ev *termbox.Event) (bool, error) {
	return false, nil
}

func NewVi(buf *fractal.Buffer, width, height, x, y int) *Vi {
	vi := new(Vi)
	vi.Init(buf, width, height, x, y)
	return vi
}

func (vi *Vi) Init(buf *fractal.Buffer, width, height, x, y int) {
	vi.Scroll.Init(buf, width, height, x, y)
	vi.SetNormalMode()
}

func (vi *Vi) SetBuffer(buf *fractal.Buffer) *fractal.Buffer {
	vi.SetNormalMode()
	return vi.Scroll.SetBuffer(buf)
}

func (vi *Vi) Handle(ev *termbox.Event) (bool, error) {
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

func (vi *Vi) MovePrevResult() {
	vi.SeekPrevResult()
}

func (vi *Vi) MoveNextResult() {

}

func (vi *Vi) MoveStartLine() {

}

func (vi *Vi) MoveEndLine() {

}

func (vi *Vi) MoveStartFile() {

}

func (vi *Vi) MoveEndFile() {

}

func (vi *Vi) MoveDown() {
	// _, y := vi.Position()
	// if vi.cursor.Y == y+vi.Height()-1 {
	// 	vi.SeekDown()
	// } else {
	// 	vi.cursor.moveDown(vi.GetView())
	// }
}

func (vi *Vi) MoveUp() {
	// _, y := vi.Position()
	// if vi.cursor.Y == y {
	// 	vi.SeekUp()
	// } else {
	// 	vi.cursor.moveUp(vi.GetView())
	// }
}

func (vi *Vi) MoveLeft() {

}

func (vi *Vi) MoveRight() {

}

func (vi *Vi) RemoveChar(x, y int) {

}
