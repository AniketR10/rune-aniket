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

package openpanel

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>

void runeOpenPanelShow(int tag, const char *title, const char *message,
                       const char *prompt, int directories, int multiple);
*/
import "C"

import "unsafe"

func showPanel(tag int, opts Options) {
	title := C.CString(opts.Title)
	defer C.free(unsafe.Pointer(title))
	message := C.CString(opts.Message)
	defer C.free(unsafe.Pointer(message))
	prompt := C.CString(opts.Prompt)
	defer C.free(unsafe.Pointer(prompt))

	C.runeOpenPanelShow(C.int(tag), title, message, prompt,
		cbool(opts.Directories), cbool(opts.Multiple))
}

func cbool(b bool) C.int {
	if b {
		return 1
	}
	return 0
}

//export runeOpenPanelDone
func runeOpenPanelDone(tag C.int, paths *C.char, length C.int) {
	var joined string
	if paths != nil && length > 0 {
		joined = C.GoStringN(paths, length)
	}
	finish(int(tag), splitPaths(joined))
}
