package proto

import (
	"net"

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

func (b loggingBroker) NewChannel(tags ...string) (lis net.Listener, err error) {
	lis, err = b.root.NewChannel(tags...)
	b.logger.Tracef("loggingBroker: NewChannel(%v): %v %v", tags, lis, err)
	return
}

func (b loggingBroker) DialChannel(addr string, tags ...string) (conn MuxConn, err error) {
	conn, err = b.root.DialChannel(addr, tags...)
	b.logger.Tracef("loggingBroker: DialChannel(%s, %v): %v %v",
		addr, tags, conn, err)
	return
}

func (b loggingBroker) Close() error {
	err := b.root.Close()
	b.logger.Tracef("loggingBroker: Close(): %v", err)
	return err
}
