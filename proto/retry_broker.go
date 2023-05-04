package proto

import (
	"context"
	"fmt"
	"strconv"

	"github.com/ernestrc/blue/logging"
	"github.com/ernestrc/blue/retry"
	log "github.com/sirupsen/logrus"
)

type retryBroker struct {
	b MuxBroker
	s retry.Strategy
}

// WithRetryBroker wraps a MuxBroker with retry.Strategy retries.
func WithRetryBroker(b MuxBroker, strategy retry.Strategy) MuxBroker {
	return retryBroker{b: b, s: strategy}
}

func (b retryBroker) log(method string, attempt int, err error) {
	log.WithFields(log.Fields{
		logging.KeyClass: "proto.retryBroker",
		"method":         method,
		"attempt":        strconv.Itoa(attempt),
		logging.KeyError: fmt.Sprintf("%v", err),
	}).Trace()
}

func (b retryBroker) NewChannel(tags ...string) (srv MuxServer, err error) {
	ctx := context.Background()
	var attempt int
	retry.Retry(ctx, b.s, func(ctx context.Context) (bool, error) {
		attempt++
		srv, err = b.b.NewChannel(tags...)
		b.log("NewChannel", attempt, err)
		return true, err
	})
	return
}

func (b retryBroker) DialChannel(addr string, tags ...string) (
	conn MuxConn, err error,
) {
	ctx := context.Background()
	var attempt int
	retry.Retry(ctx, b.s, func(ctx context.Context) (bool, error) {
		attempt++
		conn, err = b.b.DialChannel(addr, tags...)
		b.log("DialChannel", attempt, err)
		return true, err
	})
	return
}

func (b retryBroker) Close() error {
	return b.b.Close()
}
