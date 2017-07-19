package window

import "github.com/ernestrc/fractal/config"

type splitdir uint8

const (
	vertical splitdir = iota
	horizontal
)

// TODO type Attribute uint16
// TODO
// TODO type CellWriter interface {
// TODO 	SetCell(x, y int, fg, bg Attribute) error
// TODO }

type tile interface {
	Resize(width, height int) error
	MoveTo(x, y int) error
	Draw( /*w CellWriter*/ ) error
	Height() int
	Width() int
	Position() (int, int)
}

type tnode struct {
	x         int
	y         int
	width     int
	height    int
	tiles     []tile
	direction splitdir
}

func newNode(direction splitdir, w *TiledWindow, x, y, width, height int) (t *tnode) {
	t = new(tnode)
	t.x, t.y, t.width, t.height = x, y, width, height
	t.tiles = []tile{w}
	t.direction = direction
	return
}

type TiledWindow struct {
	Window
	node *tnode
}

func newTiledWindow(cfg *config.Config) (t *TiledWindow) {
	if cfg == nil {
		panic("configuration cannot be nil")
	}
	t = new(TiledWindow)
	t.Init(nil, cfg)
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
			if err = ti.MoveTo(t.x+i*width, t.y); err != nil {
				return
			}
			continue
		}

		if err = ti.MoveTo(t.x, t.y+i*height); err != nil {
			return
		}
	}
	return
}

func (t *tnode) MoveTo(x, y int) error {
	t.x = x
	t.y = y
	return t.Resize(t.width, t.height)
}

func (t *tnode) Draw( /*w CellWriter*/ ) (err error) {
	for _, ti := range t.tiles {
		if err = ti.Draw(); err != nil {
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
