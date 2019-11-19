package cell

// History adds Undo and Redo methods to a otherwise, irreversible Cells.
// type History struct {
// 	cells        Cells
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
// func (b *History) Insert(at term.Coordinates, str string) (from, to term.Coordinates) {
// }
//
// func (b *History) Delete(from, to term.Coordinates) (start, end term.Coordinates, str string) {
// }
//
// func (b *History) Reset() {
// }
//
// func (b *History) Rows() int {
// }
//
// func (b *History) Columns(row int) int {
// }
//
// func (b *History) Cell(term.Coordinates) (actual term.Coordinates, cell term.Cell) {
// }
//
// func (b *History) NextWrite() term.Coordinates {
// }
//
// func (b *History) RawCells() [][]term.Cell {
// }
//
// func (b *History) String() string {
// }
//
// func (b *History) ReadFrom(r io.Reader) (n int64, err error) {
// }
