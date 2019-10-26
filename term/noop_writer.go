package term

// used for benchmarks
type NoopWriter struct{}

func (w NoopWriter) SetCell(pos Coordinates, cell Cell) {
}

func (w NoopWriter) SetAttr(pos Coordinates, attr Attributes) {
}

func (w NoopWriter) Flush() (err error) {
	return
}

func (w NoopWriter) Clear(Attributes) (err error) {
	return
}

func (w NoopWriter) SetCursor(pos Coordinates) {
}
