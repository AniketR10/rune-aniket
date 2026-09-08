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

package streamiterator

import (
	"context"
	"errors"
	"io"
	"sync/atomic"

	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/rune/internal/debug"
)

// Stream abstract a GRPC protoc-generated wrapper of grpc.ClientStream.
type Stream[T any] interface {
	// Recv blocks until it receives a message into m or the stream is
	// done. It returns io.EOF when the stream completes successfully. On
	// any other error, the stream is aborted and the error contains the RPC
	// status.
	Recv() (*T, error)
}

// RawStream abstract a subset of grpc.ClientStream.
type RawStream interface {
	// RecvMsg blocks until it receives a message into m or the stream is
	// done. It returns io.EOF when the stream completes successfully. On
	// any other error, the stream is aborted and the error contains the RPC
	// status.
	RecvMsg(m interface{}) error
}

// BidiStream abstract a GRPC a protoc-generated wrapper of
// grpc.ClientStream with bidirectional communication capabilities.
type BidiStream[T any, A any] interface {
	Stream[T]
	// Send sends a message of type A. See grpc.ClientStream.SendMsg for
	// more details.
	Send(*A) error
}

// ValueStream abstract an arbitrary value stream. See Stream for differences.
type ValueStream[T any] interface {
	// Recv blocks until it receives a message into m or the stream is
	// done. It returns io.EOF when the stream completes successfully. On
	// any other error, the stream is aborted and the error contains the RPC
	// status.
	Recv() (T, error)
}

// FromStream takes a Stream, tipically genereated via protoc, and
// returns an Iterator of T. The iterator stops when its Close function is returned or
// the underlying stream returns io.EOF or other error. Only non io.EOF errors
// will be surfaced via the returning iterator's Err method.
//
// The given cancel function should be the context canceling function fed to
// the grpc.ClientStream's constructor. This is to ensure that when the
// returned iterator's Close method is called, the stream's resources
// are also cleaned up.
func FromStream[T any](
	ctx context.Context, cancel func(), stream Stream[T],
) iterator.Iterator[*T] {
	return FromValueStream[*T](ctx, cancel, stream)
}

// FromStreamWithAck returns an iterator of T that sends back an ack message
// of type A for every chunk of T received. See FromRawStream for more details.
func FromStreamWithAck[T any, A any](
	ctx context.Context, cancel func(), stream BidiStream[T, A],
) iterator.Iterator[*T] {
	ack := new(A)
	return fromValueStreamWithAck[*T, BidiStream[T, A]](
		ctx, cancel, stream, func(stream BidiStream[T, A]) error {
			return stream.Send(ack)
		})
}

// FromRawStream works similar to FromStream, but takes a raw grpc.ClientStream,
// so it's inherently less safe than using FromStream.
func FromRawStream[T any](
	ctx context.Context, cancel func(), stream RawStream,
) iterator.Iterator[*T] {
	return FromStream[T](ctx, cancel, rawStreamAdapter[T]{stream})
}

// FromValueStream works similar to FromStream, but expects a ValueStream.
// See ValueStream for more details.
func FromValueStream[T any](
	ctx context.Context, cancel func(), stream ValueStream[T],
) iterator.Iterator[T] {
	return fromValueStreamWithAck[T, ValueStream[T]](
		ctx, cancel, stream, nil)
}

func fromValueStreamWithAck[T any, S ValueStream[T]](
	ctx context.Context, cancel func(), stream S,
	ack func(stream S) error,
) iterator.Iterator[T] {
	var closed atomic.Bool
	type msg struct {
		data T
		err  error
	}

	ch := make(chan msg)
	go debug.CapturePanicReport(func() {

		defer close(ch)
		for {
			data, err := stream.Recv()
			select {
			case ch <- msg{data: data, err: err}:
				if err != nil {
					return
				}
				if ack == nil {
					continue
				}
				err := ack(stream)
				if err != nil {
					select {
					case ch <- msg{err: err}:
					case <-ctx.Done():
					}
					return
				}
			case <-ctx.Done():
				return
			}
		}

	})

	return iterator.FromFunc(func(ctx context.Context) (ret T, ok bool, err error) {
		var m msg
		select {
		case m, ok = <-ch:
			ret = m.data
			err = m.err
			if err != nil {
				ok = false
			}
			if errors.Is(err, io.EOF) {
				err = nil
			}
			return
		case <-ctx.Done():
			err = ctx.Err()
			return
		}
	}, func() error {
		if !closed.CompareAndSwap(false, true) {
			return nil
		}
		cancel()
		<-ch // wait for clean goroutine to be done
		return nil
	})
}

type rawStreamAdapter[T any] struct {
	s RawStream
}

func (r rawStreamAdapter[T]) Recv() (*T, error) {
	ret := new(T)
	err := r.s.RecvMsg(ret)
	return ret, err
}
