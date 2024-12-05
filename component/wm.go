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

	// this is cached and calculated to figure out
	// how to offset windows when there are minimized floating windows.
	minimizedDirty  bool
	minimizedOffset term.Coordinates
	minimizedHeight int
	minimizedWidth  int
	minimizedPos    map[uint64]windowPos
}

// Draw satisfies tui.Component
func (wm *WindowManager) Draw(w term.Writer) {
	wm.Iterate(func(win Window) {
		wm.DrawWindow(win, w)
	})
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
	if wm.minimizedDirty {
		wm.Resize(wm.width, wm.height)
	}
	f, ok := win.node.(*floatingNode)
	if !ok {
		w = VirtualWriter{
			Writer: w,
			Offset: wm.minimizedOffset,
			Height: wm.height,
			Width:  wm.width,
		}
		wm.tree.DrawTile(win.node.(*TileNode), w)
		return
	}

	if f.minimized == 0 {
		f.Draw(w)
		return
	}

	winPos, ok := wm.minimizedPos[f.ID()]
	if !ok {
		panic("corrupt WindowManager: no pre-calculated minimized position for floating node")
	}

	pos := winPos.pos
	switch f.minimized {
	case SpanAlignmentTop:
		w.SetCell(term.Coordinates{X: 0, Y: pos}, term.Cell{
			Width:      1,
			Ch:         wm.config.TopLeft,
			Attributes: wm.config.FrameAttr,
		})
		for x := 1; x < wm.width-1; x++ {
			w.SetCell(term.Coordinates{X: x, Y: pos}, term.Cell{
				Width:      1,
				Ch:         wm.config.HorizontalTop,
				Attributes: wm.config.FrameAttr,
			})
		}
		w.SetCell(term.Coordinates{X: wm.width - 1, Y: pos}, term.Cell{
			Width:      1,
			Ch:         wm.config.TopRight,
			Attributes: wm.config.FrameAttr,
		})
	case SpanAlignmentBottom:
		bottomPos := wm.height - pos - 1
		w.SetCell(term.Coordinates{X: 0, Y: bottomPos}, term.Cell{
			Width:      1,
			Ch:         wm.config.BottomLeft,
			Attributes: wm.config.FrameAttr,
		})
		for x := 1; x < wm.width-1; x++ {
			w.SetCell(term.Coordinates{X: x, Y: bottomPos}, term.Cell{
				Width:      1,
				Ch:         wm.config.HorizontalBottom,
				Attributes: wm.config.FrameAttr,
			})
		}
		w.SetCell(term.Coordinates{X: wm.width - 1, Y: bottomPos}, term.Cell{
			Width:      1,
			Ch:         wm.config.BottomRight,
			Attributes: wm.config.FrameAttr,
		})
	case SpanAlignmentLeft:
		yOffset := wm.minimizedOffset.Y
		height := wm.minimizedHeight
		w.SetCell(term.Coordinates{X: pos, Y: yOffset}, term.Cell{
			Width:      1,
			Ch:         wm.config.TopLeft,
			Attributes: wm.config.FrameAttr,
		})
		for y := 1 + yOffset; y < yOffset+height-1; y++ {
			w.SetCell(term.Coordinates{X: pos, Y: y}, term.Cell{
				Width:      1,
				Ch:         wm.config.VerticalLeft,
				Attributes: wm.config.FrameAttr,
			})
		}
		w.SetCell(term.Coordinates{X: pos, Y: yOffset + height - 1}, term.Cell{
			Width:      1,
			Ch:         wm.config.BottomLeft,
			Attributes: wm.config.FrameAttr,
		})
	case SpanAlignmentRight:
		rightPos := wm.width - pos - 1
		yOffset := wm.minimizedOffset.Y
		height := wm.minimizedHeight
		w.SetCell(term.Coordinates{X: rightPos, Y: yOffset}, term.Cell{
			Width:      1,
			Ch:         wm.config.TopRight,
			Attributes: wm.config.FrameAttr,
		})
		for y := 1 + yOffset; y < yOffset+height-1; y++ {
			w.SetCell(term.Coordinates{X: rightPos, Y: y}, term.Cell{
				Width:      1,
				Ch:         wm.config.VerticalRight,
				Attributes: wm.config.FrameAttr,
			})
		}
		w.SetCell(term.Coordinates{X: rightPos, Y: yOffset + height - 1}, term.Cell{
			Width:      1,
			Ch:         wm.config.BottomRight,
			Attributes: wm.config.FrameAttr,
		})
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
	wm.minimizedDirty = true
	wm.config = config
	wm.minimizedPos = make(map[uint64]windowPos)

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
	wm.calculateMinimizedOffsets()
	wm.tree.Resize(wm.minimizedWidth, wm.minimizedHeight)
	for _, fw := range wm.float {
		fw.SetMaxSize(wm.minimizedWidth, wm.minimizedHeight)
	}
	wm.minimizedDirty = false
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

	// first return any floating window that might be rendered over everything else
	floating := -1
	for i, fw := range wm.float {
		if fw.minimized != 0 {
			continue
		}
		fwpos := fw.Position()
		fwidth, fheight := fw.Width(), fw.Height()
		if pos.X >= fwpos.X && pos.Y >= fwpos.Y &&
			pos.X <= fwpos.X+fwidth && pos.Y <= fwpos.Y+fheight {
			floating = i
		}
	}
	if floating >= 0 {
		return wm.nodeToWindow(wm.float[floating]), true
	}

	// return any minimized top windows at position
	if pos.Y < wm.minimizedOffset.Y {
		for _, winPos := range wm.minimizedPos {
			if winPos.win.node.(*floatingNode).minimized == SpanAlignmentTop &&
				winPos.pos == pos.Y {
				return winPos.win, true
			}
		}
		panic("corrupted wm: minimized window under offset not found")
	}

	// return any minimized bottom windows at position
	if pos.Y >= wm.minimizedOffset.Y+wm.minimizedHeight {
		pos.Y -= (wm.minimizedOffset.Y + wm.minimizedHeight)
		for _, winPos := range wm.minimizedPos {
			if winPos.win.node.(*floatingNode).minimized == SpanAlignmentBottom &&
				winPos.pos == pos.Y {
				return winPos.win, true
			}
		}
		panic("corrupted wm: minimized window above offset not found")
	}

	// return any minimized left windows at position
	if pos.X < wm.minimizedOffset.X {
		for _, winPos := range wm.minimizedPos {
			if winPos.win.node.(*floatingNode).minimized == SpanAlignmentLeft &&
				winPos.pos == pos.X {
				return winPos.win, true
			}
		}
		panic("corrupted wm: minimized window before offset not found")
	}

	// return any minimized right windows at position
	if pos.X >= wm.minimizedOffset.X+wm.minimizedWidth {
		pos.X -= (wm.minimizedOffset.X + wm.minimizedWidth)
		for _, winPos := range wm.minimizedPos {
			if winPos.win.node.(*floatingNode).minimized == SpanAlignmentRight &&
				winPos.pos == pos.X {
				return winPos.win, true
			}
		}
		panic("corrupted wm: minimized window after offset not found")
	}

	pos.Y -= wm.minimizedOffset.Y
	pos.X -= wm.minimizedOffset.X
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
	wm.minimizedDirty = true
	return wm.nodeToWindow(f)
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

func (wm *WindowManager) withFrame(handler tui.Component) *Frame {
	f := NewFrame(handler)
	f.FrameCharSet = wm.config.FrameCharSet
	f.Attributes = wm.config.FrameAttr
	f.ScrollBarAttributes = wm.config.ScrollBarAttr
	f.ScrollBarChar = wm.config.ScrollBarChar
	return f
}

func (wm *WindowManager) closeFloatingWindow(w *floatingNode) {
	for i, f := range wm.float {
		if f == w {
			wm.float = append(wm.float[:i], wm.float[i+1:]...)
			wm.minimizedDirty = true
			break
		}
	}
}

func (wm *WindowManager) nodeToWindow(node windowNode) Window {
	return Window{node: node, wm: wm}
}

func (wm *WindowManager) calculateMinimizedOffsets() {
	clear(wm.minimizedPos)

	var offsetLeft, offsetTop, offsetBottom, offsetRight int
	for _, win := range wm.FloatingWindows() {
		switch win.node.(*floatingNode).minimized {
		case SpanAlignmentTop:
			wm.minimizedPos[win.ID()] = windowPos{pos: offsetTop, win: win}
			offsetTop++
		case SpanAlignmentBottom:
			wm.minimizedPos[win.ID()] = windowPos{pos: offsetBottom, win: win}
			offsetBottom++
		case SpanAlignmentLeft:
			wm.minimizedPos[win.ID()] = windowPos{pos: offsetLeft, win: win}
			offsetLeft++
		case SpanAlignmentRight:
			wm.minimizedPos[win.ID()] = windowPos{pos: offsetRight, win: win}
			offsetRight++
		default:
		}
	}
	wm.minimizedOffset = term.Coordinates{Y: offsetTop, X: offsetLeft}
	wm.minimizedHeight = wm.height - offsetBottom - offsetTop
	wm.minimizedWidth = wm.width - offsetRight - offsetLeft
}

type windowPos struct {
	win Window
	pos int
}
