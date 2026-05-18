// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package extension

import (
	"context"
	"log/slog"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguemanager"
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
	args       []string
	store      dialoguemanager.Store
	dialogueID string
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
	it, err := c.handler.HandleCommand(ctx, repl.Command{
		Name: "chats", Args: append([]string{"compact"}, c.args...),
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
