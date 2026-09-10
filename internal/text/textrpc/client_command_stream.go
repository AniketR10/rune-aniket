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

package textrpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi/textrpc"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term/termrpc"
)

type commandClientStream struct {
	log           *slog.Logger
	ctx           context.Context
	handleCommand chan string
	stream        serverStream
	completers    sync.Map
	counter       int64
	pendingMu     sync.Mutex
	pending       []chan error
	sendMu        sync.Mutex
	// whether the extension understands CompleteCancel messages; older
	// SDKs terminate the stream when they receive an unknown message type.
	supportsCompleteCancel bool
}

// subset of Editor_SubscribeCommandServer
type serverStream interface {
	RecvMsg(any) error
	Send(*textrpc.ServerCommandMessage) error
}

func newCommandClientStream(
	ctx context.Context, stream serverStream, supportsCompleteCancel bool,
) *commandClientStream {
	log := slog.Default().With("struct", "textrpc.commandClientStream")
	return &commandClientStream{
		log:                    log,
		stream:                 stream,
		ctx:                    ctx,
		handleCommand:          make(chan string),
		supportsCompleteCancel: supportsCompleteCancel,
	}
}

func (c *commandClientStream) send(msg *textrpc.ServerCommandMessage) error {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	return c.stream.Send(msg)
}

func (c *commandClientStream) receiveMessages() error {
	for {
		var msg textrpc.ClientCommandMessage
		err := c.stream.RecvMsg(&msg)
		if err != nil {
			return fmt.Errorf("receive stream message: %w", err)
		}

		switch msg.GetType() {
		case textrpc.ClientCommandMessage_Handle:
			errStr := msg.GetHandle().GetError()
			c.pendingMu.Lock()
			var waitCh chan error
			if len(c.pending) > 0 {
				waitCh = c.pending[0]
				if len(c.pending) == 1 {
					c.pending = c.pending[:0]
				} else {
					c.pending = c.pending[1:]
				}
			}
			c.pendingMu.Unlock()
			if waitCh != nil {
				var err error
				if errStr != "" {
					err = errors.New(errStr)
				}
				select {
				case waitCh <- err:
				case <-c.ctx.Done():
					return c.ctx.Err()
				}
				continue
			}
			select {
			case c.handleCommand <- errStr:
			case <-c.ctx.Done():
				return c.ctx.Err()
			}
		case textrpc.ClientCommandMessage_CompleteValue:
			complete := msg.GetCompleteValue()
			id := complete.GetId()
			if id == 0 {
				c.log.Warn("received complete value message with invalid id")
				continue
			}
			val, ok := c.completers.Load(id)
			if !ok {
				c.log.Warn("received complete message for unknown stream")
				continue
			}
			chanCtx := val.(chanCtx)
			if chanCtx.cancelled {
				continue
			}
			chanValue := chanValue{
				val: complete.GetValue(),
			}
			select {
			case chanCtx.ch <- chanValue:
			case <-chanCtx.ctx.Done():
				continue
			case <-c.ctx.Done():
				return c.ctx.Err()
			}
		case textrpc.ClientCommandMessage_CompleteDone:
			done := msg.GetCompleteDone()
			id := done.GetId()
			if id == 0 {
				c.log.Warn("received complete done message with invalid id")
				continue
			}
			val, ok := c.completers.LoadAndDelete(id)
			if !ok {
				continue
			}
			chanCtx := val.(chanCtx)
			if chanCtx.cancelled {
				continue
			}
			errStr := done.GetError()
			if errStr == "" {
				close(chanCtx.ch)
				continue
			}

			chanValue := chanValue{
				err: errors.New(errStr),
			}
			select {
			case chanCtx.ch <- chanValue:
				continue
			case <-chanCtx.ctx.Done():
				continue
			case <-c.ctx.Done():
				return c.ctx.Err()
			}
		default:
			c.log.Warn("received extraneous message type", "type", msg.GetType())
		}
	}
}

