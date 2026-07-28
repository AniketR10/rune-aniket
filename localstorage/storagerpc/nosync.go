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
