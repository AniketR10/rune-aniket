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
	context "context"
	fmt "fmt"
	"io"

	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/grpclog"
)

// MonitorConnection blocks the calling goroutine and calls doClosed callback
// and returns only when connection state is shutdown, or it has been in a transient
// failure for too long.
func MonitorConnection(
	ctx context.Context, failureTimeout time.Duration,
	conn MuxConn, doClosed func(reason string),
) {

	for {
		state := conn.GetState()
		switch state {
		case connectivity.Idle, connectivity.Connecting, connectivity.Ready:
			if !conn.WaitForStateChange(ctx, state) {
				doClosed("context canceled")
				return
			}
		case connectivity.TransientFailure:
			failureCtx, cancelFn := context.WithTimeout(ctx, failureTimeout)
			didChange := conn.WaitForStateChange(failureCtx, connectivity.TransientFailure)
			cancelFn()
			if !didChange {
				doClosed("timeout waiting for transient failure to recover")
				return
			}
		case connectivity.Shutdown:
			doClosed("grpc connection state = shutdown")
			return
		default:
			panic(fmt.Sprintf("unknown connection state: %v", state))
		}
	}
}

// AcceptAndServeChannel calls the underlying broker's NewChannel
// and calls register before serving new connections. Use context
// to automatically stop underlying MuxServer when it's no longer needed.
func AcceptAndServeChannel(
	ctx context.Context,
	broker MuxBroker,
	register func(string, MuxServer),
	tags ...string,
) (string, error) {
	// add running program as tag
	tags = append(tags, filepath.Base(os.Args[0]))

	srv, err := broker.NewChannel(tags...)
	if err != nil {
		return "", fmt.Errorf("new channel: %w", err)
	}

	if log.IsLevelEnabled(log.TraceLevel) {
		srv = LoggingGRPCServer(srv)
	}

	channelID := srv.Addr().String()

	register(channelID, srv)

	go func() {
		if err := srv.Serve(ctx); err != nil {
			log.Errorf("serve channel(%v): %v", tags, err)
		}
	}()

	return channelID, nil
}

// DisableGRPCLogging disables grpc stderr loggers.
func DisableGRPCLogging() {
	os.Setenv("GRPC_GO_LOG_SEVERITY_LEVEL", "FATAL")
	os.Setenv("GRPC_GO_LOG_VERBOSITY_LEVEL", "0")
	discard := grpclog.NewLoggerV2WithVerbosity(io.Discard, io.Discard, io.Discard, 0)
	grpclog.SetLoggerV2(discard)
}

// EnableGRPCLogging disables grpc stderr loggers.
func EnableGRPCLogging(info, warn, err io.Writer) {
	os.Setenv("GRPC_GO_LOG_SEVERITY_LEVEL", "INFO")
	os.Setenv("GRPC_GO_LOG_VERBOSITY_LEVEL", "99")
	logger := grpclog.NewLoggerV2WithVerbosity(info, warn, err, 99)
	grpclog.SetLoggerV2(logger)
}
