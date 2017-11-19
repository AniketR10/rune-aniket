package fractal

import (
	"fmt"
	"math"

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
	less Less
	mode viMode
	// cursor Coordinates relative to the position of the component in the screen
	// GetCursor method returns the absolute coordinates
	cursor Coordinates
	config *ViConfig
	// TODO clipboard Clipboard
}

func (vi *Vi) skipNulls() {
	c := vi.cellAtCursor()
	for c.Ch == 0 && vi.moveRight() {
		c = vi.cellAtCursor()
	}
	for c.Ch == 0 && vi.less.SeekRight() {
		c = vi.cellAtCursor()
	}
	for c.Ch == 0 && vi.moveLeft() {
		c = vi.cellAtCursor()
	}
	for c.Ch == 0 && vi.less.SeekLeft() {
		c = vi.cellAtCursor()
	}
}

func (vi *Vi) getMaxCursorY() int {
	hardl := vi.less.height - 2
	textl := len(vi.less.cells) - vi.less.offset.Y - 1
	return int(math.Min(float64(hardl), float64(textl)))
}

// SetCursor sets the cursor position.
// Coordinates is parsed as the desired position relative to the component position
func (vi *Vi) SetCursor(c Coordinates) {
	if c.X < 0 {
		vi.cursor.X = 0
	} else if max := vi.less.width - 1; c.X > max { // width starts at 1
		vi.cursor.X = max
	} else {
		vi.cursor.X = c.X
	}

	if c.Y < 0 {
		vi.cursor.Y = 0
	} else if max := vi.getMaxCursorY(); c.Y > max { // to account for command line
		vi.cursor.Y = max
	} else {
		vi.cursor.Y = c.Y
	}

	vi.skipNulls()
}

func (vi *Vi) setCursorResult() {
	if pos, ok := vi.less.Result(); ok {
		vi.SetCursor(Coordinates{
			X: pos.X - vi.less.offset.X,
			Y: pos.Y - vi.less.offset.Y,
		})
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
	vi.less.cmdScroll.Reset()
	vi.less.msgScroll.Reset()
	vi.less.msgScroll.WriteStr("NORMAL")
	vi.mode = normal
}

func (vi *Vi) setInsertMode() {
	vi.less.msgScroll.Reset()
	vi.less.msgScroll.WriteStr("INSERT")
	vi.mode = insert
}

func (vi *Vi) setVisualMode() {
	vi.less.msgScroll.Reset()
	vi.less.msgScroll.WriteStr("VISUAL")
	vi.mode = visual
}

func (vi *Vi) setVisualLineMode() {
	vi.less.msgScroll.Reset()
	vi.less.msgScroll.WriteStr("V-LINE")
	vi.mode = visualLine
}

func (vi *Vi) setVisualBlockMode() {
	vi.less.msgScroll.Reset()
	vi.less.msgScroll.WriteStr("V-BLOCK")
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
			vi.TruncateCell()
		case 's':
			vi.TruncateCell()
			vi.setInsertMode()
		case 'v':
			vi.setVisualMode()
		case 'w':
			vi.MoveRightStartWord()
		case 'b':
			vi.MoveLeftStartWord()
		case '/':
			return vi.less.Handle(ev)
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
			}
		}
	}

	return false, nil
}

// sets cursor with coordinates relative to buffer
func (vi *Vi) setFromScrollPos(pos Coordinates) {
	vi.cursor = Coordinates{
		X: pos.X - vi.less.offset.X,
		Y: pos.Y - vi.less.offset.Y,
	}
}

func (vi *Vi) TruncateCell() (orig termbox.Cell, ok bool) {
	pos := vi.getCursorAtBuffer()
	var next Coordinates
	if next, _, ok = vi.less.TruncateCellAt(pos); ok {
		vi.setFromScrollPos(next)
	}
	return
}

// Insert will insert rune at the current cursor's position
func (vi *Vi) Insert(r rune) {
	vi.setFromScrollPos(vi.less.InsertAt(vi.getCursorAtBuffer(), r))
}

// Write will write the given string at the end of the buffer
func (vi *Vi) Write(str string) {
	vi.less.WriteStr(str)
}

// Clear clears the buffer
func (vi *Vi) Clear() {
	vi.less.TruncateFrom(Coordinates{X: 0, Y: 0})
}

