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

// Cursor is a helper structure which manages a cursor over a Scroll.
type Cursor struct {
	scroll    *component.Scroll
	cursor    term.Coordinates
	selection struct {
		mode       int
		scrollFrom term.Coordinates
		cells      [][]term.Cell
	}
}

// NewCursor allocates storage for a new cursor,
// initializes it with an empty Scroll, and returns it.
func NewCursor() *Cursor {
	c := new(Cursor)
	scroll := component.NewScroll()
	c.Init(scroll)
	return c
}

// Init initializes this cursor with the given scroll.
// Note that this Cursor implementation does not support text wrap mode,
// so scroll.Wrap should be falsc.
func (c *Cursor) Init(scroll *component.Scroll) {
	if scroll.Wrap == true {
		panic("Cursor does not support wrap mode yet")
	}
	c.cursor = term.Coordinates{}
	c.scroll = scroll
	c.selection.mode = noSelection
}

// Cursor returns the current position of the cursor. It safisfies fractal.Handler.Cursor.
func (c *Cursor) Cursor() (term.Coordinates, bool) {
	return c.cursor, true
}

func (c *Cursor) setCursor(pos term.Coordinates) {
	if pos.X < 0 || pos.Y < 0 ||
		pos.X >= c.scroll.Width() || pos.Y >= c.scroll.Height() {
		// FIXME scroll Resize is not captured
		// panic(fmt.Sprintf("cursor out of bounds: %+v", pos))
	}

	c.cursor = pos

	if c.selection.mode != noSelection {
		c.setSelection()
	}
}

// Search searches text string in the underlying cell buffer. It returns
// the number of occurrences found.
func (c *Cursor) Search(text string) int {
	res := c.scroll.Search(text)
	c.MoveNextSearchResult()
	return res
}

func (c *Cursor) setCursorResult() (ok bool) {
	var pos term.Coordinates
	if pos, ok = c.scroll.Result(); ok {
		c.setCursor(c.scrollToWindowCoordinates(pos))
	}
	return
}

// MoveNextSearchResult moves the cursor to the next search result if any.
func (c *Cursor) MoveNextSearchResult() (ok bool) {
	c.scroll.SeekNextResult()
	ok = c.setCursorResult()
	return
}

// MovePrevSearchResult moves the cursor to the next search result if any.
func (c *Cursor) MovePrevSearchResult() (ok bool) {
	c.scroll.SeekPrevResult()
	ok = c.setCursorResult()
	return
}

// MoveStartLine moves the cursor at the start of the current line, scrolling
// to the start of the line if required.
func (c *Cursor) MoveStartLine() (ok bool) {
	ok = c.scroll.SeekStartLine()
	pos := term.Coordinates{X: 0, Y: c.cursor.Y}
	if !ok {
		ok = pos != c.cursor
	}
	c.setCursor(pos)
	return
}

// MoveEndLine moves the cursor at the end of the current line, scrolling
// to the end of the line if required.
func (c *Cursor) MoveEndLine() (ok bool) {
	y := c.cursorAtScroll().Y
	if y >= c.scroll.Rows() {
		c.setCursor(term.Coordinates{X: 0, Y: c.cursor.Y})
		ok = true
		return
	}

	cols := c.scroll.Columns(y)
	width := c.scroll.Width()
	if cols == 0 || width == 0 {
		return
	}

	var didSeek bool
	for cols > c.scroll.Offset().X+width {
		didSeek = true
		c.scroll.SeekRight()
	}

	// note that padding is subject to limits imposed by Scroll's max offset.
	// If we want to add padding even for the longest line of the scroll,
	// we should add some padding to the max X offset of the scroll.
	const padding = 10
	if didSeek {
		for i := 0; i < padding; i++ {
			c.scroll.SeekRight()
		}
	}

	// handle cursor *past* end of line
	var pos term.Coordinates
	for {
		pos = term.Coordinates{X: cols - c.scroll.Offset().X - 1, Y: c.cursor.Y}
		if pos.X >= 0 {
			break
		}
		c.scroll.SeekLeft()
	}
	ok = c.cursor != pos
	c.setCursor(pos)

	return
}

