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

// Package record observes inbound extension RPCs via gRPC server
// interceptors, matches them against spec expectations, serves
// scripted responses, and aggregates timing statistics for
// benchmarking SDK implementations.
package record

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Kind discriminates unary calls from streams.
type Kind string

// Kinds of recorded RPCs.
const (
	KindUnary  Kind = "unary"
	KindStream Kind = "stream"
)

// Event is one observed inbound RPC. For unary calls Request holds
// the request payload; for streams it holds the first client message
// (nil until one arrives) and Messages holds every client message.
type Event struct {
	ID       int64
	Method   string
	Kind     Kind
	Start    time.Time
	End      time.Time
	Err      string
	Request  map[string]any
	Messages []map[string]any

	consumed bool
}

// Snapshot is a copy of an event safe to read without holding the
// recorder lock.
type Snapshot struct {
	Method   string           `json:"method"`
	Kind     Kind             `json:"kind"`
	Start    time.Time        `json:"start"`
	End      time.Time        `json:"end,omitzero"`
	Err      string           `json:"error,omitempty"`
	Request  map[string]any   `json:"request,omitempty"`
	Messages []map[string]any `json:"messages,omitempty"`
	Consumed bool             `json:"consumed"`
}

// Recorder records every inbound RPC and serves scripted responses.
// It is safe for concurrent use.
type Recorder struct {
	mu         sync.Mutex
	events     []*Event
	responders []*responder
	nextID     int64
	lastChange time.Time
	// changed is closed and replaced on every mutation so waiters can
	// block with a timeout instead of polling.
	changed chan struct{}
}

type responder struct {
	method string
	match  *Matcher
	resp   *Response
}

// Response is a scripted reply for a unary RPC: either a message body
// decoded from Body or a gRPC error.
type Response struct {
	// Body is decoded into the method's response message using
	// proto field names.
	Body map[string]any
	// ErrCode/ErrMsg produce a gRPC status error instead of a body.
	ErrCode string
	ErrMsg  string
}

// NewRecorder returns an empty Recorder.
func NewRecorder() *Recorder {
	return &Recorder{
		changed:    make(chan struct{}),
		lastChange: time.Now(),
	}
}

// NormalizeMethod converts a gRPC full method ("/pkg.Svc/Method") to
// the spec form "pkg.Svc/Method".
func NormalizeMethod(m string) string {
	return strings.TrimPrefix(m, "/")
}

func (r *Recorder) signalLocked() {
	r.lastChange = time.Now()
	close(r.changed)
	r.changed = make(chan struct{})
}

func (r *Recorder) start(method string, kind Kind, req map[string]any) *Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	ev := &Event{
		ID:      r.nextID,
		Method:  NormalizeMethod(method),
		Kind:    kind,
		Start:   time.Now(),
		Request: req,
	}
	if req != nil {
		ev.Messages = append(ev.Messages, req)
	}
	r.events = append(r.events, ev)
	r.signalLocked()
	return ev
}

func (r *Recorder) finish(ev *Event, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ev.End = time.Now()
	if err != nil {
		ev.Err = err.Error()
	}
	r.signalLocked()
}

func (r *Recorder) addMessage(ev *Event, msg map[string]any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ev.Request == nil {
		ev.Request = msg
	}
	ev.Messages = append(ev.Messages, msg)
	r.signalLocked()
}

// AddResponder installs a scripted response for unary calls to
// method whose request matches m (nil matches all). Responders are
// persistent and matched in installation order; the first match wins.
func (r *Recorder) AddResponder(method string, m *Matcher, resp *Response) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.responders = append(r.responders, &responder{
		method: NormalizeMethod(method), match: m, resp: resp,
	})
}

func (r *Recorder) findResponse(method string, req map[string]any) *Response {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rr := range r.responders {
		if rr.method != method {
			continue
		}
		if rr.match != nil && !rr.match.Match(req) {
			continue
		}
		return rr.resp
	}
	return nil
}

// payloadToMap renders a proto message with proto field names so spec
// matchers use the field names from the .proto files.
var payloadMarshal = protojson.MarshalOptions{UseProtoNames: true}

// Snapshots returns a copy of all recorded events in arrival order.
func (r *Recorder) Snapshots() []Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	ret := make([]Snapshot, 0, len(r.events))
	for _, ev := range r.events {
		ret = append(ret, Snapshot{
			Method:   ev.Method,
			Kind:     ev.Kind,
			Start:    ev.Start,
			End:      ev.End,
			Err:      ev.Err,
			Request:  ev.Request,
			Messages: ev.Messages,
			Consumed: ev.consumed,
		})
	}
	return ret
}

