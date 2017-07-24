package tiled

import "github.com/ernestrc/fractal"

type splitdir uint8

const (
	vertical splitdir = iota
	horizontal
)

type tnode struct {
	fractal.Coordinates
	width     int
	height    int
	tiles     []fractal.Component
	direction splitdir
}

func newNode(direction splitdir, w *TiledWindow, x, y, width, height int) (t *tnode) {
	t = new(tnode)
	t.X, t.Y, t.width, t.height = x, y, width, height
	t.tiles = []fractal.Component{w}
	t.direction = direction
	return
}

type TiledWindow struct {
	content fractal.Component
	node    *tnode
}

func newTiledWindow(content fractal.Component) (t *TiledWindow) {
	t = new(TiledWindow)
	t.content = content
	return
}

func (t *tnode) Resize(width, height int) (err error) {
	t.height = height
	t.width = width

	if t.direction == vertical {
		width /= len(t.tiles)
	} else {
		height /= len(t.tiles)
	}

	for i, ti := range t.tiles {
		if err = ti.Resize(width, height); err != nil {
			return
		}
		if t.direction == vertical {
			if err = ti.Move(t.X+i*width, t.Y); err != nil {
				return
			}
			continue
		}

		if err = ti.Move(t.X, t.Y+i*height); err != nil {
			return
		}
	}
	return
}

func (t *tnode) Move(x, y int) error {
	t.X = x
	t.Y = y
	return t.Resize(t.width, t.height)
}

func (t *tnode) Draw(w fractal.Writer) (err error) {
	for _, ti := range t.tiles {
		if err = ti.Draw(w); err != nil {
			return
		}
	}

	return
}

func (t *tnode) Height() int {
	return t.height
}

func (t *tnode) Width() int {
	return t.width
}

func (t *tnode) Position() (int, int) {
	return t.X, t.Y
}

func (t *TiledWindow) Resize(width, height int) (err error) {
	return t.content.Resize(width, height)
}

func (t *TiledWindow) Move(x, y int) error {
	return t.content.Move(x, y)
}

func (t *TiledWindow) Draw(w fractal.Writer) (err error) {
	return t.content.Draw(w)
}

func (t *TiledWindow) Height() int {
	return t.content.Height()
}

func (t *TiledWindow) Width() int {
	return t.content.Width()
}

func (t *TiledWindow) Position() (int, int) {
	return t.content.Position()
}
