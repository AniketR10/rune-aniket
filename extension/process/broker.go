package process

import (
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/retry"
	"unstable.build/go-tui/proto"
)

func initHostBroker(config managerConfig, dataDir, pkg, version string) (proto.MuxBroker, error) {
	broker := proto.NewUnixGRPCBroker(dataDir, pkg, version)
	broker = proto.WithRetryBroker(broker, retry.DefaultStrategy)
	if log.IsLevelEnabled(log.TraceLevel) {
		broker = proto.LoggingBroker(broker, log.StandardLogger())
	}
	return broker, nil
}

func initClientBroker(logger *log.Logger, dataDir, pkg, version string) proto.MuxBroker {
	broker := proto.NewUnixGRPCBroker(dataDir, pkg, version)
	broker = proto.WithRetryBroker(broker, retry.DefaultStrategy)
	if logger.IsLevelEnabled(log.TraceLevel) {
		broker = proto.LoggingBroker(broker, logger)
	}
	return broker
}
