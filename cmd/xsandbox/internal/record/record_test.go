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

package record

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi/browserrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func notifyReq(msg string) *browserrpc.NotifyRequest {
	return &browserrpc.NotifyRequest{Msg: msg}
}

func interceptUnary(
	t *testing.T, r *Recorder, method string, req proto.Message,
	handler grpc.UnaryHandler,
) (any, error) {
	t.Helper()
	return r.UnaryInterceptor()(
		context.Background(), req,
		&grpc.UnaryServerInfo{FullMethod: method}, handler)
}

func TestRecorderUnaryInterceptorRecords(t *testing.T) {
	t.Parallel()

	r := NewRecorder()
	resp, err := interceptUnary(t, r, "/browser.Notifications/Notify",
		notifyReq("hello"),
		func(context.Context, any) (any, error) {
			return &browserrpc.NotifyResponse{}, nil
		})
	require.NoError(t, err)
	require.NotNil(t, resp)

	snaps := r.Snapshots()
	require.Len(t, snaps, 1)
	assert.Equal(t, "browser.Notifications/Notify", snaps[0].Method)
	assert.Equal(t, KindUnary, snaps[0].Kind)
	assert.Equal(t, "hello", snaps[0].Request["msg"])
	assert.False(t, snaps[0].End.IsZero())
	assert.False(t, snaps[0].Consumed)
}

func TestRecorderUnaryInterceptorRecordsHandlerError(t *testing.T) {
	t.Parallel()

	r := NewRecorder()
	_, err := interceptUnary(t, r, "/browser.Notifications/Notify",
		notifyReq("x"),
		func(context.Context, any) (any, error) {
			return nil, errors.New("boom")
		})
	require.Error(t, err)
	snaps := r.Snapshots()
	require.Len(t, snaps, 1)
	assert.Equal(t, "boom", snaps[0].Err)
}

func TestRecorderResponderShortCircuits(t *testing.T) {
	t.Parallel()

	r := NewRecorder()
	r.AddResponder("browser.Notifications/Notify",
		NewMatcher(map[string]any{"msg": "scripted"}),
		&Response{Body: map[string]any{"id": "canned"}})

	handlerCalled := false
	handler := func(context.Context, any) (any, error) {
		handlerCalled = true
		return &browserrpc.NotifyResponse{}, nil
	}

	// Non-matching request goes to the real handler.
	_, err := interceptUnary(t, r, "/browser.Notifications/Notify",
		notifyReq("other"), handler)
	require.NoError(t, err)
	assert.True(t, handlerCalled)

	// Matching request is served the scripted body.
	handlerCalled = false
	resp, err := interceptUnary(t, r, "/browser.Notifications/Notify",
		notifyReq("scripted"), handler)
	require.NoError(t, err)
	assert.False(t, handlerCalled)
	data, err := proto.Marshal(resp.(proto.Message))
	require.NoError(t, err)
	var decoded browserrpc.NotifyResponse
	require.NoError(t, proto.Unmarshal(data, &decoded))
	assert.Equal(t, "canned", decoded.GetId())
}

func TestRecorderResponderError(t *testing.T) {
	t.Parallel()

	r := NewRecorder()
	r.AddResponder("browser.Notifications/Notify", nil,
		&Response{ErrCode: "unavailable", ErrMsg: "scripted outage"})

	_, err := interceptUnary(t, r, "/browser.Notifications/Notify",
		notifyReq("x"),
		func(context.Context, any) (any, error) {
			t.Fatal("handler must not run")
			return nil, nil
		})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unavailable, st.Code())
	assert.Equal(t, "scripted outage", st.Message())
}

func TestRecorderWaitMatchConsumesEarliest(t *testing.T) {
	t.Parallel()

	r := NewRecorder()
	for _, msg := range []string{"first", "second"} {
		_, err := interceptUnary(t, r, "/browser.Notifications/Notify",
			notifyReq(msg),
			func(context.Context, any) (any, error) {
				return &browserrpc.NotifyResponse{}, nil
			})
		require.NoError(t, err)
	}

	snap, err := r.WaitMatch(context.Background(),
		"browser.Notifications/Notify", nil, time.Second)
	require.NoError(t, err)
	assert.Equal(t, "first", snap.Request["msg"])

	snap, err = r.WaitMatch(context.Background(),
		"browser.Notifications/Notify", nil, time.Second)
	require.NoError(t, err)
	assert.Equal(t, "second", snap.Request["msg"])

	// Everything consumed now.
	assert.Empty(t, r.Unconsumed(nil))
}