// MoveFirstLine moves the cursor to the first line, scrolling the content
// if appplicable.
func (c *Cursor) MoveFirstLine() (ok bool) {
	ok = c.scroll.SeekStartFile()
	pos := term.Coordinates{}
	if !ok {
		ok = pos != c.cursor
	}
	c.setCursor(pos)
	return
}

// MoveLastLine moves the cursor to the last line, scrolling the content
// if required.
func (c *Cursor) MoveLastLine() (ok bool) {
	height := c.scroll.Height()
	rows := c.scroll.Rows()
	if height == 0 || rows == 0 {
		return
	}

	ok = c.scroll.SeekEndFile()
	pos := term.Coordinates{X: 0, Y: rows - c.scroll.Offset().Y - 1}
	if !ok {
		ok = pos != c.cursor
	}
	c.setCursor(pos)

	return
}

// MoveDown moves the cursor to the line under the current line, scrolling
// the content if required.
func (c *Cursor) MoveDown() (ok bool) {
	if c.cursor.Y+1 >= c.scroll.Height() {
		ok = c.scroll.SeekDown()
		c.setCursor(c.cursor)
		return
	}
	ok = true
	c.setCursor(term.Coordinates{X: c.cursor.X, Y: c.cursor.Y + 1})
	return
}

// MoveUp moves the cursor the the line above the current line, scrolling
// the content up if required.
func (c *Cursor) MoveUp() (ok bool) {
	if c.cursor.Y == 0 {
		ok = c.scroll.SeekUp()
		c.setCursor(c.cursor)
		return
	}
	ok = true
	c.setCursor(term.Coordinates{X: c.cursor.X, Y: c.cursor.Y - 1})
	return
}

// MoveLeft moves the cursor to the cell left of the current cell, scrolling
// the content left if required.
func (c *Cursor) MoveLeft() (ok bool) {
	if c.cursor.X == 0 {
		ok = c.scroll.SeekLeft()
		c.setCursor(c.cursor)
		return
	}
	ok = true
	c.setCursor(term.Coordinates{X: c.cursor.X - 1, Y: c.cursor.Y})
	return
}

// MoveRight moves the cursor to the cell right of the current cell, scrolling
// the content right if required.
func (c *Cursor) MoveRight() (ok bool) {
	if c.cursor.X+1 >= c.scroll.Width() {
		ok = c.scroll.SeekRight()
		c.setCursor(c.cursor)
		return
	}
	ok = true
	c.setCursor(term.Coordinates{X: c.cursor.X + 1, Y: c.cursor.Y})
	return
}

// MoveLeftWrap will move the cursor to the left or wrap to end of
// previous line if cursor is at X=0
func (c *Cursor) MoveLeftWrap() bool {
	// we cannot use 0 as a wrap coordinate because what we really want
	// is to return false when we cannot move which depends on the type of movc.
	// Returing false when reached 0, in the case of moving one cell at a time is correct
	// but not when we jump from start of word to the next
	currc, curro := c.cursor, c.scroll.Offset()
	c.MoveLeft()
	if c.cursor.X == currc.X && c.scroll.Offset().X == curro.X {
		c.MoveUp()
		if c.cursor.Y == currc.Y && c.scroll.Offset().Y == curro.Y {
			return false
		}
		c.scroll.SeekEndLine()
		c.MoveEndLine()
	}
	return true
}

// MoveRightWrap will move the cursor to the right or wrap to beginning
// of next line if cursor is at X=EOL
func (c *Cursor) MoveRightWrap() bool {
	currc, curro := c.cursor, c.scroll.Offset()
	c.MoveRight()

	if currc.X == c.cursor.X && c.scroll.Offset().X == curro.X {
		c.MoveDown()
		if currc.Y == c.cursor.Y && c.scroll.Offset().Y == curro.Y {
			return false
		}
		c.MoveStartLine()
	}
	return true
}

