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
