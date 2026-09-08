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

package workspacerune

import (
	"context"
	"strings"

	"github.com/unstablebuild/blue/iterator"
	"unstable.build/rune/internal/handler/command"
	"unstable.build/rune/internal/runenet"
)

// prefix is what a user must type before peer names are offered.
const prefix = Scheme + "://"

// PeerLister enumerates the machines in the mesh. It is satisfied by
// [runenet.Node].
type PeerLister interface {
	Peers(ctx context.Context) ([]runenet.Peer, error)
}

// PeerCompleter completes rune:// workspace URIs with the machines
// currently in the mesh, so opening a workspace on another machine is
// a pick rather than a hostname the user has to remember. Offline
// peers are offered too: connecting is what wakes them up in the
// user's mind, and the reconnect loop handles a peer that is not up
// yet.
func PeerCompleter(peers PeerLister) command.Completer {
	return command.FuncCompleter(func(
		ctx context.Context, args []string,
	) (iterator.Iterator[string], string, error) {
		var last string
		if len(args) > 1 {
			last = args[len(args)-1]
		}
		// Only contribute once the user has committed to the scheme;
		// otherwise every workspaceopen completion would be padded
		// with peers the user is not asking for.
		if !strings.HasPrefix(last, prefix) {
			return iterator.Empty[string](), "", nil
		}
		typed := strings.TrimPrefix(last, prefix)
		if strings.Contains(typed, "/") {
			// The peer is already chosen and the user is typing a path
			// on it, which only that peer can complete.
			return iterator.Empty[string](), "", nil
		}

		found, err := peers.Peers(ctx)
		if err != nil {
			return nil, "", err
		}
		ret := make([]string, 0, len(found))
		for _, p := range found {
			if !strings.HasPrefix(p.Hostname, typed) {
				continue
			}
			ret = append(ret, prefix+p.Hostname+"/")
		}
		return iterator.FromSlice(ret), "", nil
	})
}
