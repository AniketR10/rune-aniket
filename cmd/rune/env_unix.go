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

//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// detachFromTerminal runs the command in a new session with no controlling
// terminal. An interactive login shell touches the controlling terminal on
// startup (zsh ZLE, job control: tcsetpgrp/tcsetattr). If it ran merely in a
// background process group of Rune's session, those calls would raise
// SIGTTOU/SIGTTIN whose default disposition is to STOP the process, so the
// shell would wedge in state T and never exit, blocking the probe forever.
// Setsid makes the child a session leader with no controlling terminal, so
// those calls become no-ops (ENOTTY) and terminal-generated signals (the
// SIGINT raised by Ctrl-C, SIGTTOU, SIGTTIN) can never reach it. This is
// strictly stronger than Setpgid for keeping Ctrl-C quitting `rune` working.
func detachFromTerminal(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
}
