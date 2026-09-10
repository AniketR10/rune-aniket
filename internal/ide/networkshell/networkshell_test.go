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

package networkshell

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/rune/internal/runenet"
)

type fakeNetwork struct {
	status runenet.Status
	peers  []runenet.Peer
	up     int
	upErr  error
	down   int
}

func (f *fakeNetwork) Status(context.Context) (runenet.Status, error) {
	return f.status, nil
}

func (f *fakeNetwork) Peers(context.Context) ([]runenet.Peer, error) {
	return f.peers, nil
}

func (f *fakeNetwork) Up(context.Context) error   { f.up++; return f.upErr }
func (f *fakeNetwork) Down(context.Context) error { f.down++; return nil }

type nopProgressWriter struct{}

func (nopProgressWriter) Progress(int64, int64, string) {}

func handle(
	t *testing.T, h *Handler, args ...string,
) iterator.Iterator[component.Responsive] {
	t.Helper()
	it, err := h.HandleCommand(context.Background(),
		repl.Command{Name: CommandName, Args: args}, nopProgressWriter{})
	require.NoError(t, err)
	return it
}

func TestHandleCommandRouting(t *testing.T) {
	net := &fakeNetwork{
		status: runenet.Status{
			Hostname:  "laptop",
			State:     "Running",
			LoginName: "ernie@example.com",
			Addrs:     []netip.Addr{netip.MustParseAddr("100.64.0.1")},
		},
		peers: []runenet.Peer{{Hostname: "workstation", OS: "linux", Online: true}},
	}
	h := New(Config{Network: net})

	t.Run("status", func(t *testing.T) {
		require.NotNil(t, handle(t, h, "status"))
	})

	t.Run("peers", func(t *testing.T) {
		require.NotNil(t, handle(t, h, "peers"))
	})

	t.Run("up joins the network", func(t *testing.T) {
		require.NotNil(t, handle(t, h, "up"))
		assert.Equal(t, 1, net.up)
	})

	t.Run("up surfaces the join failure", func(t *testing.T) {
		failing := &fakeNetwork{upErr: errors.New("join network: bad auth key")}
		_, err := New(Config{Network: failing}).HandleCommand(
			context.Background(),
			repl.Command{Name: CommandName, Args: []string{"up"}},
			nopProgressWriter{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bad auth key")
	})

	t.Run("down leaves the network", func(t *testing.T) {
		require.NotNil(t, handle(t, h, "down"))
		assert.Equal(t, 1, net.down)
	})

	t.Run("no argument prints usage", func(t *testing.T) {
		require.NotNil(t, handle(t, h))
	})

	t.Run("unknown subcommand is an error", func(t *testing.T) {
		_, err := h.HandleCommand(context.Background(),
			repl.Command{Name: CommandName, Args: []string{"sideways"}},
			nopProgressWriter{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown command")
	})
}

func TestStatusMarkdown(t *testing.T) {
	t.Run("joined", func(t *testing.T) {
		got := statusMarkdown(runenet.Status{
			Hostname:  "laptop",
			State:     "Running",
			LoginName: "ernie@example.com",
			Addrs:     []netip.Addr{netip.MustParseAddr("100.64.0.1")},
		})
		assert.Contains(t, got, "`laptop`")
		assert.Contains(t, got, "Running")
		assert.Contains(t, got, "`100.64.0.1`")
		assert.NotContains(t, got, "**error**")
	})

	t.Run("failed to join", func(t *testing.T) {
		got := statusMarkdown(runenet.Status{
			Hostname:  "laptop",
			State:     "NotRunning",
			LastError: "join network: control server unreachable",
		})
		assert.Contains(t, got, "NotRunning")
		assert.Contains(t, got,
			"- **error**: join network: control server unreachable")
	})

	t.Run("pending sign-in", func(t *testing.T) {
		got := statusMarkdown(runenet.Status{
			Hostname: "laptop",
			State:    "NeedsLogin",
			AuthURL:  "https://control.example.com/a/1",
		})
		assert.Contains(t, got, "https://control.example.com/a/1")
	})
}

func TestComplete(t *testing.T) {
	h := New(Config{Network: &fakeNetwork{}})

	tsuite := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "every subcommand",
			args: nil,
			want: []string{"status", "peers", "up", "down"},
		},
		{
			name: "filtered by prefix",
			args: []string{"p"},
			want: []string{"peers"},
		},
		{
			name: "subcommands take no arguments",
			args: []string{"peers", ""},
			want: nil,
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			it, err := h.Complete(context.Background(), CommandName, tcase.args)
			require.NoError(t, err)
			var got []string
			for {
				v, ok := it.Next(context.Background())
				if !ok {
					break
				}
				got = append(got, v)
			}
			assert.ElementsMatch(t, tcase.want, got)
		})
	}
}