func (c *Cursor) cursorAtScroll() term.Coordinates {
	return c.windowToScrollCoordinates(c.cursor)
}

func (c *Cursor) cellAtCursor() (cell term.Cell, ok bool) {
	cursorAtScroll := c.cursorAtScroll()
	cells := c.scroll.RawCells()
	if cursorAtScroll.Y >= len(cells) ||
		cursorAtScroll.X >= len(cells[cursorAtScroll.Y]) {
		return
	}
	ok = true
	cell = cells[cursorAtScroll.Y][cursorAtScroll.X]
	return
}

func (c *Cursor) moveAfterRune(t []rune, move func() bool) (ok bool) {
	const (
		init = iota
		foundRune
	)

	lastSanePos := c.cursor
	lastSaneOffset := c.scroll.Offset()
	state := init

	for move() {
		cell, cOk := c.cellAtCursor()
		if !cOk {
			continue
		}
		ok = true
		lastSanePos = c.cursor
		lastSaneOffset = c.scroll.Offset()

		switch state {
		case init:
			for _, r := range t {
				if r == cell.Ch {
					state = foundRune
					break
				}
			}
		case foundRune:
			none := true
			for _, r := range t {
				none = none && r != cell.Ch
			}
			if none {
				return
			}
		}
	}

	c.revertTo(lastSanePos, lastSaneOffset)
	return
}

func (c *Cursor) revertTo(pos, offset term.Coordinates) {
	c.setCursor(pos)
	c.scroll.SeekTo(offset)
}

func (c *Cursor) moveBeforeRune(t []rune, move func() bool) (ok bool) {
	const (
		skipRune = iota
		findRune
		done
	)

	state := skipRune
	initial := c.cursor
	initialOffset := c.scroll.Offset()
	prev := initial
	prevOffset := initialOffset

	for move() {
		cell, _ := c.cellAtCursor()
		switch state {
		case skipRune:
			none := true
			for _, r := range t {
				none = none && cell.Ch != r
			}
			if none {
				state = findRune
			}
		case findRune:
			for _, r := range t {
				if cell.Ch == r {
					state = done
					break
				}
			}
		}
		if state == done {
			break
		}
		prev = c.cursor
		prevOffset = c.scroll.Offset()
	}

	if state == done {
		ok = true
		c.revertTo(prev, prevOffset)
	} else {
		c.revertTo(initial, initialOffset)
	}
	return
}

var skipRunes = []rune{'.', ',', ':', ';', ' ', ')', '"',
	'\'', '(', '{', '}', '[', ']', '\t', '\x00', '\\', '/'}

// MoveRightStartWord moves the cursor right to the start of the next word.
func (c *Cursor) MoveRightStartWord() bool {
	return c.moveAfterRune(skipRunes, c.MoveRightWrap)
}

// MoveLeftStartWord moves the cursor left to the start of the previous word.
func (c *Cursor) MoveLeftStartWord() bool {
	return c.moveBeforeRune(skipRunes, c.MoveLeftWrap)
}

// MoveRightEndWord moves the cursor right to the end of the next or current word.
func (c *Cursor) MoveRightEndWord() bool {
	return c.moveBeforeRune(skipRunes, c.MoveRightWrap)
}

// MoveLeftEndWord moves the cursor left to the end of the previous word.
func (c *Cursor) MoveLeftEndWord() bool {
	return c.moveAfterRune(skipRunes, c.MoveLeftWrap)
}

func (c *Cursor) moveMatchRune(target, match rune, move func() bool) bool {
	currc, curro := c.cursor, c.scroll.Offset()
	pending := 1
	var prev, prevOffset term.Coordinates

	for pending != 0 && move() {
		prev = c.cursor
		prevOffset = c.scroll.Offset()
		cell, ok := c.cellAtCursor()
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
		c.revertTo(prev, prevOffset)
		return true
	}

	c.revertTo(currc, curro)
	return false
}

