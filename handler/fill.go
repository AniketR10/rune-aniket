package handler

import (
	"termbox"

	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/component"
)

// FillHandler is a handler used for testing
type FillHandler struct {
	component.Fill
	Active bool
}

func NewFillHandler() *FillHandler {
	t := new(FillHandler)
	t.Ch = 'A'
	t.Active = true
	return t
}

func (t *FillHandler) Handle(termbox.Event) error {
	// signal that we handled the event
	t.Ch++
	return nil
}

func (t *FillHandler) GetCursor() fractal.Coordinates {
	return fractal.Coordinates{0, 0}
}

func (t *FillHandler) GetAttr() (fg termbox.Attribute, bg termbox.Attribute) {
	return termbox.ColorDefault, termbox.ColorDefault
}

func (t *FillHandler) IsActive() bool {
	return t.Active
}

func (t *FillHandler) Man() string {
	return ""
}
