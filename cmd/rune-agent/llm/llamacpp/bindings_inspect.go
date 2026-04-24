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


package llamacpp

// Test-only inspection helpers for the C-side rune_chat_message array
// built by buildCRichChatMessages. Cgo can't be used from a _test.go
// file (see https://pkg.go.dev/cmd/cgo), so the helpers live in a
// regular source file that imports the cgo header but is only
// referenced from tests.

/*
#include "common_chat_wrap.h"
#include "llama.h"
#include <stdlib.h>
#include <string.h>
*/
import "C"

import "unsafe"
import "runtime/cgo"

// cMsgOptionalFieldsSnapshot returns the optional-pointer / count
// fields of the i-th message in an array returned by
// buildCRichChatMessages. Exposed so tests can pin down the contract
// that optional fields are zero-initialized — a crash was observed when
// the underlying allocation was not zeroed and these fields inherited
// garbage from freed heap memory.
func cMsgOptionalFieldsSnapshot(
	base unsafe.Pointer, i, count int,
) (contentPartsPtr uintptr, contentPartsLen uint64, toolCallsPtr uintptr, toolCallsLen uint64) {
	if base == nil || i < 0 || i >= count {
		return
	}
	slice := unsafe.Slice((*C.struct_rune_chat_message)(base), count)
	cm := slice[i]
	return uintptr(unsafe.Pointer(cm.content_parts)),
		uint64(cm.n_content_parts),
		uintptr(unsafe.Pointer(cm.tool_calls)),
		uint64(cm.n_tool_calls)
}

// cLegacyMsgPointersSnapshot returns the role/content C-string pointers
// of the i-th entry in an array returned by buildCLegacyChatMessages.
// Tests use it to verify both pointers are non-NULL even for empty
// strings (CString must allocate the trailing NUL).
func cLegacyMsgPointersSnapshot(
	base *C.struct_llama_chat_message, i, count int,
) (rolePtr, contentPtr uintptr) {
	if base == nil || i < 0 || i >= count {
		return
	}
	slice := unsafe.Slice(base, count)
	cm := slice[i]
	return uintptr(unsafe.Pointer(cm.role)), uintptr(unsafe.Pointer(cm.content))
}

// callLlamacppLog invokes the package's exported log callback with a
// C-allocated copy of msg. Defined here (not in bindings_test.go)
// because test files cannot call cgo-typed exported functions
// directly.
func callLlamacppLog(level int32, msg string) {
	cstr := C.CString(msg)
	defer C.free(unsafe.Pointer(cstr))
	llamacppLogCallback(C.int(level), cstr)
}

// callLlamacppProgress invokes the exported progress callback.
// Returns the int the callback chose to return so tests can pin the
// "0 user_data → return 1" branch and the cgo.Handle dispatch.
func callLlamacppProgress(progress float32, userData uintptr) int {
	return int(llamacppProgressCallback(C.float(progress), C.uintptr_t(userData)))
}

// newProgressHandle wraps cgo.NewHandle so tests can drive the
// exported progress callback without importing runtime/cgo (which
// would otherwise pull cgo into the _test.go file).
func newProgressHandle(cb func(progress, total int64, units string)) cgo.Handle {
	return cgo.NewHandle(cb)
}
