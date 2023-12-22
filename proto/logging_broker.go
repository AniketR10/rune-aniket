package proto

import (
	context "context"

	log "github.com/sirupsen/logrus"
)

type loggingBroker struct {
	root   MuxBroker
	logger *log.Logger
}

// LoggingBroker wraps other to provide trace-level logging.
func LoggingBroker(other MuxBroker, logger *log.Logger) MuxBroker {
	return loggingBroker{root: other, logger: logger}
}

func (b loggingBroker) NewChannel(tags ...string) (srv MuxServer, err error) {
	srv, err = b.root.NewChannel(tags...)
	b.logger.Tracef("loggingBroker: NewChannel(%v): %v %v", tags, srv, err)
	return
}

func (b loggingBroker) DialChannel(ctx context.Context, addr string, tags ...string) (conn MuxConn, err error) {
	conn, err = b.root.DialChannel(ctx, addr, tags...)
	b.logger.Tracef("loggingBroker: DialChannel(%s, %v): %v %v",
		addr, tags, conn, err)
	return
}

func (b loggingBroker) Close() error {
	err := b.root.Close()
	b.logger.Tracef("loggingBroker: Close(): %v", err)
	return err
}
