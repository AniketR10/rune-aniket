package process

import (
	"github.com/ernestrc/blue/retry"
	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/proto"
)

func initHostBroker(config managerConfig, dataDir string) (proto.MuxBroker, error) {
	broker := proto.NewUnixGRPCBroker(dataDir)
	broker = proto.WithRetryBroker(broker, retry.DefaultStrategy)
	if log.IsLevelEnabled(log.TraceLevel) {
		broker = proto.LoggingBroker(broker, log.StandardLogger())
	}
	return broker, nil
}

func initClientBroker(logger *log.Logger, dataDir string) proto.MuxBroker {
	broker := proto.NewUnixGRPCBroker(dataDir)
	broker = proto.WithRetryBroker(broker, retry.DefaultStrategy)
	if logger.IsLevelEnabled(log.TraceLevel) {
		broker = proto.LoggingBroker(broker, logger)
	}
	return broker
}
