package input

import (
	"unstable.build/go-tui"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
)

var _ component.Responsive = (*Box)(nil)
var _ tui.Handler = (*Box)(nil)

// BoxConfig holds configuration for initializing an Box.
type BoxConfig struct {
	MaxHeight         int
	Placeholder       string
	PlaceholderConfig component.StringConfig
	DefaultFrameAttr  term.Attributes
	ContentConfig     component.StringConfig
}

// Box is an input box that essentially collects user input.
type Box struct {
	buf         *cell.Buffer
	cfg         BoxConfig
	frame       *handler.Frame
	placeholder *handler.Frame
	// used to assist in calculate height
	scroll *component.Scroll
	cursor *text.Cursor
}

// NewBox allocates storage for a new Box and
// initializes it.
func NewBox(buf *cell.Buffer, cfg BoxConfig) *Box {
	ret := new(Box)
	ret.Init(buf, cfg)
	return ret
}

// Init initializes this Box with the given config and cell.Buffer,
// which is used to share the contents of the input box with clients.
func (i *Box) Init(buf *cell.Buffer, cfg BoxConfig) {
	// do not expose StringResponsiveConfig in BoxConfig because simple editor is not capable
	// of emulating NoSplitWords property.
	ed, scroll, cursor := text.NewSimpleHandler(buf, workspaceapi.URI{}, true /* wrap */, false /*commandBar */)
	frame := handler.NewFrame(ed)
	frame.Attributes = cfg.DefaultFrameAttr
	placeholder := handler.NewFrame(
		handler.Nop(component.StringResponsive(cfg.Placeholder,
			component.StringResponsiveConfig{StringConfig: cfg.PlaceholderConfig})))
	placeholder.Attributes = cfg.DefaultFrameAttr

	i.buf = buf
	i.cfg = cfg
	i.frame = frame
	i.placeholder = placeholder
	i.scroll = scroll
	i.cursor = cursor
}

// SetFrameAttr overrides the default frame attributes passed via BoxConfig.
func (i *Box) SetFrameAttr(attr term.Attributes) {
	i.frame.Attributes = attr
	i.placeholder.Attributes = attr
}

// Height satisfies component.Responsive.
func (i *Box) Height(width int) int {
	ret := i.scroll.Height(width - 2)
	ret += 2 + 1 // always leave one more for the cursor upon newline
	if ret > i.cfg.MaxHeight && i.cfg.MaxHeight != 0 {
		ret = i.cfg.MaxHeight
	}
	return ret
}

// Resize satisfies tui.Component.
func (i *Box) Resize(width, height int) {
	i.frame.Resize(width, height)
	i.placeholder.Resize(width, height)
	// due to the nature of responsive components:
	// first we update, then we measure the new height,
	// then we resize accordingly. The underlying scroll
	// might have been scrolled due to the update.
	pos := i.cursor.CursorAtScroll()
	i.scroll.SeekTo(i.scroll.Offset())
	i.cursor.MoveToScroll(pos)
}

// Draw satisfies tui.Component.
func (i *Box) Draw(w term.Writer) {
	if i.buf.Size() == 0 {
		i.placeholder.Draw(w)
	} else {
		i.frame.Draw(w)
	}
}

// Handle satisfies tui.Handler.
func (i *Box) Handle(ev term.Event) (exit, handled bool) {
	return i.frame.Handle(ev)
}

// Cursor satisfies tui.Handler.
func (i *Box) Cursor() (pos term.Coordinates, show bool) {
	return i.frame.Cursor()
}

// Man satisfies tui.Handler.
func (i *Box) Man() tui.Manual {
	return tui.Manual{}
}
