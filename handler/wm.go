package handler

import (
	"errors"
	"fmt"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/term"
)

// TileNode represents a tile in a WindowManager.
type TileNode struct {
	wm   *WindowManager
	node *component.TileNode
}

// WindowManager implements Handler as a tiled window manager.
type WindowManager struct {
	tree  component.TileTree
	focus TileNode

	border     bool
	borderAttr term.Attributes
	focusAttr  term.Attributes

	// If border is set to true, the Frame cells can be configured
	// through the following properties.
	frmBorders component.FrameBorders
}

// NewWindowManager allocates storage for a new WindowManager and initializes it with the
// given handler. If border is true, it will draw a border around every tile.
func NewWindowManager(handler tui.Handler, border bool) (wm *WindowManager) {
	wm = new(WindowManager)
	wm.Init(handler, border)
	return
}

func (wm *WindowManager) withFrame(handler tui.Handler) tui.Handler {
	f := NewFrame(handler)
	f.FrameBorders = wm.frmBorders
	f.SetAttr(wm.borderAttr)
	return f
}

func (wm *WindowManager) newNode(t *component.TileNode) TileNode {
	return TileNode{wm: wm, node: t}
}

// Init initializes this WindowManager with the given handler. If border is true, it will draw
// a border around every tile.
func (wm *WindowManager) Init(handler tui.Handler, border bool) {
	if border {
		wm.frmBorders = component.DefaultFrameBorders()
		wm.borderAttr.Fg = term.ColorDefault
		wm.borderAttr.Bg = term.ColorDefault
		wm.focusAttr.Fg = term.ColorRed
		wm.focusAttr.Bg = term.ColorDefault
		handler = wm.withFrame(handler)
	}
	tile := wm.tree.Init(handler)
	wm.border = border
	wm.focus = wm.newNode(tile)
	wm.SetFocus(wm.focus)
	return
}

// SetFrameBorders sets the frame border cells used to draw borders around tiles.
// Note that this has no effect if WindowManager was
// initialized with border == false.
func (wm *WindowManager) SetFrameBorders(b component.FrameBorders) {
	if !wm.border {
		return
	}

	wm.tree.Iterate(func(node *component.TileNode) {
		node.Content().(*Frame).FrameBorders = b
	})

	wm.frmBorders = b
}

// SetAttr sets the default and focus window border attributes. Note that
// this has no effect if WindowManager was initialized with border == false.
func (wm *WindowManager) SetAttr(standard, focus term.Attributes) {
	if !wm.border {
		return
	}

	wm.borderAttr = standard
	wm.focusAttr = focus

	wm.tree.Iterate(func(node *component.TileNode) {
		node.Content().(*Frame).SetAttr(wm.borderAttr)
	})

	wm.focus.node.Content().(*Frame).SetAttr(wm.focusAttr)
}

// Handle : Handler
func (wm *WindowManager) Handle(ev term.Event) (exit bool, handled bool) {
	if ev.Type == term.EventKey && ev.Mod == term.ModAlt {
		switch ev.Ch {
		case 'q':
			exit = true
		case 'k':
			wm.FocusUp()
		case 'j':
			wm.FocusDown()
		case 'h':
			wm.FocusLeft()
		case 'l':
			wm.FocusRight()
		}
		handled = true
		return
	}

	if ev.Type == term.EventMouse {
		mousePos := term.Coordinates{X: ev.MouseX, Y: ev.MouseY}
		childAtMouse := wm.tree.TileAt(mousePos)
		if wm.Focus().node != childAtMouse {
			if ev.Key == term.MouseLeft {
				wm.SetFocus(wm.newNode(childAtMouse))
			}
			return
		}
		offset := wm.tree.TilePosition(childAtMouse)
		ev.MouseX -= offset.X
		ev.MouseY -= offset.Y
	}

	var hexit bool
	hexit, handled = wm.FocusContent().Handle(ev)

	// if handler in focus wants to exit, close the window,
	// or signal exit to upstream handler if it was last window
	if hexit {
		if exit = wm.tree.Size() == 1; exit {
			return
		}
		curr := wm.focus
		wm.ShiftFocus()
		curr.Close()
	}

	return
}

