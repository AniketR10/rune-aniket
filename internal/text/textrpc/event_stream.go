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
	"io"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi/textrpc"
	"unstable.build/rune/internal/debug"
)

var _ textapi.EventHandler = (*eventStreamClient)(nil)

const (
	eventChanBuffer = 100
)

type eventStreamClient struct {
	ctx       context.Context
	cancelCtx func()
	stream    textrpc.Editor_SubscribeEventServer
	ch        chan *textrpc.EditorEvent
}

func newEventStreamClient(
	ctx context.Context, stream textrpc.Editor_SubscribeEventServer, locker sync.Locker,
) *eventStreamClient {
	ctx, cancel := context.WithCancel(ctx)
	ch := make(chan *textrpc.EditorEvent, eventChanBuffer)
	ret := &eventStreamClient{
		stream:    stream,
		ctx:       ctx,
		cancelCtx: cancel,
		ch:        ch,
	}
	go debug.CapturePanicReport(func() {
		ret.sendMessages(ctx)
	})
	return ret
}

func (e *eventStreamClient) sendMessages(ctx context.Context) {
	for {
		select {
		case ev := <-e.ch:
			err := e.stream.Send(ev)
			if err != nil {
				e.log(log.ErrorLevel, "stop sending messages: stream send: %v", err)
				return
			}
		case <-ctx.Done():
			e.log(log.TraceLevel, "stop sending messages: %v", ctx.Err())
			return
		}
	}
}

func (e *eventStreamClient) Handle(ctx context.Context, ev textapi.Event) bool {
	e.log(log.TraceLevel, "handle %v", ev.Type)
	protoEv := toProto(ev)

	// do not unlock I/O mutex here, as it might introduce
	// race conditions and violate invariants that are quite hard
	// to debug.

	select {
	case e.ch <- &protoEv:
	case <-e.ctx.Done():
		e.log(log.TraceLevel, "unsubscribing")
		return true
	default:
		e.log(log.ErrorLevel, "event stream is lagging behind: dropping messages")
	}
	return false
}

func (e *eventStreamClient) Close() error {
	e.cancelCtx()
	return nil
}

func (e *eventStreamClient) waitForUnsubscribe() error {
	req, err := e.stream.Recv()
	if err != nil {
		return fmt.Errorf("stream receive: %v", err)
	}
	if !req.GetUnsubscribe() {
		e.log(log.WarnLevel, "received message non-unsubscribe request")
	}

	// wait for CloseSend
	if _, err = e.stream.Recv(); !errors.Is(err, io.EOF) {
		return fmt.Errorf("stream receive: %v", err)
	}

	return nil
}

func (e *eventStreamClient) log(level log.Level, msg string, args ...any) {
	if !log.IsLevelEnabled(level) {
		return
	}
	log.WithFields(log.Fields{logging.KeyClass: "textrpc.eventStreamClient"}).
		Logf(level, msg, args...)
}
