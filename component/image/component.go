package image

import (
	"image"

	"unstable.build/go-tui"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
)

// New returns a tui.Component that renders the given image
// with the given config.
func New(img image.Image, config Config) tui.Component {
	return &imgComp{
		img:    img,
		config: config,
		scroll: component.NewScroll(cell.NewBuffer()),
		dirty:  true,
	}
}

type imgComp struct {
	img           image.Image
	config        Config
	scroll        *component.Scroll
	dirty         bool
	width, height int
}

func (c *imgComp) Draw(w term.Writer) {
	if c.dirty {
		c.scroll.Buffer().Reset()
		Encode(c.scroll.Buffer(), c.width, c.height, c.img, c.config)
	}
	c.scroll.Draw(w)
}

func (c *imgComp) Resize(width, height int) {
	c.dirty = true
	c.width = width
	c.height = height
	c.scroll.Resize(width, height)
}
