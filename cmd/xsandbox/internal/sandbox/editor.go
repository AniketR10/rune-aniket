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

package sandbox

import (
	"fmt"
	"sync"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"unstable.build/rune/internal/text"
	"unstable.build/rune/internal/text/texttest"
)

// recordingEditor is the sandbox's host-side text.Editor. It records
// command registrations so the spec can wait for them and invoke the
// registered handlers, routing invocations back to the extension over
// its command streams.
type recordingEditor struct {
	*texttest.TestEditor

	mu       sync.Mutex
	commands map[string]text.CommandHandler
	repls    map[string]textapi.REPLHandler
	// changed is closed and replaced whenever a registration is added
	// so waiters can block instead of polling.
	changed chan struct{}
}

func newRecordingEditor() *recordingEditor {
	return &recordingEditor{
		TestEditor: texttest.NopEditor(),
		commands:   make(map[string]text.CommandHandler),
		repls:      make(map[string]textapi.REPLHandler),
		changed:    make(chan struct{}),
	}
}

func (e *recordingEditor) signalLocked() {
	close(e.changed)
	e.changed = make(chan struct{})
}

// SubscribeCommand satisfies text.Editor.
func (e *recordingEditor) SubscribeCommand(
	cmd textapi.CommandManual, h text.CommandHandler,
) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.commands[cmd.Name] = h
	e.signalLocked()
	return nil
}

// UnsubscribeCommand satisfies text.Editor.
func (e *recordingEditor) UnsubscribeCommand(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.commands[name]; !ok {
		return text.ErrCommandNotRegistered
	}
	delete(e.commands, name)
	return nil
}

// RegisterREPLCommand satisfies text.Editor.
func (e *recordingEditor) RegisterREPLCommand(
	cmd textapi.CommandManual, h textapi.REPLHandler,
) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.repls[cmd.Name] = h
	e.signalLocked()
	return nil
}

// UnregisterREPLCommand satisfies text.Editor.
func (e *recordingEditor) UnregisterREPLCommand(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.repls[name]; !ok {
		return text.ErrCommandNotRegistered
	}
	delete(e.repls, name)
	return nil
}

func (e *recordingEditor) lookup(name string, repl bool) (any, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if repl {
		h, ok := e.repls[name]
		return h, ok
	}
	h, ok := e.commands[name]
	return h, ok
}

// waitRegistered blocks until a command with the given name has been
// registered by the extension, or the timeout expires.
func (e *recordingEditor) waitRegistered(
	name string, repl bool, timeout time.Duration, done <-chan struct{},
) (any, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		e.mu.Lock()
		var h any
		var ok bool
		if repl {
			h, ok = e.repls[name]
		} else {
			h, ok = e.commands[name]
		}
		changed := e.changed
		e.mu.Unlock()
		if ok {
			return h, nil
		}
		select {
		case <-changed:
		case <-deadline.C:
			return nil, fmt.Errorf(
				"command %q was not registered within %s (registered: %v)",
				name, timeout, e.registeredNames(repl))
		case <-done:
			return nil, fmt.Errorf("sandbox stopped while waiting for command %q", name)
		}
	}
}

func (e *recordingEditor) registeredNames(repl bool) []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	var names []string
	if repl {
		for name := range e.repls {
			names = append(names, name)
		}
		return names
	}
	for name := range e.commands {
		names = append(names, name)
	}
	return names
}
