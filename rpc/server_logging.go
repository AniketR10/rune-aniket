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
	"net"
	"time"

	log "github.com/sirupsen/logrus"
)

type loggingServer struct {
	srv    MuxServer
	ctx    context.Context
	cancel func()
}

// LoggingGRPCServer wraps a grpc.Server to provide trace-level logging.
func LoggingGRPCServer(srv MuxServer) MuxServer {
	// ch := make(chan struct{})
	s := &loggingServer{srv: srv}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	// NOTE: uncomment to debug leaks
	// go s.monitorLifecycle()
	return s
}

// nolint:unused
func (s *loggingServer) monitorLifecycle() {
	log.Tracef("LoggingGRPCServer: Create: %p", s.srv)

	t := time.NewTicker(15 * time.Second)

	for {
		select {
		case <-s.ctx.Done():
			log.Tracef("LoggingGRPCServer: Monitor(quit): %p", s.srv)
			return
		case <-t.C:
			log.Tracef("LoggingGRPCServer: Monitor(alive): %p", s.srv)
		}
	}
}

func (s *loggingServer) Serve(ctx context.Context) error {
	log.Tracef("LoggingGRPCServer: (%p) Serve(Attempt, addr=%s) ", s.srv, s.Addr().String())
	err := s.srv.Serve(ctx)
	log.Tracef("LoggingGRPCServer: (%p) Serve(Result, addr=%s): %v", s.srv, s.Addr().String(), err)
	return err
}

func (s *loggingServer) Stop() {
	s.srv.Stop()
	log.Tracef("LoggingGRPCServer: (%p) Stop() ", s.srv)
	s.cancel()
}

func (s *loggingServer) GracefulStop() {
	s.srv.GracefulStop()
	log.Tracef("LoggingGRPCServer: (%p) GracefulStop() ", s.srv)
	s.cancel()
}

func (s *loggingServer) Registrar() ServiceRegistrar {
	log.Tracef("LoggingGRPCServer: (%p) Registrar() ", s.srv)
	return s.srv.Registrar()
}

func (s *loggingServer) Addr() net.Addr {
	ret := s.srv.Addr()
	log.Tracef("LoggingGRPCServer: (%p) Addr(): %v ", s.srv, ret)
	return ret
}