func TestRecorderWaitMatchBlocksUntilArrival(t *testing.T) {
	t.Parallel()

	r := NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		time.Sleep(50 * time.Millisecond)
		_, _ = interceptUnary(t, r, "/browser.Notifications/Notify",
			notifyReq("late"),
			func(context.Context, any) (any, error) {
				return &browserrpc.NotifyResponse{}, nil
			})
	}()
	snap, err := r.WaitMatch(context.Background(),
		"browser.Notifications/Notify",
		NewMatcher(map[string]any{"msg": "late"}), 5*time.Second)
	require.NoError(t, err)
	assert.Equal(t, "late", snap.Request["msg"])
	<-done
}

func TestRecorderWaitMatchTimeout(t *testing.T) {
	t.Parallel()

	r := NewRecorder()
	_, err := interceptUnary(t, r, "/browser.Notifications/Notify",
		notifyReq("other"),
		func(context.Context, any) (any, error) {
			return &browserrpc.NotifyResponse{}, nil
		})
	require.NoError(t, err)

	_, err = r.WaitMatch(context.Background(),
		"browser.Notifications/Notify",
		NewMatcher(map[string]any{"msg": "never"}), 20*time.Millisecond)
	var timeoutErr *MatchTimeoutError
	require.ErrorAs(t, err, &timeoutErr)
	assert.Contains(t, err.Error(), "1 unmatched call(s)")
}

func TestRecorderWaitMatchContextCanceled(t *testing.T) {
	t.Parallel()

	r := NewRecorder()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.WaitMatch(ctx, "x.Y/Z", nil, time.Second)
	require.ErrorIs(t, err, context.Canceled)
}

func TestRecorderWaitIdle(t *testing.T) {
	t.Parallel()

	r := NewRecorder()
	start := time.Now()
	require.NoError(t, r.WaitIdle(context.Background(), 30*time.Millisecond))
	assert.GreaterOrEqual(t, time.Since(start), 30*time.Millisecond)
}

func TestRecorderUnconsumedIgnores(t *testing.T) {
	t.Parallel()

	r := NewRecorder()
	_, err := interceptUnary(t, r, "/browser.Notifications/Notify",
		notifyReq("x"),
		func(context.Context, any) (any, error) {
			return &browserrpc.NotifyResponse{}, nil
		})
	require.NoError(t, err)

	assert.Len(t, r.Unconsumed(nil), 1)
	assert.Empty(t, r.Unconsumed([]string{"browser.Notifications/*"}))
}

type fakeServerStream struct {
	grpc.ServerStream
	msgs []proto.Message
}

func (s *fakeServerStream) Context() context.Context { return context.Background() }

func (s *fakeServerStream) RecvMsg(m any) error {
	if len(s.msgs) == 0 {
		return errors.New("EOF")
	}
	next := s.msgs[0]
	s.msgs = s.msgs[1:]
	proto.Merge(m.(proto.Message), next)
	return nil
}

func TestRecorderStreamInterceptorRecordsMessages(t *testing.T) {
	t.Parallel()

	r := NewRecorder()
	stream := &fakeServerStream{msgs: []proto.Message{
		notifyReq("one"), notifyReq("two"),
	}}
	err := r.StreamInterceptor()(nil, stream,
		&grpc.StreamServerInfo{FullMethod: "/text.Editor/SubscribeCommand"},
		func(_ any, ss grpc.ServerStream) error {
			for {
				var msg browserrpc.NotifyRequest
				if err := ss.RecvMsg(&msg); err != nil {
					return nil
				}
			}
		})
	require.NoError(t, err)

	snaps := r.Snapshots()
	require.Len(t, snaps, 1)
	assert.Equal(t, KindStream, snaps[0].Kind)
	assert.Equal(t, "text.Editor/SubscribeCommand", snaps[0].Method)
	require.Len(t, snaps[0].Messages, 2)
	assert.Equal(t, "one", snaps[0].Request["msg"])
	assert.Equal(t, "two", snaps[0].Messages[1]["msg"])
}

func TestResponseBuildErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		method  string
		resp    Response
		wantErr string
	}{
		{
			name:    "unknown service",
			method:  "/nope.Nope/Get",
			resp:    Response{Body: map[string]any{}},
			wantErr: "unknown rpc service",
		},
		{
			name:    "unknown method",
			method:  "/browser.Notifications/Nope",
			resp:    Response{Body: map[string]any{}},
			wantErr: "unknown rpc method",
		},
		{
			name:    "bad field",
			method:  "/browser.Notifications/Notify",
			resp:    Response{Body: map[string]any{"not_a_field": 1}},
			wantErr: "does not decode",
		},
		{
			name:    "bad error code",
			method:  "/browser.Notifications/Notify",
			resp:    Response{ErrCode: "nope", ErrMsg: "x"},
			wantErr: "unknown grpc error code",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := tc.resp.build(tc.method)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
