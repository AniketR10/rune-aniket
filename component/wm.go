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
	tree          TileTree
	width, height int
	float         []*floatingNode
	config        WindowManagerConfig
}

func (wm *WindowManager) withFrame(handler tui.Component) *Frame {
	f := NewFrame(handler)
	f.FrameCharSet = wm.config.FrameCharSet
	f.Attributes = wm.config.FrameAttr
	return f
}

// Draw satisfies tui.Component
func (wm *WindowManager) Draw(w term.Writer) {
	wm.tree.Draw(w)
	for _, f := range wm.float {
		f.Draw(w)
	}
}

// NewWindowManager allocates storage for a new WindowManager and initializes it.
func NewWindowManager(
	content tui.Component, config WindowManagerConfig,
) (*WindowManager, Window) {
	ret := new(WindowManager)
	win := ret.Init(content, config)
	return ret, win
}

// Init initializes this WindowManager with content and config.
func (wm *WindowManager) Init(
	content tui.Component, config WindowManagerConfig,
) (n Window) {
	wm.float = make([]*floatingNode, 0)
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
	for _, fw := range wm.float {
		op(wm.nodeToWindow(fw))
	}
}

// Resize satisfies tui.Component.
func (wm *WindowManager) Resize(width, height int) {
	wm.width, wm.height = width, height
	wm.tree.Resize(width, height)
	for _, fw := range wm.float {
		fw.SetMaxSize(width, height)
	}
}

// Size returns the size in windows of this WindowManager.
func (wm *WindowManager) Size() int {
	return wm.tree.Size() + len(wm.float)
}

func (wm *WindowManager) nodeToWindow(node windowNode) Window {
	return Window{node: node, wm: wm}
}

// SplitHorizontal creates a new Window by splitting the height of win in two and
// initializes it with content. It returns true if it succeeds or false if
// this window is a floating window created via NewFloatingWindow and so it's not a tile.
func (wm *WindowManager) SplitHorizontal(win Window, content tui.Component) (Window, bool) {
	t, ok := win.node.(*TileNode)
	if !ok {
		return Window{}, false
	}
	if wm.config.Frame {
		content = wm.withFrame(content)
	}
	node := wm.tree.SplitHorizontal(t, content)
	return wm.nodeToWindow(node), true
}

// SplitVertical creates a new Window by splitting the width of win in two and
// initializes it with content.
func (wm *WindowManager) SplitVertical(win Window, content tui.Component) (Window, bool) {
	t, ok := win.node.(*TileNode)
	if !ok {
		return Window{}, false
	}
	if wm.config.Frame {
		content = wm.withFrame(content)
	}
	node := wm.tree.SplitVertical(t, content)
	return wm.nodeToWindow(node), true
}

// WindowAt returns the window at pos.
func (wm *WindowManager) WindowAt(pos term.Coordinates) (Window, bool) {
	if pos.X < 0 || pos.Y < 0 || pos.X >= wm.width || pos.Y >= wm.height {
		return Window{}, false
	}

	float := -1
	for i, fw := range wm.float {
		fwpos := fw.Position()
		fwidth, fheight := fw.Width(), fw.Height()
		if pos.X >= fwpos.X && pos.Y >= fwpos.Y &&
			pos.X <= fwpos.X+fwidth && pos.Y <= fwpos.Y+fheight {
			float = i
		}
	}

	if float >= 0 {
		return wm.nodeToWindow(wm.float[float]), true
	}

	return wm.nodeToWindow(wm.tree.TileAt(pos)), true
}

// SetFrameCharSet sets the defaultframe border cells used
// to draw borders around tiles.  Note that this has no effect if WindowManager was
// initialized with border == false.
func (wm *WindowManager) SetFrameCharSet(b FrameCharSet) {
	if !wm.config.Frame {
		return
	}

	wm.config.FrameCharSet = b
	wm.Iterate(func(w Window) {
		w.node.Content().(*Frame).FrameCharSet = b
	})
}

// SetFrameAttr sets the default and focus window border attributes. Note that
// this has no effect if WindowManager was initialized with border == false.
func (wm *WindowManager) SetFrameAttr(attr term.Attributes) {
	if !wm.config.Frame {
		return
	}

	wm.config.FrameAttr = attr
	wm.Iterate(func(w Window) {
		w.node.Content().(*Frame).Attributes = attr
	})
}

// FloatingWindow creates a floating window.
func (wm *WindowManager) FloatingWindow(
	content tui.Component, at term.Coordinates, width, height int,
) Window {
	if wm.config.Frame {
		content = wm.withFrame(content)
	}
	f := newFloatingNode(wm, content, at, width, height, wm.width, wm.height)
	wm.float = append(wm.float, f)
	return wm.nodeToWindow(f)
}

func (wm *WindowManager) closeFloatingWindow(w *floatingNode) {
	for i, f := range wm.float {
		if f == w {
			wm.float = append(wm.float[:i], wm.float[i+1:]...)
			break
		}
	}
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
