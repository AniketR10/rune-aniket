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

package sandbox

import (
	"fmt"
	"sync"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/text/texttest"
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
