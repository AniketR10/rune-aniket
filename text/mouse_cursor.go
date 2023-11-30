package text

import (
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
)

// CursorMouseDelegate satisfies MouseDelegate with a Cursor on a component.Scroll.
func CursorMouseDelegate(c *Cursor) MouseDelegate {
	return mouseDelegate{cursor: c}
}

// satisfies text.MouseDelegate
type mouseDelegate struct {
	cursor *Cursor
}

func (d mouseDelegate) OnAction(pos term.Coordinates, action MouseAction) bool {
	return false
}

func (d mouseDelegate) ScrollUp(n int) (ok bool) {
	for i := 0; i < n; i++ {
		ok = d.scroll().SeekUp()
		if !ok {
			return
		}
	}
	return
}

func (d mouseDelegate) ScrollDown(n int) (ok bool) {
	for i := 0; i < n; i++ {
		ok = d.scroll().SeekDown()
		if !ok {
			return
		}
	}
	return
}

func (d mouseDelegate) SetSelectionStart(pos term.Coordinates) {
	if _, ok := d.cursor.SelectionMode(); ok {
		d.cursor.Unselect()
	}
	pos = d.cursor.ScrollCoordinates(pos)
	d.cursor.MoveToScroll(pos)
	d.cursor.Select()
}

func (d mouseDelegate) SetSelectionEnd(pos term.Coordinates) {
	if _, ok := d.cursor.SelectionMode(); !ok {
		return
	}
	pos = d.cursor.ScrollCoordinates(pos)
	d.cursor.MoveToScroll(pos)
}

func (d mouseDelegate) ClearSelection() {
	d.cursor.Unselect()
}

func (d mouseDelegate) SelectWordAt(pos term.Coordinates) {
	pos = d.cursor.ScrollCoordinates(pos)
	start, end, word := d.scroll().WordAt(pos)
	if word == "" {
		return
	}
	start = d.cursor.WindowCoordinates(start)
	end = d.cursor.WindowCoordinates(end)
	d.SetSelectionStart(start)
	d.SetSelectionEnd(end)
}

func (d mouseDelegate) SelectLine(y int) {
	if _, ok := d.cursor.SelectionMode(); ok {
		d.cursor.Unselect()
	}
	pos := d.cursor.ScrollCoordinates(term.Coordinates{Y: y})
	d.cursor.MoveToScroll(pos)
	d.cursor.SelectLine()
}

func (d mouseDelegate) Width() int {
	return d.scroll().Width()
}

func (d mouseDelegate) Height() int {
	return d.scroll().SizeHeight()
}

func (d mouseDelegate) scroll() *component.Scroll {
	return d.cursor.scroll
}
