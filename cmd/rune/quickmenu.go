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

package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"unstable.build/rune/ide"
	"unstable.build/rune/term/gui/glassbar"
	"unstable.build/rune/text"
)

const (
	// quickMenuColumnCells is the width of the grid column reserved for
	// the native quick menu, and quickMenuTopCell the row it starts at,
	// which is right below the tab bar.
	quickMenuColumnCells = 5
	quickMenuTopCell     = 3

	cmdQuickMenu = "quickmenu"
)

func quickMenuGlassButtons(buttons []ide.QuickMenuButton) []glassbar.Button {
	ret := make([]glassbar.Button, 0, len(buttons))
	for _, button := range buttons {
		ret = append(ret, glassbar.Button{
			ID: button.ID(), Symbol: button.Symbol, Tooltip: button.Title,
		})
	}
	return ret
}

// quickMenuAvailable reports whether the quick menu can be shown at all:
// the platform draws it and the user configured buttons for it.
func (b *bootstrapHandler) quickMenuAvailable() bool {
	return glassbar.Supported() && len(b.quickMenu) > 0
}

// quickMenuVisible reports whether the bar is currently on screen.
func (b *bootstrapHandler) quickMenuVisible() bool {
	return b.quickMenuAvailable() && !b.quickMenuOff
}

// quickMenuCells is the reserved column width, zero while the bar is
// hidden so the editor gets the space back.
func (b *bootstrapHandler) quickMenuCells() int {
	if !b.quickMenuVisible() {
		return 0
	}
	return quickMenuColumnCells
}

// quickMenuButtons are the buttons to install, empty while hidden so
// installing clears the bar.
func (b *bootstrapHandler) quickMenuButtons() []ide.QuickMenuButton {
	if !b.quickMenuVisible() {
		return nil
	}
	return b.quickMenu
}

// setQuickMenuVisible shows or hides the bar and gives the reserved
// column back to the editor, republishing the app menu so its Quick
// Menu checkmark follows. It must run on the event loop.
func (b *bootstrapHandler) setQuickMenuVisible(visible bool) {
	if !b.quickMenuAvailable() || visible == b.quickMenuVisible() {
		return
	}
	b.quickMenuOff = !visible
	if i := b.currentIDE(); i != nil {
		i.SetRightInset(b.quickMenuCells())
	}
	b.publishQuickMenuInstall()
	b.publishAppMenuInstall()
}

// subscribeQuickMenuCommand registers the toggle on i. The command only
// exists where a native quick menu is drawn, so a config that binds it
// elsewhere fails loudly rather than silently doing nothing.
func (b *bootstrapHandler) subscribeQuickMenuCommand(i *ide.IDE) error {
	if !glassbar.Supported() {
		return nil
	}
	man := textapi.CommandManual{
		Name: cmdQuickMenu,
		Summary: "Shows or hides the quick menu, the vertical bar of native " +
			"buttons along the right edge of the window. Hiding it gives the " +
			"column it reserves back to the editor.",
		Synopsis: "[on|off]",
	}
	handler := func(_ context.Context, cmd textapi.Command) error {
		visible := !b.quickMenuVisible()
		if len(cmd.Args) > 0 {
			switch cmd.Args[0] {
			case "on":
				visible = true
			case "off":
				visible = false
			default:
				return fmt.Errorf("expected 'on' or 'off' but found %q", cmd.Args[0])
			}
		}
		if !b.quickMenuAvailable() {
			return errors.New("no quick menu buttons are configured; " +
				"set gui.quick_menu in your configuration")
		}
		b.setQuickMenuVisible(visible)
		return nil
	}
	return i.SubscribeCommand(man, text.FuncCommandHandler(handler, nil))
}
