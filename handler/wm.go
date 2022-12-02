package handler

import (
	"fmt"

	"unstable.build/go-tui"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
)

// WindowManagerConfig represents a WindowManager's
// configuration properties.
type WindowManagerConfig struct {
	component.WindowManagerConfig

	Dim               bool
	FocusFrameAttr    term.Attributes
	FocusFrameCharSet component.FrameCharSet
}

// WindowManager implements Handler as a tiled window manager.
type WindowManager struct {
	comp   component.WindowManager
	config WindowManagerConfig
	focus  Window
	subs   []WindowSubscriber
}

// WindowSubscriber wraps the OnFocus callback used
// to subscribe to window focus.
type WindowSubscriber interface {
	OnFocus(prevFocus, newFocus Window)
}

// NewWindowManager allocates storage for a new WindowManager and initializes it with the
// given handler. If border is true, it will draw a border around every window.
func NewWindowManager(
	handler tui.Handler, cfg WindowManagerConfig,
) (wm *WindowManager) {
	wm = new(WindowManager)
	wm.Init(handler, cfg)
	return
}

func (wm *WindowManager) newNode(t component.Window) Window {
	return Window{wm: wm, Window: t}
}

// Init initializes this WindowManager with the given handler. If border is true, it will draw
// a border around every tile.
func (wm *WindowManager) Init(handler tui.Handler, cfg WindowManagerConfig) {
	win := wm.comp.Init(handler, cfg.WindowManagerConfig)
	wm.config = cfg
	wm.focus = wm.newNode(win)
	wm.SetFocus(wm.focus)
	return
}

// SetFrameCharSet sets the frame border cells used to draw borders around tiles.
// Note that this has no effect if WindowManager was
// initialized with border == false.
func (wm *WindowManager) SetFrameCharSet(def, focus component.FrameCharSet) {
	if !wm.config.Frame {
		return
	}
	wm.config.FocusFrameCharSet = focus
	wm.config.FrameCharSet = def
	wm.comp.SetFrameCharSet(def)
	wm.setFocusAttr(wm.focus)
}

func (wm *WindowManager) setFocusAttr(win Window) {
	// only set focus attr+charset if there's more than one window
	if wm.Size() > 1 {
		win.SetFrameAttr(wm.config.FocusFrameAttr)
		win.SetFrameCharSet(wm.config.FocusFrameCharSet)
	} else {
		win.SetFrameAttr(wm.config.FrameAttr)
		win.SetFrameCharSet(wm.config.FrameCharSet)
	}
}

// SetAttr sets the default and focus window border attributes. Note that
// this has no effect if WindowManager was initialized with border == false.
func (wm *WindowManager) SetAttr(def, focus term.Attributes) {
	if !wm.config.Frame {
		return
	}
	wm.config.FocusFrameAttr = focus
	wm.config.FrameAttr = def
	wm.comp.SetFrameAttr(def)
	wm.setFocusAttr(wm.focus)
}

// Handle : Handler
func (wm *WindowManager) Handle(ev term.Event) (exit bool, handled bool) {
	if ev.Type == term.EventMouse {
		mousePos := term.Coordinates{X: ev.MouseX, Y: ev.MouseY}
		childAtMouse, ok := wm.comp.WindowAt(mousePos)
		if !ok {
			return
		}
		if wm.Focus().Window != childAtMouse {
			if ev.Key == term.MouseLeft {
				wm.SetFocus(wm.newNode(childAtMouse))
			}
			return
		}
		offset := childAtMouse.Position()
		if wm.config.Frame {
			offset.Y++
			offset.X++
		}
		ev.MouseX -= offset.X
		ev.MouseY -= offset.Y

		// mouse on frame
		if ev.MouseX < 0 {
			ev.MouseX = 0
		}
		if ev.MouseY < 0 {
			ev.MouseY = 0
		}
	}

	var hexit bool
	focus := wm.focus
	size := wm.comp.Size()
	hexit, handled = focus.Content().Handle(ev)

	// if handler in focus wants to exit, close the window,
	// or signal exit to upstream handler if it was last window
	if hexit {
		if exit = size == 1; exit {
			return
		}
		curr := wm.focus
		if curr.ID() == focus.ID() {
			wm.ShiftFocus()
			curr.Close()
		}
	}

	return
}

// SplitVertical creates a new vertical split over the tile currently in focus.
func (wm *WindowManager) SplitVertical(h tui.Handler) (Window, bool) {
	w, ok := wm.comp.SplitVertical(wm.focus.Window, h)
	if !ok {
		return Window{}, false
	}
	ret := wm.newNode(w)
	wm.setFocusAttr(wm.focus)
	return ret, true
}

