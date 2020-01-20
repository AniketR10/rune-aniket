package editor

import (
	"fmt"

	"github.com/ernestrc/fractal/cell"
	"github.com/ernestrc/fractal/component"
	"github.com/ernestrc/fractal/term"
)

const (
	noSelection = iota
	visualSelection
	visualLineSelection
	visualBlockSelection
)

// TODO allow client to configure how much to seek when moving cursor
// left/right/up/down and reached window limit
// TODO allow client to configure how much offset to leave before
// seeking scroll on reached window limit when moving cursor left/up/right/down
// TODO add clipboard
// TODO if Wrap is on, then move right should use moveRightWrap
// Cursor adds a cursor to a scroll.
type Cursor struct {
	scroll    *component.Scroll
	cursor    term.Coordinates
	selection struct {
		mode       int
		scrollFrom term.Coordinates
		cells      [][]term.Cell
	}
}

func NewCursor() *Cursor {
	e := new(Cursor)
	scroll := component.NewScroll()
	e.Init(scroll)
	return e
}

func (e *Cursor) Init(scroll *component.Scroll) {
	if scroll.Wrap == true {
		panic("Cursor does not support wrap mode yet")
	}
	e.cursor = term.Coordinates{}
	e.scroll = scroll
	e.selection.mode = noSelection
}

func (e *Cursor) Cursor() (term.Coordinates, bool) {
	return e.cursor, true
}

func (e *Cursor) setCursor(pos term.Coordinates) {
	if pos.X < 0 || pos.Y < 0 ||
		pos.X >= e.scroll.Width() || pos.Y >= e.scroll.Height() {
		// FIXME scroll Resize is not captured
		panic(fmt.Sprintf("cursor out of bounds: %+v", pos))
	}

	e.cursor = pos

	if e.selection.mode != noSelection {
		e.setSelection()
	}
}

// Search searches text string in the underlying cell buffer. It returns
// the number of occurrences found.
func (e *Cursor) Search(text string) int {
	res := e.scroll.Search(text)
	e.MoveNextSearchResult()
	return res
}

func (e *Cursor) setCursorResult() (ok bool) {
	var pos term.Coordinates
	if pos, ok = e.scroll.Result(); ok {
		e.setCursor(e.scrollToWindowCoordinates(pos))
	}
	return
}

// MoveNextSearchResult moves the cursor to the next search result if any.
func (e *Cursor) MoveNextSearchResult() (ok bool) {
	e.scroll.SeekNextResult()
	ok = e.setCursorResult()
	return
}

// MovePrevSearchResult moves the cursor to the next search result if any.
func (e *Cursor) MovePrevSearchResult() (ok bool) {
	e.scroll.SeekPrevResult()
	ok = e.setCursorResult()
	return
}

// MoveStartLine moves the cursor at the start of the current line, scrolling
// to the start of the line if required.
func (e *Cursor) MoveStartLine() (ok bool) {
	ok = e.scroll.SeekStartLine()
	pos := term.Coordinates{X: 0, Y: e.cursor.Y}
	if !ok {
		ok = pos != e.cursor
	}
	e.setCursor(pos)
	return
}

// MoveEndLine moves the cursor at the end of the current line, scrolling
// to the end of the line if required.
func (e *Cursor) MoveEndLine() (ok bool) {
	y := e.cursorAtScroll().Y
	if y >= e.scroll.Rows() {
		e.setCursor(term.Coordinates{X: 0, Y: e.cursor.Y})
		ok = true
		return
	}

	cols := e.scroll.Columns(y)
	width := e.scroll.Width()
	if cols == 0 || width == 0 {
		return
	}

	var didSeek bool
	for cols > e.scroll.Offset().X+width {
		didSeek = true
		e.scroll.SeekRight()
	}

	// note that padding is subject to limits imposed by Scroll's max offset.
	// If we want to add padding even for the longest line of the scroll,
	// we should add some padding to the max X offset of the scroll.
	const padding = 10
	if didSeek {
		for i := 0; i < padding; i++ {
			e.scroll.SeekRight()
		}
	}

	// handle cursor *past* end of line
	var pos term.Coordinates
	for {
		pos = term.Coordinates{X: cols - e.scroll.Offset().X - 1, Y: e.cursor.Y}
		if pos.X >= 0 {
			break
		}
		e.scroll.SeekLeft()
	}
	ok = e.cursor != pos
	e.setCursor(pos)

	return
}

