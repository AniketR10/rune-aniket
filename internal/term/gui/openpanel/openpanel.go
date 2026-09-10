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

// Package openpanel shows the platform's native open dialog. On macOS
// it is backed by NSOpenPanel; on other platforms every request
// completes immediately as cancelled.
package openpanel

import (
	"strings"
	"sync"
)

// Options configure the open panel.
type Options struct {
	// Title is the panel's window title.
	Title string
	// Message is explanatory text displayed above the file browser.
	Message string
	// Prompt overrides the confirm button label ("Open" by default).
	Prompt string
	// Directories selects directories instead of files.
	Directories bool
	// Multiple allows selecting more than one entry.
	Multiple bool
}

var (
	mu      sync.Mutex
	nextTag = 1
	pending = map[int]func([]string){}
)

// Show opens the native open panel and invokes done with the selected
// paths once the user confirms. done(nil) signals cancellation, or
// platforms without a native panel. Show does not block: the panel is
// window-modal and done fires during regular event dispatch.
//
// It must be called on the main thread; done is invoked on the main
// thread as well.
func Show(opts Options, done func(paths []string)) {
	showPanel(register(done), opts)
}

// register stores done under a fresh tag so the platform completion
// can be routed back to it. Registration is single-use: finish removes
// the entry.
func register(done func([]string)) int {
	mu.Lock()
	defer mu.Unlock()
	tag := nextTag
	nextTag++
	pending[tag] = done
	return tag
}

// finish routes the platform completion for tag to its registered
// callback.
func finish(tag int, paths []string) {
	mu.Lock()
	done, ok := pending[tag]
	delete(pending, tag)
	mu.Unlock()

	if ok && done != nil {
		done(paths)
	}
}

// splitPaths decodes the NUL-joined path list produced by the platform
// layer. The empty payload signals cancellation and decodes to nil.
func splitPaths(joined string) []string {
	if joined == "" {
		return nil
	}
	return strings.Split(joined, "\x00")
}
