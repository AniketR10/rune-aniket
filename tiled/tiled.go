package window

import "github.com/ernestrc/fractal"

type splitdir uint8

const (
	vertical splitdir = iota
	horizontal
)

type tnode struct {
	x         int
	y         int
	width     int
	height    int
	tiles     []fractal.Window
	direction splitdir
}

func newNode(direction splitdir, w *TiledWindow, x, y, width, height int) (t *tnode) {
	t = new(tnode)
	t.x, t.y, t.width, t.height = x, y, width, height
	t.tiles = []fractal.Window{w}
	t.direction = direction
	return
}

type TiledWindow struct {
	content fractal.Window
	node    *tnode
}

func newTiledWindow(content fractal.Window) (t *TiledWindow) {
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
			if err = ti.SetPosition(t.x+i*width, t.y); err != nil {
				return
			}
			continue
		}

		if err = ti.SetPosition(t.x, t.y+i*height); err != nil {
			return
		}
	}
	return
}

func (t *tnode) SetPosition(x, y int) error {
	t.x = x
	t.y = y
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
	return t.x, t.y
}

func (t *TiledWindow) Resize(width, height int) (err error) {
	return t.content.Resize(width, height)
}

func (t *TiledWindow) SetPosition(x, y int) error {
	return t.content.SetPosition(x, y)
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
