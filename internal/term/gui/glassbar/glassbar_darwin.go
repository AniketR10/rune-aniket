// Copyright (C) 2017-2026 The Rune Authors
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

package glassbar

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>

void runeGlassBarBegin(void);
void runeGlassBarAddButton(const char *id, const char *symbol,
                           const char *tooltip);
void runeGlassBarCommit(void);
void runeGlassBarSetFrame(double x, double y, double width, double padding);
*/
import "C"

import "unsafe"

const supported = true

// activate routes native button clicks back to the last Install caller.
// Rune has a single window, so a single callback is enough.
var activate func(string)

func install(buttons []Button, fn func(string)) {
	activate = fn
	C.runeGlassBarBegin()
	for _, button := range buttons {
		id := C.CString(button.ID)
		symbol := C.CString(button.Symbol)
		tooltip := C.CString(button.Tooltip)
		C.runeGlassBarAddButton(id, symbol, tooltip)
		C.free(unsafe.Pointer(id))
		C.free(unsafe.Pointer(symbol))
		C.free(unsafe.Pointer(tooltip))
	}
	C.runeGlassBarCommit()
}

func setFrame(x, y, width float64) {
	C.runeGlassBarSetFrame(C.double(x), C.double(y), C.double(width),
		C.double(Padding))
}

//export runeGlassBarActivate
func runeGlassBarActivate(id *C.char) {
	if activate == nil {
		return
	}
	activate(C.GoString(id))
}
