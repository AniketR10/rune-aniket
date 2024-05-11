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
	}
}

type imgComp struct {
	img    image.Image
	config Config
	scroll *component.Scroll
}

func (c *imgComp) Draw(w term.Writer) {
	c.scroll.Draw(w)
}

func (c *imgComp) Resize(width, height int) {
	c.scroll.Buffer().Reset()
	Encode(c.scroll.Buffer(), width, height, c.img, c.config)
	c.scroll.Resize(width, height)
}
