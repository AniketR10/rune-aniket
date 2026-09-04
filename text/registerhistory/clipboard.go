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

// Package registerhistory provides clipboard history on top of clipboard.Register.
package registerhistory

import (
	"sync"

	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"unstable.build/rune/text/registerset"
)

var _ clipboard.Register = (*Clipboard)(nil)
var _ History = (*Clipboard)(nil)

// maxHistory is the maximum number of clipboard entries to retain.
const maxHistory = 100

// History provides read access to the clipboard paste history.
type History interface {
	// HistoryLen returns the number of entries in the clipboard history.
	HistoryLen() int
	// HistoryAt returns the clipboard entry at the given index (0 = most recent).
	HistoryAt(index int) (clipboard.Data, bool)
}

// AsHistory returns a History if the clipboard supports history tracking.
func AsHistory(clip clipboard.Register) (History, bool) {
	h, ok := clip.(History)
	return h, ok
}

// NewClipboard returns a clipboard.Register that tracks paste history.
func NewClipboard(clip clipboard.Register) clipboard.Register {
	if history, ok := clip.(*Clipboard); ok {
		return history
	}
	return &Clipboard{clipboard: clip}
}

// Clipboard tracks history for a wrapped clipboard.Register.
type Clipboard struct {
	mu        sync.RWMutex
	clipboard clipboard.Register
	history   []clipboard.Data
}

// Copy satisfies clipboard.Register.
func (c *Clipboard) Copy(registerID string, data clipboard.Data) error {
	if err := c.clipboard.Copy(registerID, data); err != nil {
		return err
	}
	switch registerset.Normalize(registerID) {
	case clipboard.DefaultRegisterID, registerset.ClipboardRegisterID:
		c.mu.Lock()
		c.appendHistory(data)
		c.mu.Unlock()
	}
	return nil
}

// Paste satisfies clipboard.Register.
func (c *Clipboard) Paste(registerID string) (clipboard.Data, error) {
	return c.clipboard.Paste(registerID)
}

// appendHistory adds a clipboard entry to the history ring.
// Must be called with c.mu held.
func (c *Clipboard) appendHistory(data clipboard.Data) {
	if len(c.history) >= maxHistory {
		copy(c.history, c.history[1:])
		c.history[len(c.history)-1] = data
	} else {
		c.history = append(c.history, data)
	}
}

// HistoryLen returns the number of entries in the clipboard history.
func (c *Clipboard) HistoryLen() int {
	c.mu.RLock()
	n := len(c.history)
	c.mu.RUnlock()
	return n
}

// HistoryAt returns the clipboard entry at the given index (0 = most recent).
func (c *Clipboard) HistoryAt(index int) (clipboard.Data, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if index < 0 || index >= len(c.history) {
		return clipboard.Data{}, false
	}
	// Index 0 is the most recent entry (last in slice).
	return c.history[len(c.history)-1-index], true
}