// SplitHorizontal creates a new horizontal split over the tile currently in focus.
func (wm *WindowManager) SplitHorizontal(h tui.Handler) (Window, bool) {
	w, ok := wm.comp.SplitHorizontal(wm.focus.Window, h)
	if !ok {
		return Window{}, false
	}
	ret := wm.newNode(w)
	wm.setFocusAttr(wm.focus)
	return ret, true
}

// FloatingWindow creates a floating window.
func (wm *WindowManager) FloatingWindow(
	content Floating, cfg component.FloatingConfig,
) Window {
	ret := wm.newNode(wm.comp.FloatingWindow(content, cfg))
	wm.setFocusAttr(wm.focus)
	return ret
}

func (wm *WindowManager) switchFocus(tileFn func(Window) (Window, bool)) bool {
	tile, ok := tileFn(wm.focus)
	if !ok {
		return false
	}
	wm.SetFocus(wm.newNode(tile.Window))
	return true
}

// FocusLeft switches the focus to the tile on the left side of the tile in focus
// If the tile in focus is the left-most tile in this window manager, then this method does nothing.
func (wm *WindowManager) FocusLeft() bool {
	return wm.switchFocus((Window).TileLeft)
}

// FocusRight switches the focus to the tile on the right side of the tile in focus
// If the tile in focus is the right-most tile in this window manager, then this method does nothing.
func (wm *WindowManager) FocusRight() bool {
	return wm.switchFocus((Window).TileRight)
}

// FocusUp switches the focus to the tile above the tile in focus
// If the tile in focus is the up-most tile in this window manager, then this method does nothing.
func (wm *WindowManager) FocusUp() bool {
	return wm.switchFocus((Window).TileUp)
}

// FocusDown switches the focus to the tile beneath the tile in focus
// If the tile in focus is the down-most tile in this window manager, then this method does nothing.
func (wm *WindowManager) FocusDown() bool {
	return wm.switchFocus((Window).TileDown)
}

