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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/rune/internal/runenet"
)

type peerListerFunc func(ctx context.Context) ([]runenet.Peer, error)

func (f peerListerFunc) Peers(ctx context.Context) ([]runenet.Peer, error) {
	return f(ctx)
}

func TestPeerCompleter(t *testing.T) {
	peers := peerListerFunc(func(context.Context) ([]runenet.Peer, error) {
		return []runenet.Peer{
			{Hostname: "laptop", Online: true},
			{Hostname: "lab-box", Online: false},
			{Hostname: "workstation", Online: true},
		}, nil
	})

	tsuite := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "offers every peer once the scheme is typed",
			args: []string{"workspaceopen", "rune://"},
			want: []string{"rune://laptop/", "rune://lab-box/", "rune://workstation/"},
		},
		{
			name: "filters by the typed peer prefix",
			args: []string{"workspaceopen", "rune://la"},
			want: []string{"rune://laptop/", "rune://lab-box/"},
		},
		{
			name: "stays out of plain path completion",
			args: []string{"workspaceopen", "/home/ernie"},
			want: nil,
		},
		{
			name: "stops once the peer is chosen",
			args: []string{"workspaceopen", "rune://laptop/co"},
			want: nil,
		},
		{
			name: "no argument yet",
			args: []string{"workspaceopen"},
			want: nil,
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			it, newLastArg, err := PeerCompleter(peers).
				Complete(context.Background(), tcase.args)
			require.NoError(t, err)
			assert.Empty(t, newLastArg)

			var got []string
			for {
				v, ok := it.Next(context.Background())
				if !ok {
					break
				}
				got = append(got, v)
			}
			require.NoError(t, it.Err())
			assert.ElementsMatch(t, tcase.want, got)
		})
	}
}
