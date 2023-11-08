package component

import (
	"unstable.build/go-tui"
	"unstable.build/go-tui/term"
)

var _ tui.Component = (*Container)(nil)

// Container provides means to position components in a viewport,
// through a set of Rows that fill the available width and
// stack on top of each other. See Row for more details.
//
// A zero-valued container is ready for use.
type Container struct {
	rows          []*Row
	width, height int
	offset        int
}

// NewContainer allocates storage for a new container and initializes it.
func NewContainer() *Container {
	ret := new(Container)
	return ret
}

// AddRow appends a row to this Container.
func (g *Container) AddRow() *Row {
	row := NewRow()
	g.rows = append(g.rows, row)
	return row
}

// Resize satisfies tui.Component.
func (g *Container) Resize(width, height int) {
	g.width, g.height = width, height
	offset := -g.offset
	for _, row := range g.rows {
		rowHeight := row.Height(width)
		pos := term.Coordinates{Y: offset}
		row.Move(pos)
		if offset == height {
			// move above is enough to ensure that
			// virtual writer does not draw anything
			continue
		}
		if offset+rowHeight >= height {
			rowHeight = height - offset
		}
		// do not resize if it's before start
		if offset+rowHeight >= 0 && rowHeight > 0 {
			row.Resize(width, rowHeight)
		}
		offset += rowHeight
	}
}

// Draw satisfies tui.Component.
func (g *Container) Draw(w term.Writer) {
	// Height requirements could have changed
	// and that's something that we cannot determine from here
	// so it's better to always resize.
	g.Resize(g.width, g.height)

	// provides additional SetCell clipping for components past height
	vw := VirtualWriter(w, term.Coordinates{}, g.height, g.width)
	for _, row := range g.rows {
		row.Draw(vw)
	}
}

// ScrollUp scrolls the contents of this container up.
func (g *Container) ScrollUp() bool {
	if g.offset == 0 {
		return false
	}
	g.offset--
	return true
}

// ScrollDown scrolls the contents of this container down.
func (g *Container) ScrollDown() bool {
	if g.offset == g.maxOffset() {
		return false
	}
	g.offset++
	return true
}

func (g *Container) maxOffset() (ret int) {
	for _, row := range g.rows {
		rowHeight := row.Height(g.width)
		ret += rowHeight
	}
	if ret > 0 {
		ret--
	}
	return
}
