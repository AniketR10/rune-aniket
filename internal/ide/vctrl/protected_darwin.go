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

package vctrl

import (
	"os/user"
	"path/filepath"
)

// protectedRoots returns the absolute macOS directories that must not be
// traversed without an explicit user grant. Reading ~/Library can enter
// other apps' data containers and trip the TCC App Data prompt. Personal
// folders remain visible so users can find their own files there.
func protectedRoots() []string {
	usr, err := user.Current()
	if err != nil || usr.HomeDir == "" {
		return nil
	}
	home := filepath.Clean(usr.HomeDir)
	return []string{
		filepath.Join(home, "Library"),
	}
}
