package proto

import (
	context "context"
	fmt "fmt"
	"io"
	"io/ioutil"
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

// EnableGRPCLogging disables grpc stderr loggers.
func EnableGRPCLogging(info, warn, err io.Writer) {
	os.Setenv("GRPC_GO_LOG_SEVERITY_LEVEL", "INFO")
	os.Setenv("GRPC_GO_LOG_VERBOSITY_LEVEL", "99")
	logger := grpclog.NewLoggerV2WithVerbosity(info, warn, err, 99)
	grpclog.SetLoggerV2(logger)
}
