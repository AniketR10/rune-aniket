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

//go:build linux

package peerprocess

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// Identify returns the process on the other end of conn.
func Identify(conn *net.UnixConn) (Process, error) {
	raw, err := conn.SyscallConn()
	if err != nil {
		return Process{}, err
	}
	var cred *unix.Ucred
	var sockErr error
	err = raw.Control(func(fd uintptr) {
		cred, sockErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	})
	if err != nil {
		return Process{}, err
	}
	if sockErr != nil {
		return Process{}, sockErr
	}
	ret := Process{
		PID: int(cred.Pid),
		UID: cred.Uid,
		GID: cred.Gid,
	}
	ret.Exe, _ = exe(ret.PID)
	ret.Argv, _ = argv(ret.PID)
	return ret, nil
}

func exe(pid int) (string, error) {
	exe, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(exe, " (deleted)"), nil
}

func argv(pid int) ([]string, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return nil, err
	}
	data = bytes.TrimRight(data, "\x00")
	if len(data) == 0 {
		return nil, nil
	}
	parts := bytes.Split(data, []byte{0})
	argv := make([]string, 0, len(parts))
	for _, part := range parts {
		argv = append(argv, string(part))
	}
	return argv, nil
}
