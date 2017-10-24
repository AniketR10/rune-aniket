package fractal

type CellBuf struct {
	cells [][]Cell
}

func (b *CellBuf) Init() {
	b.cells = make([][]Cell, 1)
	b.cells[0] = make([]Cell, 0)
}

func (b *CellBuf) Write(p string) (n int, err error) {
	if b.cells == nil {
		b.Init()
	}
	for _, r := range p {
		b.WriteRune(r)
	}
	return len(p), nil
}

func (b *CellBuf) writeRune(r rune) (n int, err error) {
	lastIdx := len(b.cells) - 1
	last := b.cells[lastIdx][len(b.cells[lastIdx])-1]
	b.cells[lastIdx] = append(b.cells[lastIdx], Cell{
		Ch:          r,
		Coordinates: Coordinates{X: last.X + 1, Y: lastIdx},
	})
	return 1, nil
}

func (b *CellBuf) WriteRune(r rune) (n int, err error) {
	if b.cells == nil {
		b.Init()
	}
	return b.writeRune(r)
}

// TODO add config (tabs, spaces, null)
func (b *CellBuf) fillIn(pos Coordinates) {
	if b.cells == nil {
		b.Init()
	}
	// fill in rows
	for diff := pos.Y - len(b.cells); diff >= 0; diff-- {
		row := make([]Cell, 0)
		b.cells = append(b.cells, row)
	}

	// fill in cells
	for x := pos.X; pos.X >= len(b.cells[pos.Y]); x++ {
		b.cells[pos.Y] = append(b.cells[pos.Y], Cell{
			Coordinates: Coordinates{X: x, Y: pos.Y}})
	}
}

func (b *CellBuf) WriteAt(pos Coordinates, r rune) (n int, err error) {
	b.fillIn(pos)
	b.cells[pos.Y][pos.X] = Cell{Ch: r, Coordinates: pos}

	return 1, nil
}

func (b *CellBuf) InsertAt(pos Coordinates, r rune) (n int, err error) {
	b.fillIn(pos)

	rowLen := len(b.cells[pos.Y])
	// make sure we have enough capacity
	b.cells[pos.Y] = append(b.cells[pos.Y], Cell{})[:rowLen]
	copy(b.cells[pos.Y][pos.X+1:], b.cells[pos.Y][pos.X:])
	b.cells[pos.Y][pos.X] = Cell{Ch: r, Coordinates: pos}

	return 1, nil
}

// TODO should truncate multiple lines
func (b *CellBuf) Truncate(n int) {
	if b.cells == nil {
		return
	}
	lastIdx := len(b.cells) - 1
	b.cells[lastIdx] = b.cells[lastIdx][:n]
}

func (b *CellBuf) TruncateAt(pos Coordinates) error {
	if b.cells == nil {
		return nil
	}
	if pos.X < 0 || pos.Y < 0 {
		panic("negative coordinates")
	}
	idx := b.getIdx(pos)
	tmp := b.cells[:idx]
	tmp = append(tmp, b.cells[idx+1:]...)
	b.buffer = tmp
	return nil
}

func (b *CellBuf) RowLen(i int) (int, bool) {
	if i < 0 {
		panic("illegal index")
	}
	if i >= len(b.cells) {
		return 0, false
	}
	return len(b.cells[i]), true
}
