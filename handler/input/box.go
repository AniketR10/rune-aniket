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
	// MaxHeight defines the maximum height returned by Height.
	// If not set, then there is no max height.
	MaxHeight int
	// MinHeight defines the minimum height returned by Height.
	// If not set, then there is no minimum height.
	MinHeight         int
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
	placeholderStr component.Responsive
	scroll         *component.Scroll
	cursor         *text.Cursor
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
	if cfg.MaxHeight < cfg.MinHeight && cfg.MaxHeight != 0 {
		panic("max height cannot be smaller than min height")
	}
	// do not expose StringResponsiveConfig in BoxConfig because simple editor is not capable
	// of emulating NoSplitWords property.
	ed, scroll, cursor := text.NewSimpleHandler(buf, workspaceapi.URI{}, true /* wrap */, false /*commandBar */)
	frame := handler.NewFrame(ed)
	frame.Attributes = cfg.DefaultFrameAttr

	placeholderStr := component.StringResponsive(cfg.Placeholder,
		component.StringResponsiveConfig{StringConfig: cfg.PlaceholderConfig})
	placeholder := handler.NewFrame(
		handler.Nop(placeholderStr))
	placeholder.Attributes = cfg.DefaultFrameAttr

	i.buf = buf
	i.cfg = cfg
	i.frame = frame
	i.placeholderStr = placeholderStr
	i.placeholder = placeholder
	i.scroll = scroll
	i.cursor = cursor
}

// SetFrameAttr overrides the default frame attributes passed via BoxConfig.
func (i *Box) SetFrameAttr(attr term.Attributes) {
	i.frame.Attributes = attr
	i.placeholder.Attributes = attr
}

// Buffer returns this input.Box's underlying content buffer.
func (r *Box) Buffer() *cell.Buffer {
	return r.buf
}

// Reset resets this input box to its initial state.
func (r *Box) Reset() {
	r.cursor.MoveToScroll(term.Coordinates{})
	r.buf.Reset()
}

// Height satisfies component.Responsive.
func (i *Box) Height(width int) (ret int) {
	if width < 2 {
		return 0
	}
	width -= 2 // frame
	if i.buf.Size() == 0 {
		ret = i.placeholderStr.Height(width)
	} else {
		ret = i.scroll.Height(width)
		rows := i.scroll.Buffer().Rows()
		// if last visible row is "full", always return +1
		// to allow for cursor to fall in an empty row but within bounds.
		if rows > 0 {
			lastRowCols := i.scroll.Buffer().View().Columns(rows - 1)
			if lastRowCols != 0 && lastRowCols%width == 0 {
				ret++
			}
		}
	}
	ret += 2 // always leave one more for the cursor upon newline
	if ret > i.cfg.MaxHeight && i.cfg.MaxHeight != 0 {
		ret = i.cfg.MaxHeight
	}
	if ret < i.cfg.MinHeight && i.cfg.MinHeight != 0 {
		ret = i.cfg.MinHeight
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
