package cell

// // TODO refactor to only reverse Buffer methods not Cells methods.
// // History adds Undo and Redo methods to a otherwise, irreversible Buffer.
// type History struct {
// 	buffer       *Buffer
// 	selector     Selector
// 	undoTimeline []func()
// 	redoTimeline []func()
// }
//
// // Init initializes this History with the given Buffer and resets
// // the internal undo, redo timelines.
// func (b *History) Init(bb *Buffer) {
// 	b.buffer = bb
// 	b.selector.Init(bb)
// 	b.undoTimeline = make([]func(), 0)
// 	b.redoTimeline = make([]func(), 0)
// }
//
// // Reset resets the redo and undo history.
// func (b *History) Reset() {
// 	b.redoTimeline = b.redoTimeline[:0]
// 	b.undoTimeline = b.undoTimeline[:0]
// }
//
// // Redo reverses the previously reversed update to the underlying buffer.
// func (b *History) Redo() (ok bool) {
// 	panic("TODO")
// }
//
// // Undo reverses the last update to the underlying buffer.
// // Redo can be used to reverse the undo.
// func (b *History) Undo() (ok bool) {
// 	lastCmd := len(b.undoTimeline) - 1
// 	if lastCmd < 0 {
// 		return
// 	}
// 	b.undoTimeline[lastCmd]()
// 	b.undoTimeline = b.undoTimeline[:lastCmd]
// 	return
// }
//
// func (b *History) pushUndo(cmd func()) {
// 	b.redoTimeline = b.redoTimeline[:0]
// 	b.undoTimeline = append(b.undoTimeline, cmd)
// }
//
// func (b *History) writeCells(pos Coordinates, cells [][]termbox.Cell) {
// 	next := pos
// 	lastI := len(cells) - 1
// 	for i, row := range cells {
// 		for _, cell := range row {
// 			if cell.Ch == 0 {
// 				continue
// 			}
// 			next = b.buffer.InsertAt(next, cell.Ch)
// 		}
// 		if i < lastI {
// 			next = b.buffer.InsertAt(next, '\n')
// 		}
// 	}
// }
//
// // InsertAt calls the underlying buffer's InsertAt. Use Undo to reverse update.
// // See Undo and/or Buffer.InsertAt for more info.
// func (b *History) InsertAt(pos Coordinates, r rune) Coordinates {
// 	b.pushUndo(func() {
// 		_, n := b.buffer.TruncateCellAt(pos)
// 		if n == 0 {
// 			panic("corrupted undo buffer")
// 		}
// 	})
// 	return b.buffer.InsertAt(pos, r)
// }
//
// // InsertRowAt calls the underlying buffer's InsertRowAt. Use Undo to reverse update.
// // See Undo and/or Buffer.InsertRowAt for more info.
// func (b *History) InsertRowAt(i int) {
// 	b.pushUndo(func() {
// 		ok := b.buffer.TruncateRowAt(i)
// 		if !ok {
// 			panic("corrupted undo buffer")
// 		}
// 	})
// 	b.buffer.InsertRowAt(i)
// }
//
// // SetAttr calls the underlying buffer's SetAttr. Use Undo to reverse update.
// // See Undo and/or Buffer.SetAttr for more info.
// func (b *History) SetAttr(pos Coordinates, fg, bg termbox.Attribute) {
// 	prevFg, prevBg, _ := b.buffer.GetAttr(pos)
// 	b.pushUndo(func() { b.buffer.SetAttr(pos, prevFg, prevBg) })
// 	b.buffer.SetAttr(pos, fg, bg)
// }
//
// // ConflateRow calls the underlying buffer's ConflateRow. Use Undo to reverse update.
// // See Undo and/or Buffer.ConflateRow for more info.
// func (b *History) ConflateRow(i int) bool {
// 	at, ok := b.buffer.RowLastIdx(i)
// 	if !ok {
// 		return ok
// 	}
// 	b.pushUndo(func() { b.buffer.InsertAt(Coordinates{X: at + 1, Y: i}, '\n') })
// 	return b.buffer.ConflateRow(i)
// }
//
// // TruncateCellAt calls the underlying buffer's TruncateCellAt. Use Undo to reverse update.
// // See Undo and/or Buffer.TruncateCellAt for more info.
// func (b *History) TruncateCellAt(pos Coordinates) (orig termbox.Cell, n int) {
// 	var ok bool
// 	_, orig, ok = b.buffer.GetCellAt(pos)
// 	if !ok {
// 		n = 0
// 		return
// 	}
// 	b.pushUndo(func() { b.buffer.InsertAt(pos, orig.Ch) })
// 	orig, n = b.buffer.TruncateCellAt(pos)
// 	return
// }
//
// // TruncateFrom calls the underlying buffer's TruncateFrom. Use Undo to reverse update.
// // See Undo and/or Buffer.TruncateFrom for more info.
// func (b *History) TruncateFrom(from Coordinates) (ok bool) {
// 	to := b.buffer.NextWrite()
// 	cells := b.selector.Select(from, to)
// 	if cells == nil {
// 		return
// 	}
// 	b.pushUndo(func() { b.writeCells(from, cells) })
// 	return b.buffer.TruncateFrom(from)
// }
//
// // TruncateRowAt calls the underlying buffer's TruncateRowAt. Use Undo to reverse update.
// // See Undo and/or Buffer.TruncateRowAt for more info.
// func (b *History) TruncateRowAt(i int) (ok bool) {
// 	lastRow := b.buffer.Rows() - 1
// 	if i > lastRow {
// 		return
// 	}
// 	pos := Coordinates{X: 0, Y: i}
// 	cells := b.selector.SelectLine(pos, pos)
// 	if cells == nil {
// 		return
// 	}
// 	b.pushUndo(func() {
// 		b.InsertRowAt(pos.Y)
// 		b.writeCells(pos, cells)
// 	})
// 	return b.buffer.TruncateRowAt(i)
// }
//
// // TruncateRowFrom calls the underlying buffer's TruncateRowFrom. Use Undo to reverse update.
// // See Undo and/or Buffer.TruncateRowFrom for more info.
// func (b *History) TruncateRowFrom(pos Coordinates) (ok bool) {
// 	rowLastIdx, ok := b.buffer.RowLastIdx(pos.Y)
// 	if !ok || pos.X > rowLastIdx {
// 		return
// 	}
// 	finalPos := Coordinates{X: rowLastIdx, Y: pos.Y}
// 	cells := b.selector.Select(pos, finalPos)
// 	if cells == nil {
// 		return
// 	}
// 	b.pushUndo(func() { b.writeCells(pos, cells) })
// 	return b.buffer.TruncateRowFrom(pos)
// }
//
// // WriteAt calls the underlying buffer's WriteAt. Use Undo to reverse update.
// // See Undo and/or Buffer.WriteAt for more info.
// func (b *History) WriteAt(pos Coordinates, r rune) {
// 	pos, prev, ok := b.buffer.GetCellAt(pos)
// 	b.pushUndo(func() {
// 		_, n := b.TruncateCellAt(pos)
// 		if n == 0 {
// 			panic("corrupted undo buffer")
// 		}
// 		if ok {
// 			b.buffer.InsertAt(pos, prev.Ch)
// 		}
// 	})
// 	b.buffer.WriteAt(pos, r)
// }
//
// // WriteRune calls the underlying buffer's WriteRune. Use Undo to reverse update.
// // See Undo and/or Buffer.WriteRune for more info.
// func (b *History) WriteRune(r rune) Coordinates {
// 	pos := b.buffer.NextWrite()
// 	b.pushUndo(func() {
// 		b.buffer.TruncateCellAt(pos)
// 	})
// 	return b.buffer.WriteRune(r)
// }
//
// // WriteString calls the underlying buffer's WriteString. Use Undo to reverse update.
// // See Undo and/or Buffer.WriteString for more info.
// func (b *History) WriteString(p string) Coordinates {
// 	pos := b.buffer.NextWrite()
// 	b.pushUndo(func() {
// 		b.buffer.TruncateFrom(pos)
// 	})
// 	return b.buffer.WriteString(p)
// }
