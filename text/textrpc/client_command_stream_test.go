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

package textrpc

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	sdktextrpc "github.com/unstablebuild/rune-go-sdk/api/textapi/textrpc"
	"google.golang.org/protobuf/proto"
)

func TestCommandClientStreamCompleteCloseDeletesCompleter(t *testing.T) {
	stream := &recordingServerStream{}
	c := newCommandClientStream(context.Background(), stream)

	it, _, err := c.Complete(context.Background(), textapi.Command{Name: "cmd"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.completers.Load(int64(1)); !ok {
		t.Fatal("expected completer to be stored")
	}
	if err := it.Close(); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.completers.Load(int64(1)); ok {
		t.Fatal("expected completer to be deleted on iterator close")
	}
}

type recordingServerStream struct {
	sent []*sdktextrpc.ServerCommandMessage
}

func (r *recordingServerStream) RecvMsg(any) error { return io.EOF }

func (r *recordingServerStream) Send(msg *sdktextrpc.ServerCommandMessage) error {
	r.sent = append(r.sent, msg)
	return nil
}

// waitableServerStream lets a test observe sent messages and feed replies
// that receiveMessages consumes, modeling the real bidirectional stream.
type waitableServerStream struct {
	sent chan *sdktextrpc.ServerCommandMessage
	recv chan *sdktextrpc.ClientCommandMessage
}

func newWaitableServerStream() *waitableServerStream {
	return &waitableServerStream{
		sent: make(chan *sdktextrpc.ServerCommandMessage, 8),
		recv: make(chan *sdktextrpc.ClientCommandMessage, 8),
	}
}

func (s *waitableServerStream) Send(msg *sdktextrpc.ServerCommandMessage) error {
	s.sent <- msg
	return nil
}

func (s *waitableServerStream) RecvMsg(msg any) error {
	next, ok := <-s.recv
	if !ok {
		return io.EOF
	}
	proto.Merge(msg.(*sdktextrpc.ClientCommandMessage), next)
	return nil
}

func (s *waitableServerStream) replyHandle(errStr string) {
	s.recv <- &sdktextrpc.ClientCommandMessage{
		Type:   sdktextrpc.ClientCommandMessage_Handle,
		Handle: &sdktextrpc.HandleCommandResponse{Error: errStr},
	}
}

func TestCommandClientStreamHandleCommandWaiter(t *testing.T) {
	cases := []struct {
		name    string
		errStr  string
		wantErr string
	}{
		{name: "success", errStr: "", wantErr: ""},
		{name: "extension error", errStr: "boom", wantErr: "boom"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stream := newWaitableServerStream()
			c := newCommandClientStream(context.Background(), stream)
			go func() { _ = c.receiveMessages() }()

			w := &Waiter{Ch: make(chan error, 1)}
			ctx := ContextWithWaiter(context.Background(), w)
			if err := c.HandleCommand(ctx, textapi.Command{Name: "cmd"}); err != nil {
				t.Fatalf("HandleCommand returned an error: %v", err)
			}
			if !w.Claimed {
				t.Fatal("HandleCommand must claim the waiter before returning")
			}

			select {
			case <-stream.sent:
			case <-time.After(time.Second):
				t.Fatal("HandleCommand did not send the request")
			}

			select {
			case <-w.Ch:
				t.Fatal("waiter received a result before the extension replied")
			case <-time.After(50 * time.Millisecond):
			}

			stream.replyHandle(tc.errStr)

			select {
			case err := <-w.Ch:
				if tc.wantErr == "" {
					if err != nil {
						t.Fatalf("expected nil error, got %v", err)
					}
				} else if err == nil || err.Error() != tc.wantErr {
					t.Fatalf("expected error %q, got %v", tc.wantErr, err)
				}
			case <-time.After(time.Second):
				t.Fatal("waiter did not receive a result after reply")
			}
		})
	}
}

// A reply for a command dispatched without a Waiter must reach the
// fire-and-forget handleCommand channel (the logger); a reply for a
// command dispatched with a Waiter must reach the waiter instead.
func TestCommandClientStreamHandleResponseRouting(t *testing.T) {
	stream := newWaitableServerStream()
	c := newCommandClientStream(context.Background(), stream)
	go func() { _ = c.receiveMessages() }()

	stream.replyHandle("loose error")
	select {
	case errMsg := <-c.handleCommand:
		if errMsg != "loose error" {
			t.Fatalf("expected loose error on handleCommand, got %q", errMsg)
		}
	case <-time.After(time.Second):
		t.Fatal("unregistered Handle reply did not reach handleCommand")
	}

	w := &Waiter{Ch: make(chan error, 1)}
	ctx := ContextWithWaiter(context.Background(), w)
	if err := c.HandleCommand(ctx, textapi.Command{Name: "cmd"}); err != nil {
		t.Fatalf("HandleCommand returned an error: %v", err)
	}
	select {
	case <-stream.sent:
	case <-time.After(time.Second):
		t.Fatal("HandleCommand did not send the request")
	}

	stream.replyHandle("waited error")
	select {
	case errMsg := <-c.handleCommand:
		t.Fatalf("waited reply leaked to handleCommand: %q", errMsg)
	case err := <-w.Ch:
		if err == nil || err.Error() != "waited error" {
			t.Fatalf("expected waited error, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("waiter did not receive a result after reply")
	}
}

// Interleaved waiting and non-waiting commands on one stream must each
// receive their own reply in send order: because the wire reply has no
// correlation id, a registered waiter must not greedily consume a reply
// belonging to a different command sent between it and its own reply.
func TestCommandClientStreamHandleResponseFIFO(t *testing.T) {
	stream := newWaitableServerStream()
	c := newCommandClientStream(context.Background(), stream)
	go func() { _ = c.receiveMessages() }()

	wA := &Waiter{Ch: make(chan error, 1)}
	wC := &Waiter{Ch: make(chan error, 1)}
	require.NoError(t, c.HandleCommand(
		ContextWithWaiter(context.Background(), wA), textapi.Command{Name: "a"}))
	require.NoError(t, c.HandleCommand(
		context.Background(), textapi.Command{Name: "b"}))
	require.NoError(t, c.HandleCommand(
		ContextWithWaiter(context.Background(), wC), textapi.Command{Name: "c"}))
	for range 3 {
		select {
		case <-stream.sent:
		case <-time.After(time.Second):
			t.Fatal("HandleCommand did not send a request")
		}
	}

	stream.replyHandle("a-result")
	stream.replyHandle("b-result")
	stream.replyHandle("c-result")

	select {
	case err := <-wA.Ch:
		require.EqualError(t, err, "a-result")
	case <-time.After(time.Second):
		t.Fatal("waiter A did not receive its result")
	}
	select {
	case errMsg := <-c.handleCommand:
		require.Equal(t, "b-result", errMsg)
	case <-time.After(time.Second):
		t.Fatal("non-waiting command B's reply did not reach handleCommand")
	}
	select {
	case err := <-wC.Ch:
		require.EqualError(t, err, "c-result")
	case <-time.After(time.Second):
		t.Fatal("waiter C did not receive its result")
	}
}

// Teardown must never mis-deliver: when the extension drops a reply (it
// only does so as its stream context is cancelled, command_stream.go), no
// further replies arrive and receiveMessages exits, so a still-pending
// waiter unblocks via context cancellation rather than receiving the
// wrong command's result off the FIFO.
func TestCommandClientStreamHandleResponseDropOnTeardown(t *testing.T) {
	stream := newWaitableServerStream()
	streamCtx, cancelStream := context.WithCancel(context.Background())
	c := newCommandClientStream(streamCtx, stream)

	recvDone := make(chan error, 1)
	go func() { recvDone <- c.receiveMessages() }()

	wA := &Waiter{Ch: make(chan error, 1)}
	wB := &Waiter{Ch: make(chan error, 1)}
	require.NoError(t, c.HandleCommand(
		ContextWithWaiter(context.Background(), wA), textapi.Command{Name: "a"}))
	require.NoError(t, c.HandleCommand(
		ContextWithWaiter(context.Background(), wB), textapi.Command{Name: "b"}))
	for range 2 {
		select {
		case <-stream.sent:
		case <-time.After(time.Second):
			t.Fatal("HandleCommand did not send a request")
		}
	}

	// A replies normally; B's reply is dropped by the tearing-down
	// extension, so only A's reply ever reaches the client.
	stream.replyHandle("a-result")
	select {
	case err := <-wA.Ch:
		require.EqualError(t, err, "a-result")
	case <-time.After(time.Second):
		t.Fatal("waiter A did not receive its result")
	}

	// The stream tears down: the context is cancelled and the receive
	// side reaches EOF.
	cancelStream()
	close(stream.recv)
	select {
	case <-recvDone:
	case <-time.After(time.Second):
		t.Fatal("receiveMessages did not exit on teardown")
	}

	// B's waiter must not have been handed a stray result; a caller waits
	// on its own context alongside the channel and unblocks via teardown.
	select {
	case err := <-wB.Ch:
		t.Fatalf("pending waiter received a stray result on teardown: %v", err)
	default:
	}
}
