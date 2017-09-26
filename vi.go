package fractal

import (
	"fmt"

	"termbox"
)

type viMode uint8

const (
	normal viMode = iota
	insert
	visual
	visualLine
	visualBlock
)

type ViConfig struct {
	less LessConfig
	// TODO dispatch edit events with diffs
	Handler func(ViEvent) error
}

type ViEvent uint8

// Vi basic edit Handler and Component without ex commands
type Vi struct {
	Less
	mode   viMode
	cursor Coordinates
	config *ViConfig
	// TODO clipboard Clipboard
	// TODO history History
}

func (vi *Vi) setCursorResult() {
	if pos, ok := vi.Result(); ok {
		vi.cursor = Coordinates{
			X: pos.X - vi.offset.X,
			Y: pos.Y - vi.offset.Y,
		}
	}
}

func (vi *Vi) lessHandler(ev LessEvent) error {
	switch ev.Type {
	case EOF:
	case Search:
		vi.setCursorResult()
	}
	return nil
}

func DefaultViConfig() *ViConfig {
	lessConfig := DefaultLessConfig()
	return &ViConfig{
		less:    *lessConfig,
		Handler: nil,
	}
}

func NewViConfig(tabspaces int, wrap bool, handler func(ViEvent) error) *ViConfig {
	c := new(ViConfig)
	c.less.Tabspaces = tabspaces
	c.less.Wrap = wrap
	c.less.ResBG, c.less.ResFG = termbox.ColorDefault, termbox.AttrReverse
	return c
}

func (vi *Vi) setNormalMode() {
	vi.cmdScroll.Reset()
	vi.msgScroll.Reset()
	vi.msgScroll.Write("NORMAL")
	vi.mode = normal
}

func (vi *Vi) setInsertMode() {
	vi.msgScroll.Reset()
	vi.msgScroll.Write("INSERT")
	vi.mode = insert
}

func (vi *Vi) setVisualMode() {
	vi.msgScroll.Reset()
	vi.msgScroll.Write("VISUAL")
	vi.mode = visual
}

func (vi *Vi) setVisualLineMode() {
	vi.msgScroll.Reset()
	vi.msgScroll.Write("V-LINE")
	vi.mode = visualLine
}