func (c *commandClientStream) HandleCommand(
	ctx context.Context, cmd textapi.Command,
) error {
	// Every send reserves a slot in the reply FIFO (held across Send so
	// the slot order matches the wire order). A caller that attaches a
	// Waiter reserves its channel and is told, via Claimed, to wait for
	// the result on it; otherwise the slot is nil and the reply takes the
	// fire-and-forget log path. Claim must happen before this returns.
	var replyCh chan error
	if w, ok := WaiterFromContext(ctx); ok {
		replyCh = w.Ch
		w.Claimed = true
	}
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	c.pending = append(c.pending, replyCh)
	if err := c.send(c.buildHandleRequest(cmd)); err != nil {
		c.pending = c.pending[:len(c.pending)-1]
		return fmt.Errorf("send complete request: %w", err)
	}
	return nil
}

func (c *commandClientStream) buildHandleRequest(
	cmd textapi.Command,
) *textrpc.ServerCommandMessage {
	var cursorContent, cursorWindow termrpc.Coordinates
	cursorContent.FromModel(cmd.Cursor.Content)
	cursorWindow.FromModel(cmd.Cursor.Window)

	req := textrpc.HandleCommandRequest{
		Name:          cmd.Name,
		Args:          cmd.Args,
		CursorContent: &cursorContent,
		CursorWindow:  &cursorWindow,
	}
	if cmd.Window != nil {
		req.WindowId = cmd.Window.WindowID()
	}
	if cmd.URI != (workspaceapi.URI{}) {
		req.ResourceName = NewURI(cmd.URI)
	}

	var reqMsg textrpc.ServerCommandMessage
	reqMsg.Type = textrpc.ServerCommandMessage_Handle
	reqMsg.Handle = &req
	return &reqMsg
}

func (c *commandClientStream) Complete(ctx context.Context, cmd textapi.Command) (
	iterator.Iterator[string], string, error,
) {
	c.counter++ // start with 1, so 0 is a missing ID error
	id := c.counter

	var req textrpc.CompleteCommandRequest
	req.Id = id
	req.Name = cmd.Name
	req.Args = cmd.Args
	// the rest of fields are not propagated, since textapi.CommandHandler
	// doesn't have he same signature as text.CommandHandler.

	var reqMsg textrpc.ServerCommandMessage
	reqMsg.Type = textrpc.ServerCommandMessage_Complete
	reqMsg.Complete = &req

	ctx, cancelCtx := context.WithCancel(ctx)
	ch := make(chan chanValue)
	c.completers.Store(id, chanCtx{ctx: ctx, ch: ch})
	if err := c.send(&reqMsg); err != nil {
		cancelCtx()
		c.completers.Delete(id)
		return nil, "", fmt.Errorf("send complete request: %w", err)
	}

	return iterator.FromFunc(func(ctx context.Context) (string, bool, error) {
		select {
		case next, ok := <-ch:
			if !ok {
				return "", false, nil
			}
			if next.err != nil {
				close(ch) // force next to return !ok, in case of poor impls
				return "", false, next.err
			}
			return next.val, true, nil
		case <-ctx.Done():
			return "", false, ctx.Err()
		}
	}, func() error {
		cancelCtx()
		// keep a tombstone so values still in flight for this abandoned
		// completion are dropped silently; CompleteDone clears it.
		c.completers.Store(id, chanCtx{cancelled: true})
		if !c.supportsCompleteCancel {
			return nil
		}
		cancelMsg := textrpc.ServerCommandMessage{
			Type:           textrpc.ServerCommandMessage_CompleteCancel,
			CompleteCancel: &textrpc.CompleteCommandCancel{Id: id},
		}
		if err := c.send(&cancelMsg); err != nil {
			c.log.Warn("send complete cancel", "error", err)
		}
		return nil
	}), "", nil
}

// enables canceling the iterator from two different places:
// iterator.Close, which produces a ctx.Err() error
// and when the producer is done sending values
// via the corresponding proto message, in which case the channel
// is closed and we gracefully terminate the iterator.
type chanCtx struct {
	ctx context.Context
	ch  chan chanValue
	// tombstone for a completion abandoned by the editor: its id stays
	// known until the extension reports the completion done.
	cancelled bool
}

type chanValue struct {
	val string
	err error
}
