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
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/gliderlabs/ssh"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"github.com/unstablebuild/tcell/v3"

	"unstable.build/rune/cmd/sshshop/shop"
)

// runSession wires a single SSH session to its own tcell.Screen and
// drives a shop.Root against it via tui.RunScreen.
//
// The Screen is built from a tcell.Tty adapter wrapping the SSH
// session. tui.RunScreen handles draw/flush/event-loop without
// touching any process-global term.* state, so multiple sessions can
// run concurrently inside the same binary.
func runSession(ctx context.Context, sess ssh.Session, log *slog.Logger) error {
	pty, winCh, ok := sess.Pty()
	if !ok {
		_, _ = fmt.Fprintln(sess, "sshshop: a PTY is required. Try: ssh -t <host>")
		return sess.Exit(1)
	}

	tty := newSSHTty(sess, pty, winCh)
	screen, err := tcell.NewTerminfoScreenFromTty(tty)
	if err != nil {
		return fmt.Errorf("new terminfo screen: %w", err)
	}
	if err := screen.Init(); err != nil {
		return fmt.Errorf("init screen: %w", err)
	}
	var finiOnce sync.Once
	fini := func() { finiOnce.Do(screen.Fini) }
	defer fini()

	screen.EnablePaste()
	screen.EnableFocus()
	screen.EnableMouse(tcell.MouseButtonEvents, tcell.MouseDragEvents)

	// Tear the screen down (closing its event channel) when the SSH
	// session context is canceled, so RunScreen returns instead of
	// blocking forever on Poll.
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			fini()
		case <-stop:
		}
	}()

	// A per-session writer so child components can post interrupt
	// events that this screen's event loop will actually observe.
	// term.DefaultWriter / tui.PublishEvent both target the process
	// global writer and would be silently dropped in RunScreen mode.
	termScreen := term.NewTcellScreen(screen)
	writer := term.NewScreenWriter(termScreen)
	sessionInterrupter := writerInterrupter{w: writer}

	root := shop.NewRoot(sess, log,
		// Route term.Interrupter calls (used by the command palette's
		// async completion animation) to this session's screen.
		shop.WithInterrupter(sessionInterrupter),
		// Route child component ScheduleNextTick calls (used by the
		// markdown component's async syntax highlighting) to this
		// session's screen. The SDK's loop runs EventInterrupt
		// UserFunc callbacks on the main goroutine before the next
		// redraw, which is exactly what ScheduleNextTick's contract
		// promises to its callers.
		shop.WithScheduleNextTick(func(fn func()) bool {
			return writer.PublishEvent(term.Event{
				Type:     term.EventInterrupt,
				UserFunc: fn,
			})
		}),
	)
	defer func() { _ = root.Close() }()

	runRoot := shop.NewShaderRoot(root, sessionInterrupter)
	defer func() { _ = runRoot.Close() }()

	return tui.RunWriter(runRoot, writer)
}

// writerInterrupter adapts a *term.TermboxWriter into a term.Interrupter
// so background goroutines can request redraws by posting interrupt
// events onto this writer's screen event queue.
type writerInterrupter struct {
	w *term.ScreenWriter
}

func (wi writerInterrupter) Interrupt(ctx context.Context) error {
	payload, _ := term.PayloadFromContext(ctx)
	if !wi.w.PublishEvent(term.Event{Type: term.EventInterrupt, Raw: payload}) {
		return context.Canceled
	}
	return nil
}
