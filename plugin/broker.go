package plugin

import (
	"fmt"
	"net"
	"os"

	"github.com/ernestrc/blue/datastore/document"
	"github.com/ernestrc/blue/retry"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/util"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

const (
	envRemoteBrokerAddr = "go_tui_remote_broker_addr"
)

func makeBrokerRemoteAddrEnv(addr string) string {
	return fmt.Sprintf("%s=%s", envRemoteBrokerAddr, addr)
}

func getBrokerRemoteAddrEnv() net.Addr {
	addrStr := os.Getenv(envRemoteBrokerAddr)
	addr, err := net.ResolveUnixAddr("unix", addrStr)
	if err != nil {
		panic(fmt.Errorf("failed to resolve remote broker address: %v", err))
	}
	return addr
}

func initHostBroker(
	config managerConfig, svc document.Service, srv *document.Server,
) (proto.MuxBroker, net.Addr, error) {
	lis, err := util.TempUnixListener()
	if err != nil {
		return nil, nil, err
	}

	go srv.Serve(lis)

	broker := proto.NewDatastoreBroker(svc)
	broker = proto.WithRetryBroker(broker, retry.DefaultStrategy)
	return broker, lis.Addr(), nil
}

func initClientBroker(logger *log.Logger) proto.MuxBroker {
	remoteAddr := getBrokerRemoteAddrEnv()
	store, err := document.NewClient(remoteAddr, grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	broker := proto.NewDatastoreBroker(store)
	broker = proto.WithRetryBroker(broker, retry.DefaultStrategy)
	if logger.IsLevelEnabled(log.TraceLevel) {
		broker = proto.LoggingBroker(broker, logger)
	}
	return broker
}
