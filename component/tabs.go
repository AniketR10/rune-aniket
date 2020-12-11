package component

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
)

var (
	defaultFocusAttr    = term.Attributes{Fg: term.ColorRed}
	defaultNonFocusAttr = term.Attributes{Fg: term.ColorDefault}
	defaultScrollAttr   = term.Attributes{Fg: term.ColorWhite}
	defaultFrameAttr    = term.Attributes{Fg: term.ColorRed}
	defaultSeparator    = "  "
)

type tab struct {
	name  string
	focus bool
}

// Tabs is a simple component that draws a list of component
// names which can in in Focus (highlighted) or not.
type Tabs struct {
	fileListBuf   *cell.Buffer
	fileListFrame tui.Component
	tabs          []*tab
	width, height int
	offsetIdx     int

	border         bool
	focusAttr      term.Attributes
	nonFocusAttr   term.Attributes
	backgroundAttr term.Attributes
	frameAttr      term.Attributes
	frameBorders   FrameCharSet
}

func newListFrame(
	scrollAttr, frameAttr term.Attributes, buf *cell.Buffer, border bool,
	frameBorders FrameCharSet,
) (content tui.Component) {
	scroll := NewScroll()
	scroll.InitWithBuffer(buf)
	scroll.Attributes = scrollAttr
	background := term.Cell{Bg: scroll.Attributes.Bg, Fg: scroll.Attributes.Fg}
	spanCfg := SpanConfig{
		ContentAlignment: SpanAlignmentCentered,
		PadVertical:      -1,
	}
	span := NewSpan(WithBackground(scroll, background), spanCfg)
	if !border {
		return span
	}

	f := NewFrame(span)
	f.FrameCharSet = frameBorders
	f.SetAttr(frameAttr)
	return f
}

// NewTabs allocates storage for a new instance of Tabs and initializes it.
func NewTabs() *Tabs {
	t := new(Tabs)
	t.Init()
	return t
}

// Init initializes this Tab and effectively resets all content.
func (t *Tabs) Init() {
	t.border = true
	t.focusAttr = defaultFocusAttr
	t.nonFocusAttr = defaultNonFocusAttr
	t.frameBorders = DefaultFrameCharSet()
	t.fileListBuf = cell.NewBuffer()
	t.fileListFrame = newListFrame(
		defaultScrollAttr, defaultFrameAttr, t.fileListBuf,
		t.border, t.frameBorders)
}

// SetAttr sets the attributes of the text in focus, text not in focus, the tabs
// frame and the tabs background.
func (t *Tabs) SetAttr(focusTab, tab, frame, background term.Attributes) {
	t.focusAttr = focusTab
	t.nonFocusAttr = tab
	t.backgroundAttr = background
	t.frameAttr = frame
	t.fileListFrame = newListFrame(t.backgroundAttr,
		t.frameAttr, t.fileListBuf, t.border, t.frameBorders)
	t.fileListFrame.Resize(t.width, t.height)
}

// SetBorder defines whether this Tabs draws a border around or not.
// The default is true.
func (t *Tabs) SetBorder(border bool) {
	t.border = border
	t.fileListFrame = newListFrame(
		t.backgroundAttr, t.frameAttr, t.fileListBuf, t.border, t.frameBorders)
	t.fileListFrame.Resize(t.width, t.height)
}

// SetFrameCharSet defines the characters used to draw a frame border.
// Note that this has no effect if border is set to false on this Tabs.
func (t *Tabs) SetFrameCharSet(fb FrameCharSet) {
	t.frameBorders = fb
	t.fileListFrame = newListFrame(
		t.backgroundAttr, t.frameAttr, t.fileListBuf, t.border, t.frameBorders)
	t.fileListFrame.Resize(t.width, t.height)
}

// Resize : tui.Component
func (t *Tabs) Resize(width, height int) {
	t.width, t.height = width, height
	t.fileListFrame.Resize(width, height)
}

// Draw : tui.Component
func (t *Tabs) Draw(w term.Writer) {
	t.fileListBuf.Reset()

	if t.width == 0 || t.height == 0 {
		return
	}

	var focusLen int
	var focusPos, next term.Coordinates
	for i, tab := range t.tabs {
		var attr term.Attributes
		if tab.focus {
			focusPos = next
			focusLen = len(tab.name)
			attr = t.focusAttr
		} else {
			attr = t.nonFocusAttr
		}

		_, next = t.fileListBuf.InsertStringWithAttr(
			next, tab.name, attr)

		if i < len(t.tabs)-1 {
			_, next = t.fileListBuf.InsertString(next, defaultSeparator)
		}
	}

	t.offsetIdx = 0
	lenSeparator := len(defaultSeparator)
	var effectiveWidth int
	if frame, ok := t.fileListFrame.(*Frame); ok {
		effectiveWidth, _ = frame.ContentSize()
	} else {
		effectiveWidth = t.width
	}
	for i := 0; i < len(t.tabs) && focusLen+focusPos.X > effectiveWidth; i++ {
		lenTab := len(t.tabs[i].name)
		if i < len(t.tabs)-1 {
			lenTab += lenSeparator
		}
		_, _, str := t.fileListBuf.Delete(term.Coordinates{}, term.Coordinates{X: lenTab - 1})
		focusPos.X -= len(str)
		next.X -= len(str)
		t.offsetIdx++
	}

	if t.offsetIdx > 0 {
		separator := ".." + defaultSeparator
		t.fileListBuf.InsertStringWithAttr(term.Coordinates{},
			separator, t.nonFocusAttr)
	}

	if t.fileListBuf.Columns(0) > effectiveWidth && effectiveWidth > 2 {
		from := term.Coordinates{X: effectiveWidth - 2}
		t.fileListBuf.TruncateRowFrom(from)
		t.fileListBuf.InsertStringWithAttr(from, "..", t.nonFocusAttr)
	}

	t.fileListFrame.Draw(w)
}

// ResetFocus resets the focus of all the tabs to false.
func (t *Tabs) ResetFocus() {
	for _, tab := range t.tabs {
		tab.focus = false
	}
}

// SetFocus sets the focus to tab with ID. If tab with ID does not exist,
// this method will panic.
func (t *Tabs) SetFocus(idx int) {
	t.tabs[idx].focus = true
}

// Add adds a tab with ID.
func (t *Tabs) Add(name string) int {
	if t.tabs == nil {
		t.tabs = make([]*tab, 1)
		t.tabs[0] = &tab{name: name, focus: true}
		return 0
	}
	idx := len(t.tabs)
	t.tabs = append(t.tabs, &tab{name: name})
	return idx
}

// Remove removes the tab with ID.
func (t *Tabs) Remove(idx int) bool {
	t.tabs = append(t.tabs[:idx], t.tabs[idx+1:]...)
	return true
}

// TabAt returns the ID of the tab at pos, or panics if pos is
// out of bounds.
func (t *Tabs) TabAt(pos term.Coordinates) (int, bool) {
	x := 0
	idx := -1

	for i, t := range t.tabs[t.offsetIdx:] {
		x += len(defaultSeparator)
		x += len(t.name)
		if x >= pos.X {
			idx = i
			break
		}
	}

	return t.offsetIdx + idx, idx != -1
}

// Tab returns the name of the tab at idx.
func (t *Tabs) Tab(idx int) (string, bool) {
	if idx >= len(t.tabs) {
		return "", false
	}
	return t.tabs[idx].name, true
}
