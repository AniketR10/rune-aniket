package proto

import (
	context "context"
	fmt "fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"
	"time"

	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/grpclog"
)

// ForceCloseResource is a helper function to remove a resource
// from a resource server or client and close it, safely.
func ForceCloseResource(
	broker MuxBroker, brokerID uint64, getResourcesFn func() map[uint64]io.Closer,
	locker sync.Locker,
) (io.Closer, error) {
	locker.Lock()
	defer locker.Unlock()
	resources := getResourcesFn()
	res, ok := resources[brokerID]
	if !ok {
		log.Debugf("resource %d already closed", brokerID)
		return nil, nil
	}
	delete(resources, brokerID)

	ret := res.Close()
	if ret != nil {
		log.Errorf("resource.Close %d error: %v", brokerID, ret)
	}

	if err := broker.Cleanup(uint32(brokerID)); err != nil {
		// add to Close error or ignore, if io.Closer
		// already cleans up underlying resources
		if ret != nil {
			ret = multierr.Append(ret, err)
		}
	}

	return res, ret
}

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
			conn.WaitForStateChange(ctx, state)
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

// DEPRECATED this leaks resources since the addition of Context to Serve.
// AcceptAndServe calls the underlying broker's AcceptAndServe
// with a new brokerID, and potentially an instrumented grpc.Server,
// if and only if the level enabled at logger is Trace.
func AcceptAndServe(
	broker MuxBroker,
	register func(uint32, MuxServer),
) (uint32, MuxServer, error) {
	brokerID := broker.NextId()
	lis, err := broker.Accept(brokerID)
	if err != nil {
		return 0, nil, fmt.Errorf("Accept: %w", err)
	}

	srv := GRPCServer()
	if log.IsLevelEnabled(log.TraceLevel) {
		srv = LoggingGRPCServer(srv)
	}

	register(brokerID, srv)

	go srv.Serve(context.Background(), lis)

	return brokerID, srv, nil
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
	lis, err := broker.NewChannel(tags...)
	if err != nil {
		return "", fmt.Errorf("new channel: %w", err)
	}

	srv := GRPCServer()
	if log.IsLevelEnabled(log.TraceLevel) {
		srv = LoggingGRPCServer(srv)
	}

	channelID := lis.Addr().String()

	register(channelID, srv)

	go srv.Serve(ctx, lis)

	return channelID, nil
}

// DisableGRPCLogging disables grpc stderr loggers.
func DisableGRPCLogging() {
	os.Setenv("GRPC_GO_LOG_SEVERITY_LEVEL", "FATAL")
	os.Setenv("GRPC_GO_LOG_VERBOSITY_LEVEL", "0")
	discard := grpclog.NewLoggerV2WithVerbosity(ioutil.Discard, ioutil.Discard, ioutil.Discard, 0)
	grpclog.SetLoggerV2(discard)
}
