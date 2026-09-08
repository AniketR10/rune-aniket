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

package exoeditor

import "github.com/unstablebuild/rune-go-sdk/api/workspaceapi"

// Reloader routes an FS-watcher-driven reload for the file at uri
// into the IDE's canonical async reload pipeline — the same path
// :reloadfile uses. Implementations live in the ide package and
// resolve uri to the open tab's workspace.FlusherCloser internally.
//
// Reload is non-blocking: it starts the async reload and returns
// either nil or a start-failure error (e.g. workspace.ErrFlushInProgress
// when another op is already in flight). The actual disk I/O,
// buffer reset, and dirty-tab attribute clear happen on the IDE's
// own awaiter goroutine + UI scheduler — callers do not wait for
// completion.
//
// Reload must be called on the host UI goroutine: it touches the
// open-tab map and the FlusherCloser swap-worker state, which
// cooperate with subscribers that mutate UI-owned cell.Buffer state.
type Reloader interface {
	Reload(uri workspaceapi.URI) error
}