// Unconsumed returns snapshots of events not matched by any
// expectation, filtered by ignore patterns (exact method names or
// path.Match-style globs).
func (r *Recorder) Unconsumed(ignore []string) []Snapshot {
	ret := make([]Snapshot, 0)
	for _, ev := range r.Snapshots() {
		if ev.Consumed || methodIgnored(ev.Method, ignore) {
			continue
		}
		ret = append(ret, ev)
	}
	return ret
}

// WaitMatch blocks until an unconsumed event for method with a client
// message matching m exists (a nil matcher matches on arrival), marks
// it consumed and returns its snapshot. It fails after timeout or
// when ctx is done.
func (r *Recorder) WaitMatch(
	ctx context.Context, method string, m *Matcher, timeout time.Duration,
) (Snapshot, error) {
	method = NormalizeMethod(method)
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		r.mu.Lock()
		for _, ev := range r.events {
			if ev.consumed || ev.Method != method {
				continue
			}
			if !eventMatches(ev, m) {
				continue
			}
			ev.consumed = true
			snap := Snapshot{
				Method: ev.Method, Kind: ev.Kind, Start: ev.Start,
				End: ev.End, Err: ev.Err, Request: ev.Request,
				Messages: ev.Messages, Consumed: true,
			}
			r.mu.Unlock()
			return snap, nil
		}
		changed := r.changed
		r.mu.Unlock()
		select {
		case <-changed:
		case <-deadline.C:
			return Snapshot{}, &MatchTimeoutError{
				Method: method, Matcher: m, Observed: r.observedFor(method),
			}
		case <-ctx.Done():
			return Snapshot{}, ctx.Err()
		}
	}
}

func eventMatches(ev *Event, m *Matcher) bool {
	if m == nil {
		return true
	}
	for _, msg := range ev.Messages {
		if m.Match(msg) {
			return true
		}
	}
	return false
}

// WaitObserved blocks until an event for method exists, whether or not
// it has been consumed, without consuming it. It is used to confirm a
// handler-install stream (Split, Bar, Tab, Floating, Open) has opened
// so its handler can be rendered, and composes with expect_rpc and
// assert_no_unexpected_rpcs since it never marks the event consumed.
func (r *Recorder) WaitObserved(
	ctx context.Context, method string, timeout time.Duration,
) error {
	method = NormalizeMethod(method)
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		r.mu.Lock()
		found := slices.ContainsFunc(r.events, func(ev *Event) bool {
			return ev.Method == method
		})
		changed := r.changed
		r.mu.Unlock()
		if found {
			return nil
		}
		select {
		case <-changed:
		case <-deadline.C:
			return &MatchTimeoutError{Method: method, Observed: r.observedFor(method)}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (r *Recorder) observedFor(method string) []Snapshot {
	obs := make([]Snapshot, 0)
	for _, ev := range r.Snapshots() {
		if ev.Method == method && !ev.Consumed {
			obs = append(obs, ev)
		}
	}
	return obs
}

// WaitIdle blocks until no RPC activity has been recorded for d.
func (r *Recorder) WaitIdle(ctx context.Context, d time.Duration) error {
	for {
		r.mu.Lock()
		since := time.Since(r.lastChange)
		changed := r.changed
		r.mu.Unlock()
		if since >= d {
			return nil
		}
		wait := time.NewTimer(d - since)
		select {
		case <-changed:
			wait.Stop()
		case <-wait.C:
		case <-ctx.Done():
			wait.Stop()
			return ctx.Err()
		}
	}
}

// UnaryInterceptor records every unary RPC and serves scripted
// responses installed via AddResponder without invoking the real
// handler.
func (r *Recorder) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context, req any, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		var payload map[string]any
		if m, ok := req.(proto.Message); ok {
			payload = messageToMap(m)
		}
		ev := r.start(info.FullMethod, KindUnary, payload)
		if scripted := r.findResponse(ev.Method, payload); scripted != nil {
			resp, err := scripted.build(info.FullMethod)
			r.finish(ev, err)
			return resp, err
		}
		resp, err := handler(ctx, req)
		r.finish(ev, err)
		return resp, err
	}
}

// StreamInterceptor records stream openings and every client message.
func (r *Recorder) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ev := r.start(info.FullMethod, KindStream, nil)
		err := handler(srv, recordingStream{ServerStream: ss, r: r, ev: ev})
		r.finish(ev, err)
		return err
	}
}

type recordingStream struct {
	grpc.ServerStream
	r  *Recorder
	ev *Event
}

func (s recordingStream) RecvMsg(m any) error {
	err := s.ServerStream.RecvMsg(m)
	if err != nil {
		return err
	}
	if pm, ok := m.(proto.Message); ok {
		s.r.addMessage(s.ev, messageToMap(pm))
	}
	return err
}

func messageToMap(m proto.Message) map[string]any {
	data, err := payloadMarshal.Marshal(m)
	if err != nil {
		return map[string]any{"_marshal_error": err.Error()}
	}
	return jsonToMap(data)
}