// Focus returns the tile currently in focus.
func (wm *WindowManager) Focus() Window {
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

// Shiftable returns whether next call to ShiftFocus would return true.
func (wm *WindowManager) Shiftable() (w Window, ok bool) {
	focus := wm.Focus()
	w, ok = focus.TileLeft()
	if ok {
		return
	}
	w, ok = focus.TileUp()
	if ok {
		return
	}
	w, ok = focus.TileRight()
	if ok {
		return
	}
	w, ok = focus.TileDown()
	return
}

// Cursor returns the cursor coordinates of the tile in focus.
func (wm *WindowManager) Cursor() (term.Coordinates, bool) {
	offset := wm.focus.Window.Position()
	content := wm.focus.Content()
	if wm.config.Frame {
		offset.Y++
		offset.X++
	}
	cursor, show := content.Cursor()
	return term.Coordinates{X: offset.X + cursor.X, Y: offset.Y + cursor.Y}, show
}

// Man : Handler
func (wm *WindowManager) Man() tui.Manual {
	return tui.Manual{
		Summary: "WindowManager implements a tiled window manager.",
		Keys: tui.KeyMap{
			term.KeyComb{Mod: term.ModAlt, Ch: 'q'}: {
				ID:          "Exit",
				Description: "Exit handler.",
			},
			term.KeyComb{Mod: term.ModAlt, Ch: 'j'}: {
				ID:          "FocusDown",
				Description: "Switch focus to tile below tile in focus.",
			},
			term.KeyComb{Mod: term.ModAlt, Ch: 'k'}: {
				ID:          "FocusUp",
				Description: "Switch focus to tile above tile in focus.",
			},
			term.KeyComb{Mod: term.ModAlt, Ch: 'h'}: {
				ID:          "FocusLeft",
				Description: "Switch focus to tile on the left of tile in focus.",
			},
			term.KeyComb{Mod: term.ModAlt, Ch: 'l'}: {
				ID:          "FocusRight",
				Description: "Switch focus to tile on the right of tile in focus.",
			},
		},
	}
}

// Draw : tui.Component
func (wm *WindowManager) Draw(w term.Writer) {
	if wm.comp.Size() == 1 || !wm.config.Dim {
		wm.comp.Draw(w)
		return
	}

	focusWin := wm.focus.Window
	offset := focusWin.Position()
	// Set C to Nop for now so resize does not resize focus window
	// on every call to Draw
	v := component.Virtual{C: component.Nop()}
	v.Resize(focusWin.Width(), focusWin.Height())
	v.Move(offset)

	// draw content with frame if configured with frames
	if f, ok := wm.focus.Frame(); ok {
		v.C = f
	} else {
		v.C = wm.focus.Content()
	}

	// draw all tiles, including focus tile, if focus is not a floating window
	dimWriter := term.DimWriter(w)
	wm.comp.TileTree().Draw(dimWriter)

	// then overwrite focus tile with regular writer
	v.Draw(w)

	// if there are any floating windows that are not in focus
	// make sure they're drawn over the focus window, otherwise
	// they become hidden and unaccessible
	for _, w := range wm.comp.FloatingWindows() {
		if w != focusWin {
			wm.comp.DrawFloatingWindow(w, dimWriter)
		}
	}
}

// NOTE: this logic introduces a race-condition: If this WindowManager
// has any rpc handler's then there's a chance that this WindowManager will
// not be the first to acquire the lock after the draw request has completed
// and so it could break invariants accross the codebase (i.e. browser assuming
// all handlers are browser.Handler, which can cause panics or other issues.
// I haven't figured out a way to bypass this so drawing the current handler
// in focus twice seems like a better outcome that this nasty race-condition.
func (wm *WindowManager) _DrawOptimized(w term.Writer) {
	if wm.comp.Size() == 1 {
		wm.comp.Draw(w)
		return
	}

	// optimization to not draw focus window content twice
	// capture the state of the world here before we call Draw
	// to avoid race conditions, since Draw might call a remote
	// handler and unlock the event loop mutex.
	focus := wm.focus
	focusWin := focus.Window
	offset := focusWin.Position()
	if wm.config.Frame {
		offset.X++
		offset.Y++
	}
	// Set C to Nop for now so resize does not resize focus window
	// on every call to Draw
	v := component.Virtual{C: component.Nop()}
	v.Resize(focusWin.Width(), focusWin.Height())
	v.Move(offset)

	focusContent := focus.setContentResize(Nop(component.Nop()), false)
	v.C = focusContent

	// draw all with dimming term.Writer
	wm.comp.Draw(term.DimWriter(w))

	// reset focus window content
	focus.setContentResize(focusContent, false)

	// finally draw focus window content with standard term.Writer
	// over nop component
	v.Draw(w)
}

// Resize : tui.Component
func (wm *WindowManager) Resize(width, height int) {
	wm.comp.Resize(width, height)
}

// SetFocus sets the passed tile in focus. It returns the previous tile in focus.
// The behaviour is undefined if the given tile is not part of this WindowManager.
func (wm *WindowManager) SetFocus(tile Window) (
	prev Window,
) {
	if tile.wm != wm {
		panic(fmt.Sprintf("Tile does not belong to"+
			"this window manager: %p vs %p", wm, tile.wm))
	}
	if wm.config.Frame {
		wm.focus.Window.SetFrameAttr(wm.config.FrameAttr)
		wm.focus.Window.SetFrameCharSet(wm.config.FrameCharSet)
		wm.setFocusAttr(tile)
	}
	prev = wm.focus
	wm.focus = tile
	if prev != tile {
		wm.dispatchOnFocus(prev, wm.focus)
	}
	return
}

// DefaultWindowManagerConfig returns a sane WindowManagerConfig ready to use.
func DefaultWindowManagerConfig() WindowManagerConfig {
	return WindowManagerConfig{
		Dim:                 true,
		WindowManagerConfig: component.DefaultWindowManagerConfig(),
		FocusFrameCharSet:   component.FrameCharSetDefault(),
		FocusFrameAttr: term.Attributes{
			Fg: term.ColorRed,
			Bg: term.ColorDefault,
		},
	}
}

// Size returns the size in windows of this WindowManager.
func (wm *WindowManager) Size() int {
	return wm.comp.Size()
}

// Iterate applies op to the content of all widnows of this WindowManager.
func (wm *WindowManager) Iterate(fn func(Window)) {
	wm.comp.Iterate(func(c component.Window) {
		fn(wm.newNode(c))
	})
}

func (wm *WindowManager) dispatchOnFocus(prev, focus Window) {
	for _, sub := range wm.subs {
		sub.OnFocus(prev, focus)
	}
}

// Subscribe subscribes sub to window focus events.
func (wm *WindowManager) Subscribe(sub WindowSubscriber) {
	wm.subs = append(wm.subs, sub)
}

// UnsubscribeAll unsubscribes all WindowSubscriber.
func (wm *WindowManager) UnsubscribeAll() {
	wm.subs = nil
}