// MoveFirstLine moves the cursor to the first line, scrolling the content
// if appplicable.
func (e *Cursor) MoveFirstLine() (ok bool) {
	ok = e.scroll.SeekStartFile()
	pos := term.Coordinates{}
	if !ok {
		ok = pos != e.cursor
	}
	e.setCursor(pos)
	return
}

// MoveLastLine moves the cursor to the last line, scrolling the content
// if required.
func (e *Cursor) MoveLastLine() (ok bool) {
	height := e.scroll.Height()
	rows := e.scroll.Rows()
	if height == 0 || rows == 0 {
		return
	}

	ok = e.scroll.SeekEndFile()
	pos := term.Coordinates{X: 0, Y: rows - e.scroll.Offset().Y - 1}
	if !ok {
		ok = pos != e.cursor
	}
	e.setCursor(pos)

	return
}

// MoveDown moves the cursor to the line under the current line, scrolling
// the content if required.
func (e *Cursor) MoveDown() (ok bool) {
	if e.cursor.Y+1 >= e.scroll.Height() {
		ok = e.scroll.SeekDown()
		e.setCursor(e.cursor)
		return
	}
	ok = true
	e.setCursor(term.Coordinates{X: e.cursor.X, Y: e.cursor.Y + 1})
	return
}

// MoveUp moves the cursor the the line above the current line, scrolling
// the content up if required.
func (e *Cursor) MoveUp() (ok bool) {
	if e.cursor.Y == 0 {
		ok = e.scroll.SeekUp()
		e.setCursor(e.cursor)
		return
	}
	ok = true
	e.setCursor(term.Coordinates{X: e.cursor.X, Y: e.cursor.Y - 1})
	return
}

// MoveLeft moves the cursor to the cell left of the current cell, scrolling
// the content left if required.
func (e *Cursor) MoveLeft() (ok bool) {
	if e.cursor.X == 0 {
		ok = e.scroll.SeekLeft()
		e.setCursor(e.cursor)
		return
	}
	ok = true
	e.setCursor(term.Coordinates{X: e.cursor.X - 1, Y: e.cursor.Y})
	return
}

// MoveRight moves the cursor to the cell right of the current cell, scrolling
// the content right if required.
func (e *Cursor) MoveRight() (ok bool) {
	if e.cursor.X+1 >= e.scroll.Width() {
		ok = e.scroll.SeekRight()
		e.setCursor(e.cursor)
		return
	}
	ok = true
	e.setCursor(term.Coordinates{X: e.cursor.X + 1, Y: e.cursor.Y})
	return
}

// MoveLeftWrap will move the cursor to the left or wrap to end of
// previous line if cursor is at X=0
func (e *Cursor) MoveLeftWrap() bool {
	// we cannot use 0 as a wrap coordinate because what we really want
	// is to return false when we cannot move which depends on the type of move.
	// Returing false when reached 0, in the case of moving one cell at a time is correct
	// but not when we jump from start of word to the next
	currc, curro := e.cursor, e.scroll.Offset()
	e.MoveLeft()
	if e.cursor.X == currc.X && e.scroll.Offset().X == curro.X {
		e.MoveUp()
		if e.cursor.Y == currc.Y && e.scroll.Offset().Y == curro.Y {
			return false
		}
		e.scroll.SeekEndLine()
		e.MoveEndLine()
	}
	return true
}

// MoveRightWrap will move the cursor to the right or wrap to beginning
// of next line if cursor is at X=EOL
func (e *Cursor) MoveRightWrap() bool {
	currc, curro := e.cursor, e.scroll.Offset()
	e.MoveRight()

	if currc.X == e.cursor.X && e.scroll.Offset().X == curro.X {
		e.MoveDown()
		if currc.Y == e.cursor.Y && e.scroll.Offset().Y == curro.Y {
			return false
		}
		e.MoveStartLine()
	}
	return true
}

func (e *Cursor) cursorAtScroll() term.Coordinates {
	return e.windowToScrollCoordinates(e.cursor)
}

func (e *Cursor) cellAtCursor() (cell term.Cell, ok bool) {
	cursorAtScroll := e.cursorAtScroll()
	cells := e.scroll.RawCells()
	if cursorAtScroll.Y >= len(cells) ||
		cursorAtScroll.X >= len(cells[cursorAtScroll.Y]) {
		return
	}
	ok = true
	cell = cells[cursorAtScroll.Y][cursorAtScroll.X]
	return
}