// MoveToMatchingRune moves the cursor to the balanced matching rune of the rune at
// the current cursor's cell.
func (c *Cursor) MoveToMatchingRune() bool {
	cell, ok := c.cellAtCursor()
	if !ok {
		return ok
	}

	switch cell.Ch {
	case '[':
		ok = c.moveMatchRune('[', ']', c.MoveRightWrap)
	case '{':
		ok = c.moveMatchRune('{', '}', c.MoveRightWrap)
	case '(':
		ok = c.moveMatchRune('(', ')', c.MoveRightWrap)

	case ']':
		ok = c.moveMatchRune(']', '[', c.MoveLeftWrap)
	case '}':
		ok = c.moveMatchRune('}', '{', c.MoveLeftWrap)
	case ')':
		ok = c.moveMatchRune(')', '(', c.MoveLeftWrap)
	}

	return ok
}

// InsertRowAbove inserts a row above the current row and moves the cursor up.
func (c *Cursor) InsertRowAbove() {
	cursorAtScroll := c.cursorAtScroll()
	if cursorAtScroll.Y == 0 {
		c.scroll.InsertRowAt(0)
		return
	}
	c.scroll.InsertRowAt(cursorAtScroll.Y - 1)
	c.MoveUp()
}

// InsertRowBelow inserts a row below the current row and moves the cursor down.
func (c *Cursor) InsertRowBelow() {
	cursorAtScroll := c.cursorAtScroll()
	c.scroll.InsertRowAt(cursorAtScroll.Y + 1)
	c.MoveDown()
}

// Insert will insert rune at the current cursor's position
func (c *Cursor) Insert(r rune) {
	pos := c.scroll.InsertAt(c.cursorAtScroll(), r)
	c.setCursor(c.scrollToWindowCoordinates(pos))
}

// Delete deletes the cell at the current cursor position.
func (c *Cursor) Delete() (ok bool) {
	var pos term.Coordinates
	pos, ok = c.scroll.DeleteCell(c.cursorAtScroll())
	if ok {
		c.setCursor(c.scrollToWindowCoordinates(pos))
	}
	return
}

// Backspace is a special form of Delete, named after the keyboard key backspace.
func (c *Cursor) Backspace() (ok bool) {
	if c.cursor.X > 0 || c.scroll.Offset().X > 0 {
		c.MoveLeft()
		ok = c.Delete()
		return
	}

	if ok = c.MoveUp(); ok {
		c.Conflate()
	}
	return
}

