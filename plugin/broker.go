package plugin

import (
	"fmt"
	"net"
	"os"

	"github.com/ernestrc/blue/document"
	docrpc "github.com/ernestrc/blue/document/rpc"
	"github.com/ernestrc/blue/encoding/bson"
	"github.com/ernestrc/blue/retry"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/util"
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
	config managerConfig, svc document.Service, srv *docrpc.Server,
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
	store, err := docrpc.NewClient(remoteAddr, bson.Marshaler(), grpc.WithInsecure())
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
