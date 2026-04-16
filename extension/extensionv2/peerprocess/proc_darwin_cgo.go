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

package peerprocess

/*
#include <libproc.h>
#include <stdlib.h>
#include <string.h>
#include <sys/sysctl.h>
*/
import "C"

import (
	"bytes"
	"fmt"
	"unsafe"
)

func darwinExe(pid int) (string, error) {
	const bufsize = C.PROC_PIDPATHINFO_MAXSIZE
	buffer := (*C.char)(C.malloc(C.size_t(bufsize)))
	defer C.free(unsafe.Pointer(buffer))
	ret, err := C.proc_pidpath(C.int(pid), unsafe.Pointer(buffer), C.uint32_t(bufsize))
	if ret <= 0 {
		return "", err
	}
	return C.GoString(buffer), nil
}

func darwinArgv(pid int) ([]string, error) {
	argmax, err := darwinArgMax()
	if err != nil {
		return nil, err
	}
	if argmax <= 0 {
		return nil, fmt.Errorf("invalid kern.argmax %d", argmax)
	}
	var mib = [...]C.int{C.CTL_KERN, C.KERN_PROCARGS2, C.int(pid)}
	size := C.size_t(argmax)
	procargs := (*C.char)(C.malloc(C.size_t(argmax)))
	defer C.free(unsafe.Pointer(procargs))
	if ret, err := C.sysctl(&mib[0], 3, unsafe.Pointer(procargs), &size, nil, 0); ret != 0 {
		return nil, err
	}
	var nargs C.int
	C.memcpy(unsafe.Pointer(&nargs), unsafe.Pointer(procargs), C.sizeof_int)
	data := C.GoBytes(unsafe.Pointer(procargs), C.int(size))
	return darwinParseProcArgs(data, int(nargs)), nil
}

func darwinArgMax() (int, error) {
	var mib = [...]C.int{C.CTL_KERN, C.KERN_ARGMAX}
	var argmax C.int
	size := C.size_t(unsafe.Sizeof(argmax))
	if ret, err := C.sysctl(&mib[0], 2, unsafe.Pointer(&argmax), &size, nil, 0); ret != 0 {
		return 0, err
	}
	return int(argmax), nil
}

func darwinParseProcArgs(data []byte, nargs int) []string {
	if len(data) <= C.sizeof_int || nargs <= 0 {
		return nil
	}
	fields := bytes.Split(data[C.sizeof_int:], []byte{0})
	argv := make([]string, 0, nargs)
	for _, field := range fields[1:] {
		if len(field) == 0 {
			continue
		}
		argv = append(argv, string(field))
		if len(argv) == nargs {
			break
		}
	}
	return argv
}
