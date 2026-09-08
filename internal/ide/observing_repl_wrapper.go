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

package ide

import (
	"context"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// observingREPLHandler decorates a companion-console REPL handler so that
// every submitted command is reported to the IDE commandObserver. The
// submission is reported uniformly as a "console" command — matching the
// existing "console" ex-command observation — with the REPL command name
// prepended to its arguments. This lets tutorials observe companion-console
// REPL submissions (e.g. "pkg install rune-agent") through the same
// wait_command(command="console") path used for ex commands.
type observingREPLHandler struct {
	underlying textapi.REPLHandler
	observer   commandObserver
	name       string
	// schedule marshals the observer callback onto the event loop. The
	// companion-console REPL runs HandleCommand (and the returned
	// iterator's Next/Close) on a transient console goroutine, off the
	// event loop, while the observer mutates event-loop-owned state
	// (e.g. tutorialRunner.overlay). schedule is an invariant non-nil
	// dependency supplied at construction.
	schedule func(func()) bool
}

var _ textapi.REPLHandler = observingREPLHandler{}

// HandleCommand forwards to the underlying handler and reports the
// submission to the observer. A failed dispatch is reported immediately so
// tutorial wait_command steps stay armed with their error hint. A
// successful dispatch is reported only when the returned output iterator
// completes, so commands whose work continues after dispatch (e.g. the
// asynchronous `models providers <p> add` key prompt) are observed as
// finished only once that work resolves.
func (w observingREPLHandler) HandleCommand(
	ctx context.Context, cmd repl.Command, pw repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	it, err := w.underlying.HandleCommand(ctx, cmd, pw)
	if w.observer == nil {
		return it, err
	}
	args := append([]string{w.name}, cmd.Args...)
	if err != nil {
		w.schedule(func() {
			w.observer.observeCommand("console", "console", args, err)
		})
		return it, err
	}
	return &observingCompletionIter{
		inner: it,
		fire: func() {
			w.schedule(func() {
				w.observer.observeCommand("console", "console", args, nil)
			})
		},
	}, nil
}

// observingCompletionIter wraps a REPL command's output iterator and fires
// fire exactly once, when the iterator is first exhausted or closed.
type observingCompletionIter struct {
	inner iterator.Iterator[component.Responsive]
	fire  func()
	once  sync.Once
}

func (w *observingCompletionIter) Next(
	ctx context.Context,
) (component.Responsive, bool) {
	v, ok := w.inner.Next(ctx)
	if !ok {
		w.once.Do(w.fire)
	}
	return v, ok
}

func (w *observingCompletionIter) Err() error { return w.inner.Err() }

func (w *observingCompletionIter) Close() error {
	w.once.Do(w.fire)
	return w.inner.Close()
}

// Complete forwards completion requests unchanged.
func (w observingREPLHandler) Complete(
	ctx context.Context, cmd string, args []string,
) (iterator.Iterator[string], error) {
	return w.underlying.Complete(ctx, cmd, args)
}

// Help forwards help requests unchanged.
func (w observingREPLHandler) Help(
	ctx context.Context, args []string,
) (iterator.Iterator[component.Responsive], error) {
	return w.underlying.Help(ctx, args)
}
