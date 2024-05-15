package process

import (
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/retry"
	"unstable.build/go-tui/rpc"
)

func initHostBroker(config managerConfig, dataDir, pkg, version string) (rpc.MuxBroker, error) {
	broker := rpc.NewUnixGRPCBroker(dataDir, pkg, version)
	broker = rpc.WithRetryBroker(broker, retry.DefaultStrategy)
	if log.IsLevelEnabled(log.TraceLevel) {
		broker = rpc.LoggingBroker(broker, log.StandardLogger())
	}
	return broker, nil
}

func initClientBroker(logger *log.Logger, dataDir, pkg, version string) rpc.MuxBroker {
	broker := rpc.NewUnixGRPCBroker(dataDir, pkg, version)
	broker = rpc.WithRetryBroker(broker, retry.DefaultStrategy)
	if logger.IsLevelEnabled(log.TraceLevel) {
		broker = rpc.LoggingBroker(broker, logger)
	}
	return broker
}
