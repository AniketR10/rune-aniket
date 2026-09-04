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

package browser

import (
	"errors"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/handler"
)

type browserWindow struct {
	parent *Component
	win    handler.Window
}

func (w *browserWindow) Focus() (bool, error) {
	return w.win.Focus(), nil
}

func (w *browserWindow) WindowID() uint64 {
	return w.win.ID()
}

func (w *browserWindow) Closed() bool {
	return w.parent == nil || w.win.Closed()
}

// Position returns the top-left coordinate of this window within
// its WindowManager.
func (w *browserWindow) Position() term.Coordinates {
	return w.win.Position()
}

// Width returns the current rendered width of this window.
func (w *browserWindow) Width() int {
	return w.win.Width()
}

// Height returns the current rendered height of this window.
func (w *browserWindow) Height() int {
	return w.win.Height()
}

func (w *browserWindow) IsFloating() bool {
	return w.win.IsFloating()
}

func (w *browserWindow) Content() (browserapi.Handler, error) {
	h := w.win.Content().(browserapi.Handler)
	t, ok := h.(*Tab)
	if !ok {
		bc, ok := h.(*browserContent)
		if ok {
			return bc.Handler, nil
		}
		fsc, ok := h.(*browserFloatingScrollableContent)
		if ok {
			return fsc.Handler, nil
		}
		fc, ok := h.(*browserFloatingContent)
		if ok {
			return fc.Handler, nil
		}
		return h.(*browserScrollableContent).Handler, nil
	}
	return t, nil
}

func (w *browserWindow) SetContent(h browserapi.Handler) error {
	if w.parent == nil {
		return errors.New("window is closing")
	}
	return w.parent.tryUpdateWindowContent(w, h, w.win.Content().(browserapi.Handler))
}

func (w *browserWindow) IsMinimized() (component.Alignment, bool) {
	return w.win.IsMinimized()
}

func (w *browserWindow) MinimizeUp(padding int) bool {
	return w.win.MinimizeUp(padding)
}

func (w *browserWindow) MinimizeDown(padding int) bool {
	return w.win.MinimizeDown(padding)
}

func (w *browserWindow) MinimizeLeft(padding int) bool {
	return w.win.MinimizeLeft(padding)
}

func (w *browserWindow) MinimizeRight(padding int) bool {
	return w.win.MinimizeRight(padding)
}

func (w *browserWindow) Unminimize() bool {
	return w.win.Unminimize()
}

func (w *browserWindow) SetFrameAttr(attr term.Attributes) (term.Attributes, bool) {
	return w.win.SetFrameAttr(attr)
}

func (w *browserWindow) Close() error {
	if w.parent == nil {
		return nil
	}

	parent := w.parent

	err := parent.closeWindow(w)
	if err != nil {
		return err
	}

	w.parent = nil

	return nil
}
