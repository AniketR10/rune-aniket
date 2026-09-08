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

package workspacessh

import (
	"errors"
	"strings"
)

// isRetryableConnectError returns true when err looks transient enough
// that re-running ssh.Dial may succeed (network hiccups, intermittent
// timeouts). Permanent failures (auth rejected, host key mismatch, the
// remote workspace path or rune binary missing) return false: retrying
// only re-prompts the user for credentials they have already shown they
// don't have.
func isRetryableConnectError(err error) bool {
	if err == nil {
		return false
	}
	// Typed errors surfaced by std_remote.translateDialError.
	if errors.Is(err, ErrAuthRequiredKey) ||
		errors.Is(err, ErrHostKeyMismatch) ||
		errors.Is(err, ErrHostKeyUnknown) ||
		errors.Is(err, ErrKnownHostsUnparsable) {
		return false
	}
	msg := err.Error()
	// Generic auth failure strings from the Go ssh client.
	if strings.Contains(msg, "unable to authenticate") ||
		strings.Contains(msg, "no supported methods remain") ||
		strings.Contains(msg, "ssh authentication") {
		return false
	}
	// connectScheme verifies the remote workspace path / rune binary
	// before returning. These are not transient.
	if strings.Contains(msg, "executable was not found on remote") ||
		strings.Contains(msg, "was not found on remote") {
		return false
	}
	return true
}
