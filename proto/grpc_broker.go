package proto

import (
	context "context"
	fmt "fmt"
	"net"
	"os"
	"sync"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"unstable.build/go-tui/util"
)

// implements MuxBroker
type grpcBroker struct {
	dataDir     string
	unixSockets sync.Map
}

// NewUnixGRPCBroker provides brokerage by using unix socket listeners
// and grpc connections.
func NewUnixGRPCBroker(dataDir string) MuxBroker {
	ret := new(grpcBroker)
	ret.dataDir = dataDir
	return ret
}

func (t *grpcBroker) NewChannel(tags ...string) (net.Listener, error) {
	ret, err := util.TempUnixListenerTags(t.dataDir, tags...)
	if err != nil {
		return nil, fmt.Errorf("temp unix listener")
	}
	t.unixSockets.Store(ret.Addr().String(), struct{}{})
	return ret, nil
}

func (t *grpcBroker) DialChannel(address string, tags ...string) (
	conn MuxConn, err error,
) {
	opts := []grpc.DialOption{
		grpc.WithStatsHandler(nil),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(
			func(ctx context.Context, _ string) (net.Conn, error) {
				addr, err := net.ResolveUnixAddr("unix", address)
				if err != nil {
					return nil, err
				}
				var d net.Dialer
				d.Deadline, _ = ctx.Deadline()
				return d.Dial(addr.Network(), addr.String())
			},
		)}
	conn, err = grpc.Dial("", opts...)
	if err != nil {
		return
	}

	if log.IsLevelEnabled(log.TraceLevel) {
		conn = newLoggingConn(address, conn, tags...)
	}
	return
}

func (t *grpcBroker) Close() (ret error) {
	t.unixSockets.Range(func(key, value any) bool {
		// might be redundant if clients clean up correctly
		_ = os.Remove(key.(string))
		return true
	})
	return nil
}