func (e *Cursor) moveAfterRune(t []rune, move func() bool) (ok bool) {
	const (
		init = iota
		foundRune
	)

	lastSanePos := e.cursor
	lastSaneOffset := e.scroll.Offset()
	state := init

	for move() {
		c, cOk := e.cellAtCursor()
		if !cOk {
			continue
		}
		ok = true
		lastSanePos = e.cursor
		lastSaneOffset = e.scroll.Offset()

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

	e.moveTo(lastSanePos, lastSaneOffset)
	return
}

func (e *Cursor) moveTo(pos, offset term.Coordinates) {
	e.setCursor(pos)
	e.scroll.SeekTo(offset)
}

func (e *Cursor) moveBeforeRune(t []rune, move func() bool) (ok bool) {
	const (
		skipRune = iota
		findRune
		done
	)

	state := skipRune
	initial := e.cursor
	initialOffset := e.scroll.Offset()
	prev := initial
	prevOffset := initialOffset

	for move() {
		c, _ := e.cellAtCursor()
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
		prev = e.cursor
		prevOffset = e.scroll.Offset()
	}

	if state == done {
		ok = true
		e.moveTo(prev, prevOffset)
	} else {
		e.moveTo(initial, initialOffset)
	}
	return
}

var skipRunes = []rune{'.', ',', ':', ';', ' ', ')', '"',
	'\'', '(', '{', '}', '[', ']', '\t', '\x00', '\\', '/'}

// MoveRightStartWord moves the cursor right to the start of the next word.
func (e *Cursor) MoveRightStartWord() bool {
	return e.moveAfterRune(skipRunes, e.MoveRightWrap)
}

// MoveLeftStartWord moves the cursor left to the start of the previous word.
func (e *Cursor) MoveLeftStartWord() bool {
	return e.moveBeforeRune(skipRunes, e.MoveLeftWrap)
}

// MoveRightEndWord moves the cursor right to the end of the next or current word.
func (e *Cursor) MoveRightEndWord() bool {
	return e.moveBeforeRune(skipRunes, e.MoveRightWrap)
}

// MoveLeftEndWord moves the cursor left to the end of the previous word.
func (e *Cursor) MoveLeftEndWord() bool {
	return e.moveAfterRune(skipRunes, e.MoveLeftWrap)
}

func (e *Cursor) moveMatchRune(target, match rune, move func() bool) bool {
	currc, curro := e.cursor, e.scroll.Offset()
	pending := 1
	var prev, prevOffset term.Coordinates

	for pending != 0 && move() {
		prev = e.cursor
		prevOffset = e.scroll.Offset()
		cell, ok := e.cellAtCursor()
		if !ok {
			continue
		}
		switch cell.Ch {
		case target:
			pending++
		case match:
			pending--
		}
	}

	if pending == 0 {
		e.moveTo(prev, prevOffset)
		return true
	}

	e.moveTo(currc, curro)
	return false
}

// MoveToMatchingRune moves the cursor to the balanced matching rune of the rune at
// the current cursor's cell.
func (e *Cursor) MoveToMatchingRune() bool {
	cell, ok := e.cellAtCursor()
	if !ok {
		return ok
	}

	switch cell.Ch {
	case '[':
		ok = e.moveMatchRune('[', ']', e.MoveRightWrap)
	case '{':
		ok = e.moveMatchRune('{', '}', e.MoveRightWrap)
	case '(':
		ok = e.moveMatchRune('(', ')', e.MoveRightWrap)

	case ']':
		ok = e.moveMatchRune(']', '[', e.MoveLeftWrap)
	case '}':
		ok = e.moveMatchRune('}', '{', e.MoveLeftWrap)
	case ')':
		ok = e.moveMatchRune(')', '(', e.MoveLeftWrap)
	}

	return ok
}

// InsertRowAbove inserts a row above the current row and moves the cursor up.
func (e *Cursor) InsertRowAbove() {
	cursorAtScroll := e.cursorAtScroll()
	if cursorAtScroll.Y == 0 {
		e.scroll.InsertRowAt(0)
		return
	}
	e.scroll.InsertRowAt(cursorAtScroll.Y - 1)
	e.MoveUp()
}

// InsertRowBelow inserts a row below the current row and moves the cursor down.
func (e *Cursor) InsertRowBelow() {
	cursorAtScroll := e.cursorAtScroll()
	e.scroll.InsertRowAt(cursorAtScroll.Y + 1)
	e.MoveDown()
}

// Insert will insert rune at the current cursor's position
func (e *Cursor) Insert(r rune) {
	pos := e.scroll.InsertAt(e.cursorAtScroll(), r)
	e.setCursor(e.scrollToWindowCoordinates(pos))
}

// Delete deletes the cell at the current cursor position.
func (e *Cursor) Delete() (ok bool) {
	var pos term.Coordinates
	pos, ok = e.scroll.DeleteCell(e.cursorAtScroll())
	if ok {
		// if DeleteCell deletes a tab, it could be that
		// pos.X at scroll yields a negative coordinate
		// (i.e. -3 if tabspaces is 4)
		cursorAt := e.scrollToWindowCoordinates(pos)
		for cursorAt.X < 0 {
			e.scroll.SeekLeft()
			cursorAt.X++
		}
		e.setCursor(cursorAt)
	}
	return
}

// Backspace is a special form of Delete, named after the keyboard key backspace.
func (e *Cursor) Backspace() (ok bool) {
	if e.cursor.X > 0 || e.scroll.Offset().X > 0 {
		e.MoveLeft()
		ok = e.Delete()
		return
	}

	if ok = e.MoveUp(); ok {
		e.Conflate()
	}
	return
}

// Conflate removes the new line character at the end of the current line.
func (e *Cursor) Conflate() (ok bool) {
	pos := e.cursorAtScroll()
	if pos.Y >= e.scroll.Rows() {
		ok = false
		return
	}

	ok = true

	e.MoveEndLine()
	length := e.scroll.Columns(pos.Y)
	if length == 0 {
		ok = e.scroll.DeleteRow(pos.Y)
		if !ok {
			panic(fmt.Sprintf("could not delete row at index: %d", pos.Y))
		}
		return
	}

	e.MoveRight()
	e.scroll.ConflateRow(pos.Y)
	return
}

func invertAttr(cells [][]term.Cell) {
	for i := 0; i < len(cells); i++ {
		for j := 0; j < len(cells[i]); j++ {
			c := cells[i][j]
			if c.Bg&term.AttrReverse == term.AttrReverse ||
				c.Fg&term.AttrReverse == term.AttrReverse {
				cells[i][j].Fg &^= term.AttrReverse
				cells[i][j].Bg &^= term.AttrReverse
			} else {
				cells[i][j].Fg |= term.AttrReverse
				cells[i][j].Bg |= term.AttrReverse
			}
		}
	}
}

func (e *Cursor) scrollToWindowCoordinates(pos term.Coordinates) term.Coordinates {
	return term.Coordinates{
		X: pos.X - e.scroll.Offset().X,
		Y: pos.Y - e.scroll.Offset().Y,
	}
}

func (e *Cursor) windowToScrollCoordinates(pos term.Coordinates) term.Coordinates {
	return term.Coordinates{
		X: pos.X + e.scroll.Offset().X,
		Y: pos.Y + e.scroll.Offset().Y,
	}
}

func (e *Cursor) setSelection() {
	invertAttr(e.selection.cells)

	from := e.selection.scrollFrom
	to := e.cursorAtScroll()

	switch e.selection.mode {
	case visualSelection:
		e.selection.cells = e.scroll.Select(from, to)
	case visualLineSelection:
		e.selection.cells = e.scroll.SelectLine(from, to)
	case visualBlockSelection:
		e.selection.cells = e.scroll.SelectBlock(from, to)
	}

	invertAttr(e.selection.cells)
}

// Select anchors the current cursor position as the start of a text selection.
// In order to unset anchor, use Unselect().
func (e *Cursor) Select() {
	e.selection.scrollFrom = e.cursorAtScroll()
	e.selection.mode = visualSelection
	e.setSelection()
}

// SelectLine anchors the current cursor position as the start of a line selection.
// In order to unset anchor, use Unselect().
func (e *Cursor) SelectLine() {
	e.selection.scrollFrom = e.cursorAtScroll()
	e.selection.mode = visualLineSelection
	e.setSelection()
}

// SelectBlock anchors the current cursor position as the start of a block selection.
// In order to unset anchor, use Unselect().
func (e *Cursor) SelectBlock() {
	e.selection.scrollFrom = e.cursorAtScroll()
	e.selection.mode = visualBlockSelection
	e.setSelection()
}

// Unselect resets the current selection anchor.
func (e *Cursor) Unselect() {
	e.selection.mode = noSelection
	invertAttr(e.selection.cells)
	e.selection.cells = nil
}

// Selection returns the current text under either text, line or block selection.
func (e *Cursor) Selection() string {
	return cell.CellsToString(e.selection.cells)
}

func (e *Cursor) Redo() bool {
	// TODO set cursor
	return e.scroll.Redo()
}

func (e *Cursor) Undo() bool {
	// TODO set cursor
	return e.scroll.Undo()
}
