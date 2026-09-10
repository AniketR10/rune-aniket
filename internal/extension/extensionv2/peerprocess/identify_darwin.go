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

//go:build darwin

package peerprocess

import (
	"net"

	"golang.org/x/sys/unix"
)

// Identify returns the process on the other end of conn.
func Identify(conn *net.UnixConn) (Process, error) {
	raw, err := conn.SyscallConn()
	if err != nil {
		return Process{}, err
	}
	var pid int
	var uid uint32
	var gid uint32
	var sockErr error
	err = raw.Control(func(fd uintptr) {
		pid, sockErr = unix.GetsockoptInt(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERPID)
		if sockErr != nil {
			return
		}
		cred, err := unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
		if err == nil {
			uid = cred.Uid
			if cred.Ngroups > 0 {
				gid = cred.Groups[0]
			}
		}
	})
	if err != nil {
		return Process{}, err
	}
	if sockErr != nil {
		return Process{}, sockErr
	}
	ret := Process{
		PID: pid,
		UID: uid,
		GID: gid,
	}
	ret.Exe, _ = darwinExe(pid)
	ret.Argv, _ = darwinArgv(pid)
	return ret, nil
}
