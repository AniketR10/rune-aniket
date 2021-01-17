package component

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
)

// WindowManagerConfig represents the configuration for a WindowManager
// to be initialized.
type WindowManagerConfig struct {
	Frame     bool
	FrameAttr term.Attributes
	FrameCharSet
}

// WindowManager wraps a TileTree to provide an easier API.
type WindowManager struct {
	tree   TileTree
	config WindowManagerConfig
}

func (wm *WindowManager) withFrame(handler tui.Component) tui.Component {
	f := NewFrame(handler)
	f.FrameCharSet = wm.config.FrameCharSet
	f.Attributes = wm.config.FrameAttr
	return f
}

// Draw satisfies tui.Component
func (wm *WindowManager) Draw(w term.Writer) {
	wm.tree.Draw(w)
}

// NewWindowManager allocates storage for a new WindowManager and initializes it.
func NewWindowManager(
	content tui.Component, config WindowManagerConfig,
) *WindowManager {
	ret := new(WindowManager)
	ret.Init(content, config)
	return ret
}

// Init initializes this WindowManager with content and config.
func (wm *WindowManager) Init(
	content tui.Component, config WindowManagerConfig,
) (n Window) {
	wm.config = config

	if wm.config.Frame {
		content = wm.withFrame(content)
	}
	node := wm.tree.Init(content)
	return wm.nodeToWindow(node)
}

// Iterate applies op to the content of all widnows of this WindowManager.
func (wm *WindowManager) Iterate(op func(Window)) {
	wm.tree.Iterate(func(node *TileNode) {
		op(wm.nodeToWindow(node))
	})
}

// Resize satisfies tui.Component.
func (wm *WindowManager) Resize(width, height int) {
	wm.tree.Resize(width, height)
}

// Size returns the size in windows of this WindowManager.
func (wm *WindowManager) Size() int {
	return wm.tree.Size()
}

func (wm *WindowManager) nodeToWindow(node *TileNode) Window {
	return Window{node: node, wm: wm}
}

// SplitHorizontal creates a new Window by splitting the height of win in two and
// initializes it with content.
func (wm *WindowManager) SplitHorizontal(win Window, content tui.Component) Window {
	if wm.config.Frame {
		content = wm.withFrame(content)
	}
	node := wm.tree.SplitHorizontal(win.node, content)
	return wm.nodeToWindow(node)
}

// SplitVertical creates a new Window by splitting the width of win in two and
// initializes it with content.
func (wm *WindowManager) SplitVertical(win Window, content tui.Component) Window {
	if wm.config.Frame {
		content = wm.withFrame(content)
	}
	node := wm.tree.SplitVertical(win.node, content)
	return wm.nodeToWindow(node)
}

// WindowAt returns the window at pos.
func (wm *WindowManager) WindowAt(pos term.Coordinates) Window {
	return wm.nodeToWindow(wm.tree.TileAt(pos))
}

// SetFrameCharSet sets the defaultframe border cells used
// to draw borders around tiles.  Note that this has no effect if WindowManager was
// initialized with border == false.
func (wm *WindowManager) SetFrameCharSet(b FrameCharSet) {
	if !wm.config.Frame {
		return
	}

	wm.config.FrameCharSet = b
	wm.tree.Iterate(func(node *TileNode) {
		node.Content().(*Frame).FrameCharSet = b
	})
}

// SetFrameAttr sets the default and focus window border attributes. Note that
// this has no effect if WindowManager was initialized with border == false.
func (wm *WindowManager) SetFrameAttr(attr term.Attributes) {
	if !wm.config.Frame {
		return
	}

	wm.config.FrameAttr = attr
	wm.tree.Iterate(func(node *TileNode) {
		node.Content().(*Frame).Attributes = attr
	})
}

// DefaultWindowManagerConfig returns a sane WindowManagerConfig.
func DefaultWindowManagerConfig() WindowManagerConfig {
	return WindowManagerConfig{
		Frame: true,
		FrameAttr: term.Attributes{
			Fg: term.ColorDefault,
			Bg: term.ColorDefault,
		},
		FrameCharSet: FrameCharSetDefault(),
	}
}
