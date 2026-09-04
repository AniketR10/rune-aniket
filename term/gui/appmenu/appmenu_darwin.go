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

//go:build darwin && cgo

package appmenu

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>

void runeAppMenuBegin(void);
void runeAppMenuAddMenu(const char *title);
void runeAppMenuBeginSubmenu(const char *title);
void runeAppMenuEndSubmenu(void);
void runeAppMenuAddSeparator(void);
void runeAppMenuAddItem(const char *title, const char *selector,
                        const char *keyEquiv, unsigned long modifiers, int tag,
                        int disabled, int checked);
void runeAppMenuCommit(void);
*/
import "C"

import "unsafe"

func installMenus(menus []menuSpec) {
	C.runeAppMenuBegin()
	for _, menu := range menus {
		title := C.CString(menu.title)
		C.runeAppMenuAddMenu(title)
		C.free(unsafe.Pointer(title))

		addItems(menu.items)
	}
	C.runeAppMenuCommit()
}

// addItems appends items to the current menu, descending into any
// submenu children so the whole tree is materialized.
func addItems(items []itemSpec) {
	for _, item := range items {
		switch {
		case item.separator:
			C.runeAppMenuAddSeparator()
		case item.children != nil:
			title := C.CString(item.title)
			C.runeAppMenuBeginSubmenu(title)
			C.free(unsafe.Pointer(title))
			addItems(item.children)
			C.runeAppMenuEndSubmenu()
		default:
			addItem(item)
		}
	}
}

func addItem(item itemSpec) {
	title := C.CString(item.title)
	defer C.free(unsafe.Pointer(title))
	selector := C.CString(item.selector)
	defer C.free(unsafe.Pointer(selector))
	keyEquiv := C.CString(item.keyEquiv)
	defer C.free(unsafe.Pointer(keyEquiv))

	C.runeAppMenuAddItem(title, selector, keyEquiv, C.ulong(item.modifiers),
		C.int(item.tag), cbool(item.disabled), cbool(item.checked))
}

func cbool(b bool) C.int {
	if b {
		return 1
	}
	return 0
}

//export runeAppMenuActivate
func runeAppMenuActivate(tag C.int) {
	activateTag(int(tag))
}
