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

package extension

import (
	"context"
	"log/slog"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"unstable.build/rune/cmd/rune-agent/dialogue/dialoguemanager"
)

// compactIterator is a lazy iterator for the /compact command.
// The first call to Next blocks while the agentshell runs the LLM compact
// call, then returns false (no items are yielded to AddCommand). Close
// triggers the visual reset and replay of compacted messages.
//
// AddCommand's deferred cleanup order is:
//  1. Remove(animNode) — safe because Reset has not been called yet
//  2. it.Close()       — calls compactFn which does Reset + replay
type compactIterator struct {
	handler    repl.CommandHandler
	store      dialoguemanager.Store
	dialogueID string
	model      string
	compactFn  func(msgs []llmapi.Message)

	called bool
	closed bool
	err    error
}

func (c *compactIterator) Next(ctx context.Context) (component.Responsive, bool) {
	if c.called {
		return nil, false
	}
	c.called = true

	// This call blocks while the LLM summarises the conversation.
	args := []string{"compact", c.dialogueID}
	if c.model != "" {
		args = append(args, c.model)
	}
	it, err := c.handler.HandleCommand(ctx, repl.Command{
		Name: "chats", Args: args,
	}, repl.NopProgressWriter())
	if err != nil {
		c.err = err
		return nil, false
	}
	drainErr := it.Close() // discard UI elements
	if drainErr != nil {
		c.err = drainErr
		return nil, false
	}
	return nil, false
}

func (c *compactIterator) Err() error { return c.err }

func (c *compactIterator) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	if c.err != nil || c.compactFn == nil || c.store == nil {
		return nil
	}
	d, err := c.store.Get(context.Background(), c.dialogueID)
	if err != nil {
		slog.Error("compact: fetch compacted dialogue", "id", c.dialogueID, "error", err)
		return nil
	}
	c.compactFn(d.Messages)
	return nil
}