// SplitVertical creates a new vertical split over the tile currently in focus.
func (wm *WindowManager) SplitVertical(h tui.Handler) TileNode {
	if wm.border {
		h = wm.withFrame(h)
	}
	return wm.newNode(wm.tree.SplitVertical(wm.focus.node, h))
}

// SplitHorizontal creates a new horizontal split over the tile currently in focus.
func (wm *WindowManager) SplitHorizontal(h tui.Handler) TileNode {
	if wm.border {
		h = wm.withFrame(h)
	}
	return wm.newNode(wm.tree.SplitHorizontal(wm.focus.node, h))

}

func (wm *WindowManager) switchFocus(tileFn func(TileNode) (TileNode, bool)) bool {
	tile, ok := tileFn(wm.focus)
	if !ok {
		return false
	}
	wm.SetFocus(wm.newNode(tile.node))
	return true
}

// FocusLeft switches the focus to the tile on the left side of the tile in focus
// If the tile in focus is the left-most tile in this window manager, then this method does nothing.
func (wm *WindowManager) FocusLeft() bool {
	return wm.switchFocus((TileNode).TileLeft)
}

// FocusRight switches the focus to the tile on the right side of the tile in focus
// If the tile in focus is the right-most tile in this window manager, then this method does nothing.
func (wm *WindowManager) FocusRight() bool {
	return wm.switchFocus((TileNode).TileRight)
}

// FocusUp switches the focus to the tile above the tile in focus
// If the tile in focus is the up-most tile in this window manager, then this method does nothing.
func (wm *WindowManager) FocusUp() bool {
	return wm.switchFocus((TileNode).TileUp)
}

// FocusDown switches the focus to the tile beneath the tile in focus
// If the tile in focus is the down-most tile in this window manager, then this method does nothing.
func (wm *WindowManager) FocusDown() bool {
	return wm.switchFocus((TileNode).TileDown)
}

// Content returns the content of t.
func (wm *WindowManager) Content(t TileNode) tui.Handler {
	if t.wm != wm {
		panic(fmt.Sprintf("Tile does not belong to"+
			"this window manager: %p vs %p", wm, t.wm))
	}
	if wm.border {
		return t.node.Content().(*Frame).Content().(tui.Handler)
	}
	return t.node.Content().(tui.Handler)
}

// FocusContent returns the current focus content.
func (wm *WindowManager) FocusContent() tui.Handler {
	return wm.Content(wm.focus)
}

// Focus returns the tile currently in focus.
func (wm *WindowManager) Focus() TileNode {
	return wm.focus
}

// ShiftFocus attempts to shift to focus to another tile. It returns
// false if focus did not shift to another tile because there aren't any tiles left.
func (wm *WindowManager) ShiftFocus() (ok bool) {
	ok = wm.FocusLeft()
	if ok {
		return
	}
	ok = wm.FocusUp()
	if ok {
		return
	}
	ok = wm.FocusRight()
	if ok {
		return
	}
	ok = wm.FocusDown()
	return
}

// SetFocus sets the passed tile in focus. It returns the previous tile in focus.
// The behaviour is undefined if the given tile is not part of this WindowManager.
func (wm *WindowManager) SetFocus(tile TileNode) (
	prev TileNode,
) {
	if tile.wm != wm {
		panic(fmt.Sprintf("Tile does not belong to"+
			"this window manager: %p vs %p", wm, tile.wm))
	}
	if wm.border {
		wm.focus.node.Content().(*Frame).SetAttr(wm.borderAttr)
		tile.node.Content().(*Frame).SetAttr(wm.focusAttr)
	}
	prev = wm.focus
	wm.focus = tile
	return
}

// SetContent sets the content of the tile in focus to h.
func (wm *WindowManager) SetContent(tile TileNode, h tui.Handler) (
	prev tui.Handler,
) {
	if tile.wm != wm {
		panic(fmt.Sprintf("Tile does not belong to"+
			"this window manager: %p vs %p", wm, tile.wm))
	}
	prev = wm.Content(tile)
	if wm.border {
		h = wm.withFrame(h)
		h.(*Frame).SetAttr(wm.focusAttr)
	}
	tile.node.SetContent(h)
	return
}

