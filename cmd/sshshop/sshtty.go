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

package main

import (
	"sync"

	"github.com/gliderlabs/ssh"
	"github.com/unstablebuild/tcell/v3"
)

// sshTty adapts a gliderlabs/ssh session into a tcell.Tty.
//
// tcell uses Tty.Read for decoded input, Tty.Write for rendered output,
// NotifyResize to plumb SIGWINCH-equivalent signals into its event loop,
// and WindowSize to seed the screen dimensions. The SSH session gives us
// all of that directly — there is no real terminal driver involved.
type sshTty struct {
	sess  ssh.Session
	winCh <-chan ssh.Window

	mu       sync.Mutex
	w, h     int
	onResize func()
	stop     chan struct{}
}

// newSSHTty constructs a tcell.Tty bound to the given SSH session. The
// initial window size is taken from the caller; subsequent window-change
// requests are observed on winCh.
func newSSHTty(sess ssh.Session, pty ssh.Pty, winCh <-chan ssh.Window) *sshTty {
	return &sshTty{
		sess:  sess,
		winCh: winCh,
		w:     pty.Window.Width,
		h:     pty.Window.Height,
		stop:  make(chan struct{}),
	}
}

func (t *sshTty) Read(p []byte) (int, error)  { return t.sess.Read(p) }
func (t *sshTty) Write(p []byte) (int, error) { return t.sess.Write(p) }

// Close is a no-op: the SSH session lifecycle is owned by the handler
// in server.go, which calls sess.Close when the handler returns.
func (t *sshTty) Close() error { return nil }

// Start spawns a goroutine that forwards window-change events from the
// SSH channel to tcell's resize callback.
func (t *sshTty) Start() error {
	go func() {
		for {
			select {
			case <-t.stop:
				return
			case w, ok := <-t.winCh:
				if !ok {
					return
				}
				t.mu.Lock()
				t.w, t.h = w.Width, w.Height
				cb := t.onResize
				t.mu.Unlock()
				if cb != nil {
					cb()
				}
			}
		}
	}()
	return nil
}

// Stop terminates the resize goroutine. Safe to call multiple times.
func (t *sshTty) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	select {
	case <-t.stop:
	default:
		close(t.stop)
	}
	return nil
}

// Drain is called by tcell before Stop to wake up any blocked reader.
// We have nothing to drain: gliderlabs cancels the session's context
// when the underlying channel closes, which unblocks Read naturally.
func (t *sshTty) Drain() error { return nil }

// NotifyResize registers the callback that tcell uses to learn about
// window-size changes. A nil callback unregisters.
func (t *sshTty) NotifyResize(cb func()) {
	t.mu.Lock()
	t.onResize = cb
	t.mu.Unlock()
}

// WindowSize returns the latest observed window size from the client.
//
// Some SSH clients (or clients whose stdin is not a real TTY even with
// `-t`) send a pty-req with zero dimensions. tcell divides by width
// internally, so we clamp to a conservative 80x24 in that case.
func (t *sshTty) WindowSize() (tcell.WindowSize, error) {
	t.mu.Lock()
	w, h := t.w, t.h
	t.mu.Unlock()
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	return tcell.WindowSize{Width: w, Height: h}, nil
}
