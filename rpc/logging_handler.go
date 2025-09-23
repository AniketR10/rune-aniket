// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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
