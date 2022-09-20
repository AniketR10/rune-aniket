package plugin

import (
	"sync"

	configpb "github.com/ernestrc/go-tui/plugin/rpc"
	"github.com/ernestrc/go-tui/proto"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

const (
	// PermissionConfig requests access to read the loaded configuration.
	PermissionConfig Permission = "_PermConfig"
)

type configResourceServer struct {
	mu  sync.Mutex
	cfg Config
	srv proto.MuxServer
}

func newConfigResourceServer(cfg Config) *configResourceServer {
	ret := new(configResourceServer)
	ret.cfg = cfg
	return ret
}

func (s *configResourceServer) Serve(
	pluginID string, grantID uint32, broker proto.MuxBroker,
	l *log.Logger, lock sync.Locker,
) error {
	return acceptAndServe(broker, grantID,
		func(opts []grpc.ServerOption) proto.MuxServer {
			s.mu.Lock()
			defer s.mu.Unlock()
			if s.srv == nil {
				var srv proto.MuxServer
				if l != nil && l.IsLevelEnabled(log.TraceLevel) {
					srv = proto.LoggingGRPCServer(l, opts...)
				} else {
					srv = proto.GRPCServer(opts...)
				}
				grpc := srv.GRPC()
				s.srv = srv
				server := newConfigServer(s.cfg)
				configpb.RegisterConfigServer(grpc, server)
			}
			return s.srv
		})
}

// ConfigResources returns a map of Permission to a ResourceServer
// capable of serving each of the b Config's resources.
func ConfigResources(b Config) map[Permission]ResourceServer {
	s := newConfigResourceServer(b)
	return map[Permission]ResourceServer{
		PermissionConfig: s,
	}
}

func dialConfig(token uint32, broker proto.MuxBroker) (
	Config, error,
) {
	if c, ok := clients.Load(token); ok {
		return c.(Config), nil
	}
	conn, err := broker.Dial(token)
	if err != nil {
		return nil, err
	}
	c, err := newConfigFromServer(conn)
	if err != nil {
		return nil, err
	}
	clients.Store(token, c)
	return c, nil
}

// FetchConfig acquires the loaded config with the given permission token.
func FetchConfig(token uint32, broker proto.MuxBroker) (
	Config, error,
) {
	return dialConfig(token, broker)
}
