package proto

import (
	context "context"
	"net"
	"time"

	log "github.com/sirupsen/logrus"
	grpc "google.golang.org/grpc"
)

type loggingServer struct {
	srv      MuxServer
	quitChan chan struct{}
}

// LoggingGRPCServer wraps a grpc.Server to provide trace-level logging.
func LoggingGRPCServer(srv MuxServer) MuxServer {
	ch := make(chan struct{})
	s := &loggingServer{srv: srv, quitChan: ch}
	// NOTE: uncomment to debug leaks
	// go s.monitorLifecycle()
	return s
}

func (s *loggingServer) monitorLifecycle() {
	log.Tracef("LoggingGRPCServer: Create: %p", s.srv)

	t := time.NewTicker(15 * time.Second)

	for {
		select {
		case <-s.quitChan:
			log.Tracef("LoggingGRPCServer: Monitor(quit): %p", s.srv)
			return
		case <-t.C:
			log.Tracef("LoggingGRPCServer: Monitor(alive): %p", s.srv)
		}
	}
}

func (s *loggingServer) Serve(ctx context.Context, lis net.Listener) error {
	log.Tracef("LoggingGRPCServer: (%p) Serve(Attempt, addr=%s) ", s.srv, lis.Addr().String())
	err := s.srv.Serve(ctx, lis)
	log.Tracef("LoggingGRPCServer: (%p) Serve(Result, addr=%s): %v", s.srv, lis.Addr().String(), err)
	return err
}

func (s *loggingServer) Stop() {
	s.srv.Stop()
	log.Tracef("LoggingGRPCServer: (%p) Stop() ", s.srv)
	close(s.quitChan)
}

func (s *loggingServer) Registrar() grpc.ServiceRegistrar {
	log.Tracef("LoggingGRPCServer: (%p) Registrar() ", s.srv)
	return s.srv.Registrar()
}

func (s *loggingServer) Addr() net.Addr {
	ret := s.srv.Addr()
	log.Tracef("LoggingGRPCServer: (%p) Addr(): %v ", s.srv, ret)
	return ret
}
