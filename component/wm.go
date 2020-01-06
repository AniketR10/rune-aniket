package component

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
)

// WindowManagerConfig represents the configuration for a WindowManager
// to be initialized.
type WindowManagerConfig struct {
	Border     bool
	BorderAttr term.Attributes
	FrameCharSet
}

// WindowManager wraps a TileTree to provide an easier API.
type WindowManager struct {
	tree TileTree

	border     bool
	borderAttr term.Attributes

	// If border is set to true, the Frame cells can be configured
	// through the following properties.
	frmBorders FrameCharSet
}

func (wm *WindowManager) withFrame(handler tui.Component) tui.Component {
	f := NewFrame(handler)
	f.FrameCharSet = wm.frmBorders
	f.SetAttr(wm.borderAttr)
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
	wm.border = config.Border
	wm.borderAttr = config.BorderAttr
	wm.frmBorders = config.FrameCharSet

	if wm.border {
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
	if wm.border {
		content = wm.withFrame(content)
	}
	node := wm.tree.SplitHorizontal(win.node, content)
	return wm.nodeToWindow(node)
}

// SplitVertical creates a new Window by splitting the width of win in two and
// initializes it with content.
func (wm *WindowManager) SplitVertical(win Window, content tui.Component) Window {
	if wm.border {
		content = wm.withFrame(content)
	}
	node := wm.tree.SplitVertical(win.node, content)
	return wm.nodeToWindow(node)
}

// WindowAt returns the window at pos.
func (wm *WindowManager) WindowAt(pos term.Coordinates) Window {
	return wm.nodeToWindow(wm.tree.TileAt(pos))
}

// SetDefaultFrameCharSet sets the defaultframe border cells used
// to draw borders around tiles.  Note that this has no effect if WindowManager was
// initialized with border == false.
func (wm *WindowManager) SetDefaultFrameCharSet(b FrameCharSet) {
	if !wm.border {
		return
	}

	wm.tree.Iterate(func(node *TileNode) {
		node.Content().(*Frame).FrameCharSet = b
	})

	wm.frmBorders = b
}

// SetDefaultAttr sets the default and focus window border attributes. Note that
// this has no effect if WindowManager was initialized with border == false.
func (wm *WindowManager) SetDefaultAttr(attr term.Attributes) {
	if !wm.border {
		return
	}

	wm.borderAttr = attr
	wm.tree.Iterate(func(node *TileNode) {
		node.Content().(*Frame).SetAttr(wm.borderAttr)
	})
}

// DefaultWindowManagerConfig returns a sane WindowManagerConfig.
func DefaultWindowManagerConfig() WindowManagerConfig {
	return WindowManagerConfig{
		Border:       true,
		FrameCharSet: FrameCharSetDefault(),
	}
}
