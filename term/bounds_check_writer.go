package term

type boundsCheckWriter struct {
	height int
	width  int
	w      Writer
}

// BoundsCheckWriter returns a Writer which wraps w to make sure that
// calls to SetCell and SetCursor will never be out of the bounds defined by height
// or width.
func BoundsCheckWriter(width, height int, w Writer) Writer {
	return boundsCheckWriter{height: height, width: width, w: w}
}

func (p boundsCheckWriter) SetCell(pos Coordinates, c Cell) {
	if p.outOfBounds(pos) {
		return
	}
	p.w.SetCell(pos, c)
}

func (p boundsCheckWriter) Flush() error {
	return p.w.Flush()
}

func (p boundsCheckWriter) Clear(attr Attributes) error {
	return p.w.Clear(attr)
}

func (p boundsCheckWriter) SetCursor(pos Coordinates) {
	if p.outOfBounds(pos) {
		return
	}
	p.w.SetCursor(pos)
}

func (p boundsCheckWriter) outOfBounds(pos Coordinates) bool {
	return pos.X >= p.width || pos.Y >= p.height || pos.X < 0 || pos.Y < 0
}
