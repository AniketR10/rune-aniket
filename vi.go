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
	// TODO install search handler to position cursor
	Less
	mode   ViMode
	cursor Coordinates
	// TODO ViEventHandler to dispatch edits
	// TODO clipboard Clipboard
	// TODO history History
}

func (vi *Vi) setNormalMode() {
	vi.cmdScroll.Reset()
	vi.msgScroll.Reset()
	vi.msgScroll.Write("NORMAL")
	vi.mode = Normal
}

func (vi *Vi) setInsertMode() {
	vi.msgScroll.Reset()
	vi.msgScroll.Write("INSERT")
	vi.mode = Insert
}

func (vi *Vi) setVisualMode() {
	vi.msgScroll.Reset()
	vi.msgScroll.Write("VISUAL")
	vi.mode = Visual
}
func (vi *Vi) setVisualLineMode() {
	vi.msgScroll.Reset()
	vi.msgScroll.Write("V-LINE")
	vi.mode = VisualLine
}
func (vi *Vi) setVisualBlockMode() {
	vi.msgScroll.Reset()
	vi.msgScroll.Write("V-BLOCK")
	vi.mode = VisualBlock
}

func (vi *Vi) handleNormal(ev termbox.Event) (bool, error) {
	switch ev.Type {
	case termbox.EventKey:
		switch ev.Ch {
		case 'q':
			return true, nil
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
		case 'O':
			vi.MoveUp()
			vi.setInsertMode()
		case 'o':
			vi.MoveDown()
			vi.setInsertMode()
		case 'i':
			vi.setInsertMode()
		case 'I':
			vi.MoveStartLine()
			vi.setInsertMode()
		case 'a':
			vi.MoveRight()
			vi.setInsertMode()
		case 'A':
			vi.MoveEndLine()
			vi.setInsertMode()
		case 'x':
			vi.TruncateAt(vi.cursor)
		case 's':
			vi.TruncateAt(vi.cursor)
			vi.setInsertMode()
		case 'v':
			vi.setVisualMode()
		case '/':
			return vi.Less.Handle(ev)
		}
	}

	return false, nil
}

func (vi *Vi) handleInsert(ev termbox.Event) (bool, error) {
	switch ev.Key {
	case termbox.KeyEsc:
		vi.setNormalMode()
	}
	return false, nil
}

func (vi *Vi) handleVisual(ev termbox.Event) (bool, error) {
	switch ev.Key {
	case termbox.KeyEsc:
		vi.setNormalMode()
	}
	return false, nil
}

func (vi *Vi) handleVisualLine(ev termbox.Event) (bool, error) {
	switch ev.Key {
	case termbox.KeyEsc:
		vi.setNormalMode()
	}
	return false, nil
}

func (vi *Vi) handleVisualBlock(ev termbox.Event) (bool, error) {
	switch ev.Key {
	case termbox.KeyEsc:
		vi.setNormalMode()
	}
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
		if vi.Less.mode != normalMode {
			return vi.Less.Handle(ev)
		}
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
}

func (vi *Vi) MoveNextResult() {

}

func (vi *Vi) MoveStartLine() {
	vi.cursor.X = 0
	if vi.offset.X > 0 {
		vi.SeekStartLine()
	}
}

func (vi *Vi) translatedCursor() Coordinates {
	return Coordinates{
		X: vi.offset.X + vi.cursor.X,
		Y: vi.offset.Y + vi.cursor.Y,
	}
}

// lastIdxCursorRow returns the last legal cursor X position on the current row
func (vi *Vi) lastIdxCursorRow() int {
	cursor := vi.translatedCursor()
	return vi.Less.RowLastIdx(cursor.Y) - vi.offset.X
}

func (vi *Vi) cellAtCursor() Cell {
	cursor := vi.translatedCursor()
	return vi.cells[cursor.X+cursor.Y*vi.columns]
}

func (vi *Vi) isCursorAtNull() bool {
	return vi.cellAtCursor().Ch == 0
}

func (vi *Vi) MoveEndLine() {
	vi.cursor.X = vi.width - 1
	i := vi.lastIdxCursorRow()
	if i < vi.cursor.X {
		vi.cursor.X = i
	} else if i > vi.cursor.X {
		vi.SeekEndLine()
	}
}

func (vi *Vi) MoveStartFile() {
	vi.cursor.X, vi.cursor.Y = 0, 0
	vi.SeekStartFile()
}

func (vi *Vi) MoveEndFile() {
	vi.cursor.Y = vi.height - 2
	vi.SeekEndFile()
}

func (vi *Vi) MoveUp() {
	if vi.cursor.Y > 0 {
		vi.cursor.Y--
		if vi.isCursorAtNull() {
			vi.MoveRight()
		}
	} else {
		vi.SeekUp()
	}
}

func (vi *Vi) MoveDown() {
	if vi.cursor.Y < vi.height-2 { // account for command line
		vi.cursor.Y++
		if vi.isCursorAtNull() {
			vi.MoveRight()
		}
	} else {
		vi.SeekDown()
	}
}

func (vi *Vi) MoveLeft() {
	if vi.cursor.X > 0 {
		vi.cursor.X--
		if vi.isCursorAtNull() {
			vi.MoveLeft()
		}
	} else {
		vi.SeekLeft()
	}
}

func (vi *Vi) MoveRight() {
	i := vi.lastIdxCursorRow()
	if vi.cursor.X < vi.width-1 && vi.cursor.X < i {
		vi.cursor.X++
		if vi.isCursorAtNull() {
			vi.MoveRight()
		}
	} else if vi.cursor.X < i {
		vi.SeekRight()
	}
}

func (vi *Vi) GetCursor() Coordinates {
	if vi.Less.mode != normalMode {
		return vi.Less.GetCursor()
	}
	x, y := vi.Position()
	cursor := vi.cursor

	if i := vi.lastIdxCursorRow(); vi.cursor.X > i {
		cursor.X = i
	}

	return Coordinates{
		X: x + cursor.X,
		Y: y + cursor.Y,
	}
}

func (vi *Vi) Resize(width, height int) error {
	if err := vi.Less.Resize(width, height); err != nil {
		return err
	}

	vi.MoveLeft()
	vi.MoveRight()
	vi.MoveUp()
	vi.MoveDown()

	return nil
}