func (vi *Vi) handleInsert(ev termbox.Event) (bool, error) {
	cursor := vi.getCursorAtBuffer()
	switch ev.Key {
	case termbox.KeyEnter:
		vi.Insert('\n')
	case termbox.KeySpace:
		vi.Insert(' ')
	case termbox.KeyTab:
		vi.Insert('\t')
	case termbox.KeyBackspace, termbox.KeyBackspace2:
		if cursor.X > 0 {
			vi.less.TruncateCellAt(Coordinates{cursor.X - 1, cursor.Y})
		} else if cursor.Y > 0 {
			y := cursor.Y - 1
			i, ok := vi.less.RowLastIdx(y)
			l := len(vi.less.cells[y])
			vi.less.ConflateRow(y)
			if ok {
				x := i
				if l > 0 {
					x++
				}
				vi.setFromScrollPos(Coordinates{X: x, Y: y})
			}
		}
	case termbox.KeyEsc:
		vi.MoveLeft()
		vi.setNormalMode()
	default:
		if ev.Ch != 0 {
			vi.Insert(ev.Ch)
		}
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
	vi.less.Init(&vi.config.less)
	vi.setNormalMode()
}

func (vi *Vi) InitWithScroll(scroll *Scroll, cfg *ViConfig) {
	if cfg == nil {
		cfg = DefaultViConfig()
	}
	vi.config = cfg
	vi.config.less.Handler = vi.lessHandler
	vi.less.InitWithScroll(scroll, &vi.config.less)
	vi.setNormalMode()
}

func (vi *Vi) SetScroll(scroll *Scroll) (orig *Scroll) {
	vi.setNormalMode()
	return vi.less.SetScroll(scroll)
}

func (vi *Vi) moveMatchRune(target, match rune, move func() bool) {
	currc, curro := vi.cursor, vi.less.offset
	pending := 1
	var prev Coordinates
	for pending != 0 && move() {
		prev = vi.cursor
		switch vi.cellAtCursor().Ch {
		case target:
			pending++
		case match:
			pending--
		}
	}

	if pending == 0 {
		vi.cursor = prev
	} else {
		vi.less.offset = curro
		vi.cursor = currc
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
	var prev Coordinates
	for move() {
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
		if state == done {
			break
		}
		prev = vi.cursor
	}

	vi.cursor = prev
}

func (vi *Vi) moveAfterRune(t []rune, move func() bool) {
	const (
		init = iota
		foundRune
	)

	var c termbox.Cell
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

func (vi *Vi) MoveRightStartWord() {
	vi.moveAfterRune([]rune{' ', '\n', '\t', 0}, vi.MoveRightWrap)
}

func (vi *Vi) MoveLeftStartWord() {
	vi.moveBeforeRune([]rune{' ', '\n', '\t', 0}, vi.MoveLeftWrap)
}

// MoveLeftWrap will move the cursor to the left or wrap to end of
// previous line if cursor is at X=0
func (vi *Vi) MoveLeftWrap() bool {
	// we cannot use 0 as a wrap coordinate because what we really want
	// is to return false when we cannot move which depends on the type of move.
	// Returing false when reached 0, in the case of moving one cell at a time is correct
	// but not when we jump from start of word to the next
	currc, curro := vi.cursor, vi.less.offset
	vi.MoveLeft()
	if vi.cursor.X == currc.X && vi.less.offset.X == curro.X {
		vi.MoveUp()
		if vi.cursor.Y == currc.Y && vi.less.offset.Y == curro.Y {
			return false
		}
		vi.MoveEndLine()
	}
	return true
}

// MoveRightWrap will move the cursor to the right or wrap to beginning
// of next line if cursor is at X=EOL
func (vi *Vi) MoveRightWrap() bool {
	currc, curro := vi.cursor, vi.less.offset
	vi.MoveRight()

	if currc.X == vi.cursor.X && vi.less.offset.X == curro.X {
		vi.MoveDown()
		if currc.Y == vi.cursor.Y && vi.less.offset.Y == curro.Y {
			return false
		}
		vi.MoveStartLine()
	}

	return true
}

func (vi *Vi) MovePrevResult() {
	vi.less.SeekPrevResult()
	vi.setCursorResult()
}

func (vi *Vi) MoveNextResult() {
	vi.less.SeekNextResult()
	vi.setCursorResult()
}

func (vi *Vi) MoveStartLine() {
	vi.SetCursor(Coordinates{X: 0, Y: vi.cursor.Y})
	if vi.less.offset.X > 0 {
		vi.less.SeekStartLine()
	}
	vi.skipNulls()
}

func (vi *Vi) getCursorAtBuffer() Coordinates {
	cursor := vi.cursor
	c := Coordinates{
		X: vi.less.offset.X + cursor.X,
		Y: vi.less.offset.Y + cursor.Y,
	}
	return c
}

// lastIdxCursorRow returns the last legal cursor X position on the current row
func (vi *Vi) lastIdxCursorRow() int {
	if i, ok := vi.less.RowLastIdx(vi.cursor.Y + vi.less.offset.Y); ok {
		return i - vi.less.offset.X
	}
	return 0
}

func (vi *Vi) cellAtCursor() (cell termbox.Cell) {
	cursor := vi.getCursorAtBuffer()
	if cursor.Y < len(vi.less.cells) && cursor.X < len(vi.less.cells[cursor.Y]) {
		cell = vi.less.cells[cursor.Y][cursor.X]
	}

	return
}

func (vi *Vi) MoveEndLine() {
	vi.SetCursor(Coordinates{X: vi.less.width - 1, Y: vi.cursor.Y})
	i := vi.lastIdxCursorRow()
	if i < vi.cursor.X {
		vi.SetCursor(Coordinates{X: i, Y: vi.cursor.Y})
	} else if i > vi.cursor.X {
		vi.less.SeekEndLine()
	}
}

func (vi *Vi) MoveStartFile() {
	vi.SetCursor(Coordinates{0, 0})
	vi.less.SeekStartFile()
}

func (vi *Vi) MoveEndFile() {
	vi.SetCursor(Coordinates{vi.cursor.X, vi.getMaxCursorY()})
	vi.less.SeekEndFile()
}

func (vi *Vi) MoveUp() {
	if vi.cursor.Y > 0 {
		vi.cursor.Y--
		vi.skipNulls()
	} else {
		vi.less.SeekUp()
	}
}

func (vi *Vi) MoveDown() {
	if vi.cursor.Y < vi.getMaxCursorY() { // account for command line
		vi.cursor.Y++
		vi.skipNulls()
	} else {
		vi.less.SeekDown()
	}
}

func (vi *Vi) MoveLeft() {
	if vi.moveLeft() {
		vi.skipNulls()
	}
}

func (vi *Vi) moveLeft() (ok bool) {
	if vi.cursor.X > 0 {
		vi.cursor.X--
		ok = true
	} else {
		vi.less.SeekLeft()
	}
	return
}

func (vi *Vi) MoveRight() {
	if vi.moveRight() {
		vi.skipNulls()
	}
}

func (vi *Vi) moveRight() (ok bool) {
	i := vi.lastIdxCursorRow()
	if vi.cursor.X < vi.less.width-1 && vi.cursor.X < i {
		vi.cursor.X++
		ok = true
	} else if vi.cursor.X < i {
		vi.less.SeekRight()
	}
	return
}

/* satisfy Component interface */

func (vi *Vi) Resize(width, height int) error {
	return vi.less.Resize(width, height)
}

func (vi *Vi) Draw(w Writer) error {
	return vi.less.Draw(w)
}

func (vi *Vi) Move(x, y int) error {
	return vi.less.Move(x, y)
}
func (vi *Vi) Height() int {
	return vi.less.Height()
}
func (vi *Vi) Width() int {
	return vi.less.Width()
}
func (vi *Vi) Position() (int, int) {
	return vi.less.Position()
}

/* satisfy Handler interface */

func (vi *Vi) Man() string {
	panic("todo")
}

func (vi *Vi) GetCursor() Coordinates {
	// use less GetCursor if we are in search mode
	if vi.less.mode != normalMode {
		return vi.less.GetCursor()
	}

	x, y := vi.less.Position()
	cursor := vi.cursor
	if i := vi.lastIdxCursorRow(); cursor.X > i {
		cursor.X = i
	}

	return Coordinates{
		X: x + cursor.X,
		Y: y + cursor.Y,
	}
}

func (vi *Vi) Handle(ev termbox.Event) (bool, error) {
	switch vi.mode {
	case normal:
		if vi.less.mode != normalMode {
			return vi.less.Handle(ev)
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