// Conflate removes the new line character at the end of the current line.
func (c *Cursor) Conflate() (ok bool) {
	pos := c.cursorAtScroll()
	if pos.Y >= c.scroll.Rows() {
		ok = false
		return
	}

	ok = true

	c.MoveEndLine()
	length := c.scroll.Columns(pos.Y)
	if length == 0 {
		ok = c.scroll.DeleteRow(pos.Y)
		if !ok {
			panic(fmt.Sprintf("could not delete row at index: %d", pos.Y))
		}
		return
	}

	c.MoveRight()
	c.scroll.ConflateRow(pos.Y)
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

func (c *Cursor) scrollToWindowCoordinates(pos term.Coordinates) term.Coordinates {
	pos = term.Coordinates{
		X: pos.X - c.scroll.Offset().X,
		Y: pos.Y - c.scroll.Offset().Y,
	}

	// scroll can return some coordinates that are be outside
	// of the bounds of the current window.
	// For instance, if DeleteCell deletes a tab, it could be that
	// pos.X at scroll yields a negative coordinate
	// (i.c. -3 if tabspaces is 4)
	for pos.X < 0 && c.scroll.SeekLeft() {
		pos.X++
	}
	for pos.Y < 0 && c.scroll.SeekUp() {
		pos.Y++
	}
	for pos.X >= c.scroll.Width() && c.scroll.SeekRight() {
		pos.X--
	}
	for pos.Y >= c.scroll.Height() && c.scroll.SeekDown() {
		pos.Y--
	}
	return pos
}

func (c *Cursor) windowToScrollCoordinates(pos term.Coordinates) term.Coordinates {
	return term.Coordinates{
		X: pos.X + c.scroll.Offset().X,
		Y: pos.Y + c.scroll.Offset().Y,
	}
}

func (c *Cursor) setSelection() {
	invertAttr(c.selection.cells)

	from := c.selection.scrollFrom
	to := c.cursorAtScroll()

	switch c.selection.mode {
	case visualSelection:
		c.selection.cells = c.scroll.Select(from, to)
	case visualLineSelection:
		c.selection.cells = c.scroll.SelectLine(from, to)
	case visualBlockSelection:
		c.selection.cells = c.scroll.SelectBlock(from, to)
	}

	invertAttr(c.selection.cells)
}

func (c *Cursor) inBounds() (ok bool) {
	pos := c.cursorAtScroll()
	if pos.Y >= c.scroll.Rows() || pos.X > c.scroll.Columns(pos.Y) {
		return
	}
	ok = true
	return
}

// Select anchors the current cursor position as the start of a text selection.
// In order to unset anchor, use Unselect(). It returns true if cursor is in bounds or
// false if selection failed.
func (c *Cursor) Select() (ok bool) {
	if ok = c.inBounds(); !ok {
		return
	}
	c.selection.scrollFrom = c.cursorAtScroll()
	c.selection.mode = visualSelection
	c.setSelection()
	return
}

// SelectLine anchors the current cursor position as the start of a line selection.
// In order to unset anchor, use Unselect(). It returns true if cursor is in bounds or
// false if selection failed.
func (c *Cursor) SelectLine() (ok bool) {
	if ok = c.inBounds(); !ok {
		return
	}
	c.selection.scrollFrom = c.cursorAtScroll()
	c.selection.mode = visualLineSelection
	c.setSelection()
	return
}

// SelectBlock anchors the current cursor position as the start of a block selection.
// In order to unset anchor, use Unselect(). It returns true if cursor is in bounds or
// false if selection failed.
func (c *Cursor) SelectBlock() (ok bool) {
	if ok = c.inBounds(); !ok {
		return
	}
	c.selection.scrollFrom = c.cursorAtScroll()
	c.selection.mode = visualBlockSelection
	c.setSelection()
	return
}

// Unselect resets the current selection anchor.
func (c *Cursor) Unselect() {
	c.selection.mode = noSelection
	invertAttr(c.selection.cells)
	c.selection.cells = nil
}

// Selection returns the current text under either text, line or block selection.
func (c *Cursor) Selection() string {
	return cell.CellsToString(c.selection.cells)
}

// Redo reverses the previously reversed update to the underlying buffer.
func (c *Cursor) Redo() bool {
	ok, at := c.scroll.Redo()
	if !ok {
		return false
	}
	c.setCursor(c.scrollToWindowCoordinates(at))
	return true
}

// Undo reverses the last update to the underlying buffer.
func (c *Cursor) Undo() bool {
	ok, at := c.scroll.Undo()
	if !ok {
		return false
	}
	c.setCursor(c.scrollToWindowCoordinates(at))
	return true
}

// Row returns the row number of the row where the cursor is positioned.
func (c *Cursor) Row() int {
	return c.cursorAtScroll().Y
}

// Column returns the column number of the column where the cursor is positioned.
func (c *Cursor) Column() int {
	return c.cursorAtScroll().X
}

// Cell returns the cell where the cursor is positioned or false if there's no cell
// at the current cursor position.
func (c *Cursor) Cell() (term.Cell, bool) {
	return c.cellAtCursor()
}

// DeleteSelection deletes the current text under selection or does nothing
func (c *Cursor) DeleteSelection() (ok bool) {
	if len(c.selection.cells) == 0 {
		return
	}
	if ok = c.inBounds(); !ok {
		return
	}
	c.Unselect()
	start, _, _ := c.scroll.Delete(c.selection.scrollFrom, c.cursorAtScroll())
	c.setCursor(c.scrollToWindowCoordinates(start))
	return
}
