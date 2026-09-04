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

package rpc

import (
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/blue/logging/trace"
	grpc "google.golang.org/grpc"
)

// UnaryLoggingInterceptor implements a grpc.UnaryServerInterceptor that logs.
func UnaryLoggingInterceptor(fields []logging.Field) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (ret any, err error) {
		traceID, ctx := trace.FromContextOrNew(ctx)
		attemptAt := logging.LogAttempt(traceID, info.FullMethod, fields...)
		ret, err = handler(ctx, req)
		logging.LogResultLevel(log.TraceLevel, log.ErrorLevel, err, attemptAt,
			traceID, info.FullMethod, fields...)
		return
	}
}

// StreamLoggingInterceptor implements a grpc.UnaryServerInterceptor that logs.
func StreamLoggingInterceptor(fields []logging.Field) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream,
		info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		traceID, _ := trace.FromContextOrNew(stream.Context())
		attemptAt := logging.LogAttempt(traceID, info.FullMethod, fields...)
		err = handler(srv, stream)
		logging.LogResultLevel(log.TraceLevel, log.ErrorLevel, err, attemptAt,
			traceID, info.FullMethod, fields...)
		return
	}
}
