package handler

import "github.com/ernestrc/fractal"

type cursorHelper struct {
	idx int
	pos fractal.Coordinates
}

func (c *cursorHelper) Reset() {
	c.idx, c.pos.X, c.pos.Y = 0, 0, 0
}

func (c *cursorHelper) moveRight(cells []fractal.Cell) {
}

func (c *cursorHelper) moveLeft(cells []fractal.Cell) {
}

func (c *cursorHelper) moveUp(cells []fractal.Cell) {
}

func (c *cursorHelper) moveDown(cells []fractal.Cell) {
}

func (c *cursorHelper) moveEndLine(cells []fractal.Cell) {
}

func (c *cursorHelper) moveStartLine(cells []fractal.Cell) {
}
