package proto

import (
	"fmt"

	"github.com/ernestrc/blue/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	log "github.com/sirupsen/logrus"
	grpc "google.golang.org/grpc"
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
