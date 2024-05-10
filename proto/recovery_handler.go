package proto

import (
	"context"
	"fmt"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	grpc "google.golang.org/grpc"
	"unstable.build/go-tui/debug"
)

// UnaryLoggingRecoveryHandler implements a grpc.UnaryServerInterceptor
// that recovers and logs panics.
func UnaryLoggingRecoveryInterceptor(tags ...string) grpc.UnaryServerInterceptor {
	return recovery.UnaryServerInterceptor(logOption(tags...))
}

// StreamLoggingRecoveryHandler implements a grpc.UnaryServerInterceptor
// that recovers and logs panics.
func StreamLoggingRecoveryInterceptor(tags ...string) grpc.StreamServerInterceptor {
	return recovery.StreamServerInterceptor(logOption(tags...))
}

// UnaryReportRecoveryHandler implements a grpc.UnaryServerInterceptor
// that recovers and logs panics.
func UnaryReportRecoveryInterceptor(dir, pkg, version string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (ret any, err error) {
		ok, reportname, captureErr := debug.CapturePanicReportDir(dir, pkg, version, func() {
			ret, err = handler(ctx, req)
		})
		if ok {
			return ret, err
		}
		if captureErr != nil {
			panic(fmt.Sprintf("capture panic report: error capturing: %v", captureErr))
		}
		panic(fmt.Sprintf("grpc goroutine panic: report: %s", reportname))
	}
}

// StreamReportRecoveryHandler implements a grpc.UnaryServerInterceptor
// that recovers and logs panics.
func StreamReportRecoveryInterceptor(dir, pkg, version string) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream,
		info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		ok, reportname, captureErr := debug.CapturePanicReportDir(dir, pkg, version, func() {
			err = handler(srv, stream)
		})
		if ok {
			return err
		}
		if captureErr != nil {
			panic(fmt.Sprintf("capture panic report: error capturing: %v", captureErr))
		}
		panic(fmt.Sprintf("grpc goroutine panic: report: %s", reportname))
	}
}

func logOption(tags ...string) recovery.Option {
	return recovery.WithRecoveryHandler(
		func(p any) error {
			log.WithFields(log.Fields{
				"Tags":           fmt.Sprintf("%+v", tags),
				logging.KeyError: fmt.Sprintf("%+v", p),
			}).Panicf("grpc goroutine panic")
			return nil
		})
}
