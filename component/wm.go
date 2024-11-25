// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package component

import (
	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui"
	"unstable.build/go-tui/term"
)

// WindowManagerConfig represents the configuration for a WindowManager
// to be initialized.
type WindowManagerConfig struct {
	Frame         bool
	FrameAttr     term.Attributes
	ScrollBarAttr term.Attributes
	ScrollBarChar rune
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
	f.ScrollBarAttributes = wm.config.ScrollBarAttr
	f.ScrollBarChar = wm.config.ScrollBarChar
	return f
}

// Draw satisfies tui.Component
func (wm *WindowManager) Draw(w term.Writer) {
	wm.tree.Draw(w)
	for _, f := range wm.float {
		f.Draw(w)
	}
}

// FloatingWindows return a slice of all the open floating windows.
func (wm *WindowManager) FloatingWindows() (ret []Window) {
	for _, w := range wm.float {
		ret = append(ret, wm.nodeToWindow(w))
	}
	return
}

// TileTree returns a TileTree representing all the open tiles.
func (wm *WindowManager) TileTree() *TileTree {
	return &wm.tree
}

// DrawWindow can be used to arbitrarily draw floating windows returned
// by FloatingWindows. If win is not a floating window, this method will panic.
func (wm *WindowManager) DrawWindow(win Window, w term.Writer) {
	if f, ok := win.node.(*floatingNode); ok {
		f.Draw(w)
		return
	}
	wm.tree.DrawTile(win.node.(*TileNode), w)
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

// SizeTiles returns the number of tiled windows of this WindowManager.
func (wm *WindowManager) SizeTiles() int {
	return wm.tree.Size()
}

// SizeFloating returns the number of floating windows of this
// WindowManager.
func (wm *WindowManager) SizeFloating() int {
	return len(wm.float)
}

func (wm *WindowManager) nodeToWindow(node windowNode) Window {
	return Window{node: node, wm: wm}
}

// SplitHorizontal creates a new Window by splitting the height of win in two and
// initializes it with content. It returns true if it succeeds or false if
// this window is a floating window created via FloatingWindow and so it's not a tile.
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

// FloatingConfig abstracts configuration for
// creating floating windows.
type FloatingConfig struct {
	// Sets the alignment of the window.
	Alignment
	// Offset is to be applied to the position of the window
	// after alignment has been determined.
	Offset term.Coordinates
}

// FloatingWindow creates a floating window.
func (wm *WindowManager) FloatingWindow(
	content Floating, cfg FloatingConfig,
) Window {
	if wm.config.Frame {
		content = wm.withFrame(content)
	}
	f := newFloatingNode(wm, content, cfg, wm.width, wm.height)
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
	charset := FrameCharSetDefault()
	return WindowManagerConfig{
		Frame:         true,
		FrameAttr:     term.Attributes{},
		FrameCharSet:  charset,
		ScrollBarAttr: term.Attributes{Attrs: tcell.AttrBold},
	}
}
