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
