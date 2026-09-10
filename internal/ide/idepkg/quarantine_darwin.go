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

package idepkg

import (
	"errors"

	"golang.org/x/sys/unix"
)

// clearQuarantine removes the com.apple.quarantine xattr so Gatekeeper
// does not block a freshly installed, unsigned extension binary the
// first time it runs. A missing attribute is not an error.
func clearQuarantine(path string) error {
	err := unix.Removexattr(path, "com.apple.quarantine")
	if err == nil || errors.Is(err, unix.ENOATTR) {
		return nil
	}
	return err
}