func (vi *Vi) setVisualBlockMode() {
	vi.msgScroll.Reset()
	vi.msgScroll.Write("V-BLOCK")
	vi.mode = visualBlock
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
		case 'w':
			vi.MoveNextWord()
		case 'b':
			vi.MovePrevWord()
		case '/':
			return vi.Less.Handle(ev)
		case '%':
			switch vi.cellAtCursor().Ch {
			case '[':
				vi.moveMatchRune('[', ']', vi.MoveRightWrap)
			case '{':
				vi.moveMatchRune('{', '}', vi.MoveRightWrap)
			case '(':
				vi.moveMatchRune('(', ')', vi.MoveRightWrap)

			case ']':
				vi.moveMatchRune(']', '[', vi.MoveLeftWrap)
			case '}':
				vi.moveMatchRune('}', '{', vi.MoveLeftWrap)
			case ')':
				vi.moveMatchRune(')', '(', vi.MoveLeftWrap)
			default:
				vi.SetMessage(fmt.Sprintf("%+v", vi.cellAtCursor()))
			}
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

func NewVi(cfg *ViConfig) *Vi {
	vi := new(Vi)
	vi.Init(cfg)
	return vi
}

func (vi *Vi) Init(cfg *ViConfig) {
	if cfg == nil {
		cfg = DefaultViConfig()
	}
	vi.config = cfg
	vi.config.less.Handler = vi.lessHandler
	vi.Less.Init(&vi.config.less)
	vi.setNormalMode()
}

func (vi *Vi) InitWithScroll(scroll *Scroll, cfg *ViConfig) {
	if cfg == nil {
		cfg = DefaultViConfig()
	}
	vi.config = cfg
	vi.config.less.Handler = vi.lessHandler
	vi.Less.InitWithScroll(scroll, &vi.config.less)
	vi.setNormalMode()
}

func (vi *Vi) SetScroll(scroll *Scroll) (orig *Scroll) {
	vi.setNormalMode()
	return vi.Less.SetScroll(scroll)
}

func (vi *Vi) Handle(ev termbox.Event) (bool, error) {
	switch vi.mode {
	case normal:
		if vi.Less.mode != normalMode {
			return vi.Less.Handle(ev)
		}
		return vi.handleNormal(ev)
	case insert:
		return vi.handleInsert(ev)
	case visual:
		return vi.handleVisual(ev)
	case visualLine:
		return vi.handleVisualLine(ev)
	case visualBlock:
		return vi.handleVisualBlock(ev)
	default:
		panic(fmt.Sprintf("unknown mode: %d", vi.mode))
	}
}

func (vi *Vi) moveMatchRune(target, match rune, move func() bool) {
	curr := vi.cursor
	pending := 1
	var c Cell
	for pending != 0 && move() {
		c = vi.cellAtCursor()
		switch c.Ch {
		case target:
			pending++
		case match:
			pending--
		}
	}

	if pending == 0 {
		vi.cursor.X = c.Coordinates.X - vi.offset.X
		vi.cursor.Y = c.Coordinates.Y - vi.offset.Y
	} else {
		vi.cursor = curr
	}
}

func (vi *Vi) moveBeforeRune(t []rune, move func() bool) {
	const (
		skipRune = iota
		findRune
		done
	)

	state := skipRune
	c := vi.cellAtCursor()
	prev := c

	for state != done && move() {
		prev = c
		c = vi.cellAtCursor()

		switch state {
		case skipRune:
			none := true
			for _, r := range t {
				none = none && c.Ch != r
			}
			if none {
				state = findRune
			}
		case findRune:
			for _, r := range t {
				if c.Ch == r {
					state = done
					break
				}
			}
		}
	}

	vi.cursor.X = prev.Coordinates.X - vi.offset.X
	vi.cursor.Y = prev.Coordinates.Y - vi.offset.Y
}

func (vi *Vi) moveAfterRune(t []rune, move func() bool) {
	const (
		init = iota
		foundRune
	)

	var c Cell
	state := init

	for move() {
		c = vi.cellAtCursor()

		switch state {
		case init:
			for _, r := range t {
				if r == c.Ch {
					state = foundRune
					break
				}
			}
		case foundRune:
			none := true
			for _, r := range t {
				none = none && r != c.Ch
			}
			if none {
				return
			}
		}
	}
}

func (vi *Vi) MoveNextWord() {
	vi.moveAfterRune([]rune{' ', '\n', '\t'}, vi.MoveRightWrap)
}

func (vi *Vi) MovePrevWord() {
	vi.moveBeforeRune([]rune{' ', '\n', '\t'}, vi.MoveLeftWrap)
}

// MoveLeftWrap will move the cursor to the left or wrap to end of
// previous line if cursor is at X=0
func (vi *Vi) MoveLeftWrap() bool {
	curr := vi.cursor
	vi.MoveLeft()

	if curr.X == vi.cursor.X {
		vi.MoveUp()
		if curr.Y == vi.cursor.Y {
			return false
		}
		vi.MoveEndLine()
	}
	return true
}

// MoveRightWrap will move the cursor to the right or wrap to beginning
// of next line if cursor is at X=EOL
func (vi *Vi) MoveRightWrap() bool {
	curr := vi.cursor
	vi.MoveRight()

	if curr.X == vi.cursor.X {
		vi.MoveDown()
		if curr.Y == vi.cursor.Y {
			return false
		}
		vi.MoveStartLine()
	}

	return true
}

func (vi *Vi) MovePrevResult() {
	vi.SeekPrevResult()
	vi.setCursorResult()
}

func (vi *Vi) MoveNextResult() {
	vi.SeekNextResult()
	vi.setCursorResult()
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
	cursor := Coordinates{
		X: vi.offset.X + vi.cursor.X,
		Y: vi.offset.Y + vi.cursor.Y,
	}
	if i := vi.lastIdxCursorRow(); vi.cursor.X > i {
		cursor.X = i
	}
	return vi.cells[cursor.X+cursor.Y*vi.columns]
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
	} else {
		vi.SeekUp()
	}
}

func (vi *Vi) MoveDown() {
	if vi.cursor.Y < vi.height-2 { // account for command line
		vi.cursor.Y++
	} else {
		vi.SeekDown()
	}
}

func (vi *Vi) MoveLeft() {
	if vi.cursor.X > 0 {
		vi.cursor.X--
	} else {
		vi.SeekLeft()
	}
}

func (vi *Vi) MoveRight() {
	i := vi.lastIdxCursorRow()
	if vi.cursor.X < vi.width-1 && vi.cursor.X < i {
		vi.cursor.X++
	} else if vi.cursor.X < i {
		vi.SeekRight()
	}
}

func (vi *Vi) isCursorAtNull() bool {
	if len(vi.cells) == 0 {
		return false
	}
	c := vi.cellAtCursor().Ch

	return c == '\t' || c == 0
}

// GetCursor returns the cursor coordinates relative to the current
// fractal global positioning
func (vi *Vi) GetCursor() Coordinates {
	if vi.Less.mode != normalMode {
		return vi.Less.GetCursor()
	}
	x, y := vi.Position()
	cursor := vi.cursor

	if i := vi.lastIdxCursorRow(); vi.cursor.X > i {
		cursor.X = i
	}

	for vi.isCursorAtNull() {
		if !vi.MoveRightWrap() {
			break
		}
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

func (vi *Vi) Draw(w Writer) error {
	c := vi.cellAtCursor()
	vi.SetMessage("%d,%d %c", c.X, c.Y, c.Ch)
	return vi.Less.Draw(w)
}
