package term

import "context"

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

func (p boundsCheckWriter) UnionAttributes(pos Coordinates, attr Attributes) {
	if p.outOfBounds(pos) {
		return
	}
	p.w.UnionAttributes(pos, attr)
}

func (p boundsCheckWriter) Context() context.Context {
	return p.w.Context()
}

func (p boundsCheckWriter) outOfBounds(pos Coordinates) bool {
	return pos.X >= p.width || pos.Y >= p.height || pos.X < 0 || pos.Y < 0
}
