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
