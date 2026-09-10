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

//go:build !windows

package main

import (
	"context"
	"testing"
)

// TestLoginShellPATHCmdDetachesFromTerminal is a regression for two terminal
// interactions: the Ctrl-C quitting breakage and the SIGTTOU stop that froze
// the GUI. The interactive login shell spawned to resolve PATH must not
// inherit Rune's terminal and must run in a new session (Setsid) with no
// controlling terminal, so its startup tcsetpgrp/tcsetattr calls cannot raise
// SIGTTOU (which would STOP it in state T and block the probe forever) and
// terminal-generated signals (the SIGINT raised by Ctrl-C) never reach it.
func TestLoginShellPATHCmdDetachesFromTerminal(t *testing.T) {
	cmd := loginShellPATHCmd(context.Background(), "/bin/sh")

	if cmd.Stdin != nil {
		t.Fatalf("login shell stdin must not be inherited from the terminal")
	}
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setsid {
		t.Fatalf("login shell must run in a new session (Setsid)")
	}
}
