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

package extension

import (
	"context"
	"fmt"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
)

// NewExtension returns the color palette extension and its metadata.
func NewExtension() (extensionapi.WorkspaceExtension, extensionapi.Metadata) {
	return workspaceExtension{}, extensionapi.Metadata{
		DeveloperID:      "Unstable Build",
		DeveloperEmail:   "it@unstable.build",
		DeveloperKey:     "064D4ABCFA6D9338",
		ExtensionID:      "color_palette",
		ExtensionName:    "Color Palette",
		ExtensionVersion: "development",
		Permissions: extensionapi.NewPermissions(
			extensionapi.PermissionCommands,
			extensionapi.PermissionEditor,
			extensionapi.PermissionBrowserWindowManager,
		),
	}
}

type workspaceExtension struct{}

func (workspaceExtension) ExtendWorkspace(
	ctx context.Context, w *extensionapi.Workspace, cfg config.Config,
) error {
	return w.RegisterCommand(colorPaletteCmd, &colorPaletteCommandHandler{
		wm: w.WindowManager(ctx),
	})
}

type colorPaletteCommandHandler struct {
	mu     sync.Mutex
	wm     browserapi.WindowManager
	window browserapi.Window
}

func (h *colorPaletteCommandHandler) HandleCommand(
	ctx context.Context, cmd textapi.Command,
) error {
	h.mu.Lock()
	if h.window != nil {
		h.mu.Unlock()
		return nil
	}
	h.mu.Unlock()

	palette := new(colorPaletteHandler)
	cleaningHandler := browserapi.FuncHandler(palette, func() error {
		h.mu.Lock()
		h.window = nil
		h.mu.Unlock()
		return palette.Close()
	})
	window, err := h.wm.Split(browserapi.OrientationRight, cmd.Window, cleaningHandler)
	if err != nil {
		return err
	}

	h.mu.Lock()
	h.window = window
	h.mu.Unlock()
	return err
}

func (*colorPaletteCommandHandler) Complete(
	ctx context.Context, cmd string, args []string,
) (iterator.Iterator[string], error) {
	return iterator.FromSlice[string](nil), nil
}

var colorPaletteCmd = textapi.CommandManual{
	Name: "colorpalette",
	Summary: "Opens a new window and displays all the color codes available " +
		"to customize the UI via configuration.",
}

type colorPaletteHandler struct {
	width, height int
	grid          tui.Component
	dim           bool
	dirty         bool
}

func (h *colorPaletteHandler) Resize(width, height int) {
	if h.grid != nil {
		h.grid.Resize(width, height)
	}
	h.width = width
	h.height = height
}

func makeColorGrid(dim bool) tui.Component {
	ret := make([][]tui.Component, 16)
	var nameNum int
	for y := range 16 {
		ret[y] = make([]tui.Component, 16)
		for x := range 16 {
			var attrs term.AttrMask
			color := term.PaletteColor(nameNum)
			name := color.Name(true)
			if dim {
				attrs = term.AttrDim
				name = fmt.Sprintf("D%s", name)
			}
			ret[y][x] = component.NewStringWithConfig(name,
				component.StringConfig{
					Attributes:           term.Attributes{Bg: color, Attrs: attrs},
					BackgroundAttributes: term.Attributes{Bg: color, Attrs: attrs},
				},
			)
			nameNum++
		}
	}
	nextGridOf := 12
	nextGrid := make([][]tui.Component, 0, nextGridOf)
	var i, x int
	y := -1
	for name := range term.GetColorNames() {
		if i%nextGridOf == 0 {
			y++
			x = 0
			nextGrid = append(nextGrid, make([]tui.Component, nextGridOf))
		}
		var attrs term.AttrMask
		color := term.GetColor(name)
		if dim {
			attrs = term.AttrDim
			name = fmt.Sprintf("D%s", name)
		}
		nextGrid[y][x] = component.NewStringWithConfig(name,
			component.StringConfig{
				Attributes:           term.Attributes{Bg: color, Attrs: attrs},
				BackgroundAttributes: term.Attributes{Bg: color, Attrs: attrs},
			},
		)
		i++
		x++
	}
	ret = append(ret, nextGrid...)
	return component.Grid(ret)
}

func (h *colorPaletteHandler) Draw(w term.Writer) {
	if h.grid == nil || h.dirty {
		h.dirty = false
		h.grid = makeColorGrid(h.dim)
		h.grid.Resize(h.width, h.height)
	}
	h.grid.Draw(w)
}

func (h *colorPaletteHandler) Handle(ev term.Event) (exit, handled bool) {
	if ev.Type != term.EventKey {
		return
	}
	if ev.Ch == 'd' && ev.Mod == term.ModCtrl {
		h.dim = !h.dim
		h.dirty = true
		return
	}
	exit = ev.Key == term.KeyEsc
	return
}

func (h *colorPaletteHandler) Cursor() (pos term.Coordinates, style term.CursorStyle, show bool) {
	return
}

func (h *colorPaletteHandler) Selection() (string, bool) {
	return "", false
}

func (h *colorPaletteHandler) Close() error {
	return nil
}
