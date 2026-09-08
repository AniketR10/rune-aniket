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

package peerprocess

// Process describes the process on the other end of a Unix-domain socket.
type Process struct {
	PID  int
	UID  uint32
	GID  uint32
	Exe  string
	Argv []string
}

// ProgramPath returns the best available executable path for the process.
func (p Process) ProgramPath() string {
	if p.Exe != "" {
		return p.Exe
	}
	if len(p.Argv) > 0 {
		return p.Argv[0]
	}
	return ""
}

// ProgramArgs returns the process argv after argv[0].
func (p Process) ProgramArgs() []string {
	if len(p.Argv) <= 1 {
		return nil
	}
	return append([]string(nil), p.Argv[1:]...)
}
