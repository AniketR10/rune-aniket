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

package appmenu

import (
	"sync"

	"github.com/unstablebuild/rune-go-sdk/term"
)

// Item is a single entry in an application menu. The set of
// implementations is closed: Command, Native, Separator and Submenu.
type Item interface {
	item()
}

// Command is a menu item that dispatches a Rune command.
type Command struct {
	Title   string
	Command string
	Args    []string
	// Key is the key combination the command is bound to. It is
	// displayed as the item's accelerator and, when the user presses
	// it, AppKit routes it here instead of to the window. The zero
	// value renders the item without an accelerator.
	Key term.KeyComb
	// Disabled renders the item greyed out and unclickable. It is used
	// for placeholder entries such as an empty "Open Recent" list.
	Disabled bool
	// Checked renders a checkmark next to the item, for commands that
	// toggle a setting the menu reflects.
	Checked bool
}

// Native is a menu item wired to a standard AppKit action such as
// "toggleFullScreen:". It is dispatched through the responder chain
// rather than through Rune's handler tree.
type Native struct {
	Title    string
	Selector string
	Key      term.KeyComb
}

// Separator is a horizontal rule between menu items.
type Separator struct{}

// Submenu is a menu item that expands into a nested list of items.
type Submenu struct {
	Title string
	Items []Item
}

func (Command) item()   {}
func (Native) item()    {}
func (Separator) item() {}
func (Submenu) item()   {}

// Menu is a top-level menu in the application menu bar.
type Menu struct {
	Title string
	Items []Item
}

// menuSpec is the flattened, platform-neutral description handed to the
// platform layer. Keeping the translation in Go leaves the Objective-C
// layer free of any logic.
type menuSpec struct {
	title string
	items []itemSpec
}

// itemSpec describes one menu entry. Exactly one of separator, tag or
// selector is meaningful: a separator entry, a Rune command identified
// by its callback tag, a native AppKit selector, or a submenu carrying
// nested children.
type itemSpec struct {
	title     string
	selector  string
	keyEquiv  string
	modifiers uint
	tag       int
	separator bool
	disabled  bool
	checked   bool
	children  []itemSpec
}

var (
	mu        sync.Mutex
	commands  = map[int]Command{}
	activated func(Command)
)

// Install replaces the application menu bar with menus. Selecting a
// Command item invokes activate with that item; Native items are
// dispatched by the platform. On platforms without a native menu bar
// Install does nothing.
//
// It must be called on the main thread.
func Install(menus []Menu, activate func(Command)) {
	spec, byTag := buildSpec(menus)

	mu.Lock()
	commands = byTag
	activated = activate
	mu.Unlock()

	installMenus(spec)
}

// activateTag runs the callback registered by the most recent Install
// for the command item carrying tag.
func activateTag(tag int) {
	mu.Lock()
	cmd, ok := commands[tag]
	activate := activated
	mu.Unlock()

	if ok && activate != nil {
		activate(cmd)
	}
}

func buildSpec(menus []Menu) ([]menuSpec, map[int]Command) {
	byTag := map[int]Command{}
	// Tag 0 is AppKit's default and must not address a command.
	nextTag := 1
	specs := make([]menuSpec, 0, len(menus))

	for _, menu := range menus {
		specs = append(specs, menuSpec{
			title: menu.Title,
			items: buildItems(menu.Items, byTag, &nextTag),
		})
	}

	return specs, byTag
}

// buildItems flattens items into itemSpecs, recursing into submenus.
// Command tags are drawn from the shared nextTag so every command in
// the tree — nested or not — gets a unique callback tag.
func buildItems(items []Item, byTag map[int]Command, nextTag *int) []itemSpec {
	specs := make([]itemSpec, 0, len(items))
	for _, item := range items {
		switch it := item.(type) {
		case Separator:
			specs = append(specs, itemSpec{separator: true})
		case Native:
			equiv, mods, _ := keyEquivalent(it.Key)
			specs = append(specs, itemSpec{
				title:     it.Title,
				selector:  it.Selector,
				keyEquiv:  equiv,
				modifiers: mods,
			})
		case Submenu:
			specs = append(specs, itemSpec{
				title:    it.Title,
				children: buildItems(it.Items, byTag, nextTag),
			})
		case Command:
			equiv, mods, _ := keyEquivalent(it.Key)
			byTag[*nextTag] = it
			specs = append(specs, itemSpec{
				title:     it.Title,
				keyEquiv:  equiv,
				modifiers: mods,
				tag:       *nextTag,
				disabled:  it.Disabled,
				checked:   it.Checked,
			})
			*nextTag++
		}
	}
	return specs
}
