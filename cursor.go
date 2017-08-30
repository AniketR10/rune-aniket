package fractal

type cursorHelper struct {
	idx int
	Coordinates
}

func (c *cursorHelper) Reset() {
	c.idx, c.X, c.Y = 0, 0, 0
}

func (c *cursorHelper) moveRight(cells []Cell) {
}

func (c *cursorHelper) moveLeft(cells []Cell) {
}

func (c *cursorHelper) moveUp(cells []Cell) {
}

func (c *cursorHelper) moveDown(cells []Cell) {
}

func (c *cursorHelper) moveEndLine(cells []Cell) {
}

func (c *cursorHelper) moveStartLine(cells []Cell) {
}
