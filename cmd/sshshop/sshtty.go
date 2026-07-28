// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

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
