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

// Package registerset provides a register-aware clipboard.Register implementation.
package registerset

import (
	"sync"
	"unicode"

	"github.com/unstablebuild/rune-go-sdk/clipboard"
)

var _ clipboard.Register = (*registerSet)(nil)

const (
	// UnnamedRegisterID identifies the Vim-compatible unnamed register.
	UnnamedRegisterID = `"`
	// LastYankRegisterID identifies the Vim-compatible last-yank register.
	LastYankRegisterID = "0"
	// ClipboardRegisterID identifies the system clipboard register.
	ClipboardRegisterID = "+"
	// BlackHoleRegisterID identifies the black-hole register.
	BlackHoleRegisterID = "_"
)

// New returns a register-aware clipboard backed by clip.
func New(clip clipboard.Register) clipboard.Register {
	if registers, ok := clip.(*registerSet); ok {
		return registers
	}
	return &registerSet{
		clipboard: clip,
		data:      make(map[string]clipboard.Data),
	}
}

// registerSet stores independent clipboard data for each register ID.
type registerSet struct {
	mu        sync.RWMutex
	clipboard clipboard.Register
	data      map[string]clipboard.Data
}

// Copy satisfies clipboard.Register.
func (r *registerSet) Copy(registerID string, data clipboard.Data) error {
	registerID = Normalize(registerID)
	switch registerID {
	case BlackHoleRegisterID:
		return nil
	case clipboard.DefaultRegisterID, ClipboardRegisterID:
		return r.clipboard.Copy(clipboard.DefaultRegisterID, data)
	}

	r.mu.Lock()
	r.data[registerID] = data
	r.mu.Unlock()
	return nil
}

// Paste satisfies clipboard.Register.
func (r *registerSet) Paste(registerID string) (clipboard.Data, error) {
	registerID = Normalize(registerID)
	if registerID == clipboard.DefaultRegisterID || registerID == ClipboardRegisterID {
		return r.clipboard.Paste(clipboard.DefaultRegisterID)
	}

	r.mu.RLock()
	data := r.data[registerID]
	r.mu.RUnlock()
	return data, nil
}

// Normalize converts aliases and letter case to canonical register IDs.
func Normalize(registerID string) string {
	if registerID == "" || registerID == clipboard.DefaultRegisterID || registerID == UnnamedRegisterID {
		return clipboard.DefaultRegisterID
	}
	for _, name := range registerID {
		return string(unicode.ToLower(name))
	}
	return clipboard.DefaultRegisterID
}
