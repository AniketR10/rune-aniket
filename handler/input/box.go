// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package input

import (
	"context"
	"fmt"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/rune/cell"
	tcomponent "unstable.build/rune/component"
	"unstable.build/rune/text"
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
	ContentConfig     term.Attributes
}

// Box is an input box that essentially collects user input.
type Box struct {
	buf         *cell.Buffer
	cfg         BoxConfig
	frame       *handler.Frame
	placeholder *handler.Frame
	// used to assist in calculate height
	placeholderStr component.Responsive
	editor         text.Editor
	handler        text.Handler
	// used just for Height calculation support
	scroll tcomponent.Scroll
}

// NewBox allocates storage for a new Box and
// initializes it.
func NewBox(buf *cell.Buffer, ed text.Editor, cfg BoxConfig) *Box {
	ret := new(Box)
	ret.Init(buf, ed, cfg)
	return ret
}

// Init initializes this Box with the given config and cell.Buffer,
// which is used to share the contents of the input box with clients.
func (i *Box) Init(buf *cell.Buffer, ed text.Editor, cfg BoxConfig) {
	if cfg.MaxHeight < cfg.MinHeight && cfg.MaxHeight != 0 {
		panic("max height cannot be smaller than min height")
	}

	uri := workspaceapi.RandomURI("memory")
	edh, err := ed.Edit(context.Background(), uri, buf, false, false)
	if err != nil {
		// this should not really happen, as editor implementations
		// passed to an input box should all be internal, and so
		// no errors can occur.
		panic(fmt.Errorf("editor edit: %v", err))
	}

	// wrap must be set to true for input box
	// to work as expected by users.
	edh.SetWrap(true)
	// space is usually limited, so hide the text editor's
	// command bar
	edh.ShowCommandBar(false)

	edh.SetDefaultAttributes(cfg.ContentConfig)

	frame := handler.NewFrame(edh)
	frame.Attributes = cfg.DefaultFrameAttr

	// do not expose StringResponsiveConfig in BoxConfig because editor handlers are
	// not capable of emulating NoSplitWords property.
	placeholderStr := component.NewResponsiveString(cfg.Placeholder,
		component.StringResponsiveConfig{StringConfig: cfg.PlaceholderConfig})
	placeholder := handler.NewFrame(
		handler.NopFromComponent(placeholderStr))
	placeholder.Attributes = cfg.DefaultFrameAttr

	i.buf = buf
	i.cfg = cfg
	i.frame = frame
	i.placeholderStr = placeholderStr
	i.placeholder = placeholder
	i.handler = edh
	i.editor = ed
	i.scroll.Init(i.buf)
	i.scroll.Wrap = true
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

// CursorAtScroll returns the editor cursor in the input buffer.
func (r *Box) CursorAtScroll() term.Coordinates {
	return r.handler.CursorAtScroll()
}

// SetCursorAtScroll sets the editor cursor in the input buffer.
func (r *Box) SetCursorAtScroll(pos term.Coordinates) bool {
	return r.handler.SetCursorAtScroll(pos)
}

// SelectionBounds returns the editor selection in input-buffer coordinates.
func (r *Box) SelectionBounds() (from, to term.Coordinates, ok bool) {
	h, ok := r.handler.(interface {
		SelectionBounds() (term.Coordinates, term.Coordinates, bool)
	})
	if !ok {
		return term.Coordinates{}, term.Coordinates{}, false
	}
	return h.SelectionBounds()
}

// Reset resets this input box to its initial state.
func (r *Box) Reset() {
	_ = r.handler.SetCursorAtScroll(term.Coordinates{})
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
		// use scroll, rather than frame as Frame's content is the editor
		// which doesn't satisfy component.Responsive.
		ret = i.scroll.Height(width)
		rows := i.buf.Rows()
		// if last visible row is "full", always return +1
		// to allow for cursor to fall in an empty row but within bounds.
		if rows > 0 {
			lastRowCols := i.buf.View().Columns(rows - 1)
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
func (i *Box) Cursor() (pos term.Coordinates, style term.CursorStyle, show bool) {
	return i.frame.Cursor()
}

// Selection satisfies tui.Handler.
func (i *Box) Selection() (string, bool) {
	return i.frame.Selection()
}
