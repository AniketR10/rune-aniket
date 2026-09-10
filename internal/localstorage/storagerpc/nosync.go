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

package storagerpc

import (
	"context"

	bluebolt "github.com/unstablebuild/blue/document/bolt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// NoSyncMetadataKey carries the bluebolt relaxed-durability request across
// process boundaries as gRPC metadata. See bluebolt.ContextWithNoSync.
const NoSyncMetadataKey = "x-rune-storage-nosync"

// NoSyncDialOptions returns dial options that forward a bluebolt no-sync
// request carried by the per-call context as outgoing gRPC metadata, so a
// remote Server can restore the request before it reaches its bolt-backed
// store. The bluebolt context value does not cross process boundaries on
// its own.
func NoSyncDialOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithChainUnaryInterceptor(noSyncUnaryClientInterceptor),
		grpc.WithChainStreamInterceptor(noSyncStreamClientInterceptor),
	}
}

func noSyncUnaryClientInterceptor(
	ctx context.Context, method string, req, reply any,
	cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
) error {
	return invoker(noSyncOutgoingContext(ctx), method, req, reply, cc, opts...)
}

func noSyncStreamClientInterceptor(
	ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn,
	method string, streamer grpc.Streamer, opts ...grpc.CallOption,
) (grpc.ClientStream, error) {
	return streamer(noSyncOutgoingContext(ctx), desc, cc, method, opts...)
}

func noSyncOutgoingContext(ctx context.Context) context.Context {
	if !bluebolt.NoSyncRequested(ctx) {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, NoSyncMetadataKey, "1")
}

// noSyncIncomingContext restores a forwarded no-sync request from incoming
// gRPC metadata onto the handler context.
func noSyncIncomingContext(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok || len(md.Get(NoSyncMetadataKey)) == 0 {
		return ctx
	}
	return bluebolt.ContextWithNoSync(ctx)
}
