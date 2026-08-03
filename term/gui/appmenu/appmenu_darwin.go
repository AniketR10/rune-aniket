// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

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
                        int disabled);
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
		C.int(item.tag), cbool(item.disabled))
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
