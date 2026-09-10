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

package rpc

import (
	"context"

	grpc "google.golang.org/grpc"
	"unstable.build/rune/internal/debug"
)

// UnaryReportRecoveryInterceptor implements a grpc.UnaryServerInterceptor
// that recovers and logs panics.
func UnaryReportRecoveryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (ret any, err error) {
		debug.CapturePanicReport(func() {
			ret, err = handler(ctx, req)
		})
		return ret, err
	}
}

// StreamReportRecoveryInterceptor implements a grpc.UnaryServerInterceptor
// that recovers and logs panics.
func StreamReportRecoveryInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream,
		info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		debug.CapturePanicReport(func() {
			err = handler(srv, stream)
		})
		return err
	}
}
