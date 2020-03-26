package component

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
)

// Virtual wraps a component to
// provide virtual coordinates and write bound checking.
// It exposes Move which can be used to move the inner component
// in the virtual coordinate space.
type Virtual struct {
	C             tui.Component
	pos           term.Coordinates
	height, width int
}

type virtualWriter struct {
	writer        tui.Writer
	offset        term.Coordinates
	height, width int
}

func (w *virtualWriter) isOutOfBounds(pos term.Coordinates) bool {
	return pos.X >= w.width || pos.Y >= w.height || pos.Y < 0 || pos.X < 0
}

func (w *virtualWriter) assertNotOutOfBounds(pos term.Coordinates) {
	/*
		panic: bounds check: component out of bounds: tried to write at 96,3 but width is 96 and height is 54

		goroutine 1 [running]:
		github.com/ernestrc/go-tui/component.(*virtualWriter).assertNotOutOfBounds(0xc000236cc0, 0x60, 0x3)
			/home/ernestrc/src/go-tui/component/virtual.go:32 +0x199
		github.com/ernestrc/go-tui/component.(*virtualWriter).SetCell(0xc000236cc0, 0x60, 0x3, 0x70)
			/home/ernestrc/src/go-tui/component/virtual.go:38 +0x43
		github.com/ernestrc/go-tui/component.(*virtualWriter).SetCell(0xc000236cf0, 0x5f, 0x2, 0x70)
			/home/ernestrc/src/go-tui/component/virtual.go:40 +0x97
		github.com/ernestrc/go-tui/component.(*Scroll).drawFast(0xc000170420, 0xb42660, 0xc000236cf0)
			/home/ernestrc/src/go-tui/component/scroll.go:261 +0x11e
		github.com/ernestrc/go-tui/component.(*Scroll).Draw(0xc000170420, 0xb42660, 0xc000236cf0)
			/home/ernestrc/src/go-tui/component/scroll.go:367 +0x7e
		github.com/ernestrc/go-tui/handler.(*Less).Draw(0xc000170420, 0xb42660, 0xc000236cf0)
			/home/ernestrc/src/go-tui/handler/less.go:201 +0x48
		github.com/ernestrc/go-tui/editor/vi.(*Vi).Draw(0xc000170400, 0xb42660, 0xc000236cf0)
			/home/ernestrc/src/go-tui/editor/vi/handler.go:100 +0x45
		github.com/ernestrc/go-tui/browser.(*browserBuffer).Draw(0xc000021f40, 0xb42660, 0xc000236cf0)
			/home/ernestrc/src/go-tui/browser/buffer.go:22 +0x48
		github.com/ernestrc/go-tui/component.(*Virtual).Draw(...)
			/home/ernestrc/src/go-tui/component/virtual.go:72
		github.com/ernestrc/go-tui/component.(*Frame).Draw(0xc0003c66c0, 0xb42660, 0xc000236cc0)
			/home/ernestrc/src/go-tui/component/frame.go:137 +0x319
		github.com/ernestrc/go-tui/component.(*TileNode).Draw(0xc0003a7bd0, 0xb42660, 0xc000236cc0)
			/home/ernestrc/src/go-tui/component/tile.go:140 +0x13f
		github.com/ernestrc/go-tui/component.(*Virtual).Draw(...)
			/home/ernestrc/src/go-tui/component/virtual.go:72
		github.com/ernestrc/go-tui/component.(*TileNode).Draw(0xc0003a7d60, 0xb42660, 0xc000236c00)
			/home/ernestrc/src/go-tui/component/tile.go:145 +0xa4
		github.com/ernestrc/go-tui/component.(*Virtual).Draw(...)
			/home/ernestrc/src/go-tui/component/virtual.go:72
		github.com/ernestrc/go-tui/component.(*TileNode).Draw(0xc00011b360, 0xb42660, 0xc000236bd0)
			/home/ernestrc/src/go-tui/component/tile.go:145 +0xa4
		github.com/ernestrc/go-tui/component.(*TileTree).Draw(...)
			/home/ernestrc/src/go-tui/component/tile.go:56
		github.com/ernestrc/go-tui/handler.(*WindowManager).Draw(0xc00011b360, 0xb42660, 0xc000236bd0)
			/home/ernestrc/src/go-tui/handler/wm.go:322 +0x41
		github.com/ernestrc/go-tui/component.(*Virtual).Draw(...)
			/home/ernestrc/src/go-tui/component/virtual.go:72
		github.com/ernestrc/go-tui/component.(*FrameUnion).Draw(0xc000021f80, 0xb427e0, 0xf71930)
			/home/ernestrc/src/go-tui/component/frame_union.go:56 +0x130
		github.com/ernestrc/go-tui/browser.(*Handler).Draw(0xc00000c5a0, 0xb427e0, 0xf71930)
			/home/ernestrc/src/go-tui/browser/handler.go:614 +0x4a
		github.com/ernestrc/go-tui.redraw(0xb44120, 0xc00000c5a0, 0xb427e0, 0xf71930, 0x0, 0x0)
			/home/ernestrc/src/go-tui/runtime.go:19 +0x7a
		github.com/ernestrc/go-tui.run(0xb44120, 0xc00000c5a0, 0xb3b440, 0xc000025208, 0xb427e0, 0xf71930, 0x0, 0x0)
			/home/ernestrc/src/go-tui/runtime.go:50 +0x11e
		github.com/ernestrc/go-tui.RunWithLocker(...)
			/home/ernestrc/src/go-tui/tui.go:105
		main.main()
			/home/ernestrc/src/go-tui/cmd/six/main.go:99 +0x60d
	*/
	// FIXME data race possibly if w.isOutOfBounds(pos) {
	// FIXME data race possibly 	panic(fmt.Sprintf("bounds check: component out of bounds: tried to write at %d,%d but width is %d and height is %d",
	// FIXME data race possibly 		pos.X, pos.Y, w.width, w.height))
	// FIXME data race possibly }
}

func (w *virtualWriter) SetCell(pos term.Coordinates, c term.Cell) {
	w.assertNotOutOfBounds(pos)
	pos = term.Coordinates{X: w.offset.X + pos.X, Y: w.offset.Y + pos.Y}
	w.writer.SetCell(pos, c)
}

func (w *virtualWriter) Flush() error {
	return w.writer.Flush()
}

func (w *virtualWriter) Clear(attr term.Attributes) error {
	return w.writer.Clear(attr)
}

func (w *virtualWriter) SetCursor(pos term.Coordinates) {
	w.writer.SetCursor(pos)
}

// Resize resizes the underlying component and stores size
// to perform bound checking on Draw.
func (c *Virtual) Resize(width, height int) {
	c.width = width
	c.height = height
	c.C.Resize(width, height)
}

// Draw uses a virtual writer to perform bound checking and
// if successful draw the inner component in the virtual coordinate space.
func (c *Virtual) Draw(writer tui.Writer) {
	writer = &virtualWriter{
		writer: writer,
		offset: c.pos,
		height: c.height,
		width:  c.width,
	}
	c.C.Draw(writer)
}

// Move changes the position of this virtual component
// in the virtual coordinate space.
func (c *Virtual) Move(pos term.Coordinates) {
	c.pos = pos
}

// Width returns the width set in last Resize.
func (c *Virtual) Width() int {
	return c.width
}

// Height returns the height set in last Resize.
func (c *Virtual) Height() int {
	return c.height
}

// Position returns this virtual component's position
// in the virtual coordinate space.
func (c *Virtual) Position() term.Coordinates {
	return c.pos
}
