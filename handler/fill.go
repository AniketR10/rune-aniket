package handler

import (
	"termbox"

	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/component"
)

// FillHandler is a handler used for testing composite handlers
type FillHandler struct {
	fill component.Fill
	component.Frame
}

func NewFillHandler() *FillHandler {
	t := new(FillHandler)
	t.fill.Ch = 'A'
	if err := t.SetContent(&t.fill); err != nil {
		panic(err)
	}
	return t
}

func (t *FillHandler) Handle(termbox.Event) (bool, error) {
	// signal that we handled the event
	t.fill.Ch++
	return false, nil
}

func (t *FillHandler) GetCursor() fractal.Coordinates {
	return fractal.Coordinates{X: -1, Y: -1}
}

func (t *FillHandler) GetAttr() (fg termbox.Attribute, bg termbox.Attribute) {
	return termbox.ColorDefault, termbox.ColorDefault
}

func (t *FillHandler) Man() string {
	return ""
}