// SetFocusContent sets the content of the tile in focus to h.
func (wm *WindowManager) SetFocusContent(h tui.Handler) (
	prev tui.Handler,
) {
	return wm.SetContent(wm.focus, h)
}

// Cursor returns the cursor coordinates of the tile in focus.
func (wm *WindowManager) Cursor() (term.Coordinates, bool) {
	offset := wm.tree.TilePosition(wm.focus.node)
	cursor, show := wm.focus.node.Content().(tui.Handler).Cursor()
	return term.Coordinates{X: offset.X + cursor.X, Y: offset.Y + cursor.Y}, show
}

// Man : Handler
func (wm *WindowManager) Man() tui.Manual {
	return tui.Manual{
		Summary: "WindowManager implements a tiled window manager.",
		Keys: tui.KeyMap{
			term.Event{Mod: term.ModAlt, Ch: 'q'}: {
				ID:          "Exit",
				Description: "Exit handler.",
			},
			term.Event{Mod: term.ModAlt, Ch: 'j'}: {
				ID:          "FocusDown",
				Description: "Switch focus to tile below tile in focus.",
			},
			term.Event{Mod: term.ModAlt, Ch: 'k'}: {
				ID:          "FocusUp",
				Description: "Switch focus to tile above tile in focus.",
			},
			term.Event{Mod: term.ModAlt, Ch: 'h'}: {
				ID:          "FocusLeft",
				Description: "Switch focus to tile on the left of tile in focus.",
			},
			term.Event{Mod: term.ModAlt, Ch: 'l'}: {
				ID:          "FocusRight",
				Description: "Switch focus to tile on the right of tile in focus.",
			},
		},
	}
}

// Draw : tui.Component
func (wm *WindowManager) Draw(w tui.Writer) {
	wm.tree.Draw(w)
}

// Resize : tui.Component
func (wm *WindowManager) Resize(width, height int) {
	wm.tree.Resize(width, height)
}

// Content returns the Component held by this TileNode in the TileTree.
func (t TileNode) Content() tui.Handler {
	return t.wm.Content(t)
}

// SetContent sets the content of a TileNode to c.
func (t TileNode) SetContent(h tui.Handler) tui.Handler {
	return t.wm.SetContent(t, h)
}

// Size returns the total number of nodes under this TileNode.
func (t TileNode) Size() (size int) {
	return t.node.Size()
}

// TileDown returns the tile in the bottom of t or nil if t is the
// bottom-most tile in the tree.
func (t TileNode) TileDown() (TileNode, bool) {
	node := t.node.TileDown()
	if node == nil {
		return TileNode{}, false
	}
	return t.wm.newNode(node), true
}

// TileLeft returns the tile left-adjacent to t or nil if t is the
// left-most tile in the tree.
func (t TileNode) TileLeft() (TileNode, bool) {
	node := t.node.TileLeft()
	if node == nil {
		return TileNode{}, false
	}
	return t.wm.newNode(node), true
}

// TileRight returns the tile right-adjacent to t or nil if t is the
// right-most tile in the tree.
func (t TileNode) TileRight() (TileNode, bool) {
	node := t.node.TileRight()
	if node == nil {
		return TileNode{}, false
	}
	return t.wm.newNode(node), true
}

// TileUp returns the tile on top of t or nil if t is the
// top-most tile in the tree.
func (t TileNode) TileUp() (TileNode, bool) {
	node := t.node.TileUp()
	if node == nil {
		return TileNode{}, false
	}
	return t.wm.newNode(node), true
}

// Close removes this node from the tree.
// It panics if node is last node on the tree.
func (t TileNode) Close() error {
	if t.wm == nil {
		return nil
	}

	ok := true
	if t.wm.focus.node == t.node {
		ok = t.wm.ShiftFocus()
	}
	if !ok {
		return errors.New("trying to close last node")
	}

	t.node.Close()

	return nil
}
