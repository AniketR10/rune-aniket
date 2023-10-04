package extension

import (
	"io"
	"sync"

	"unstable.build/go-tui/api/config"
	configpb "unstable.build/go-tui/api/config/rpc"
	"unstable.build/go-tui/proto"
)

type configResourceServer struct {
	cfg config.Config
}

func newConfigResourceServer(cfg config.Config) *configResourceServer {
	ret := new(configResourceServer)
	ret.cfg = cfg
	return ret
}

func (s *configResourceServer) Register(
	extensionID string, grantor Grantor, registrar proto.ServiceRegistrar,
	broker proto.MuxBroker, lock sync.Locker,
) (io.Closer, error) {
	server := configpb.NewServer(s.cfg, lock)
	configpb.RegisterConfigServer(registrar, server)
	return nopCloser{}, nil
}

// ConfigResources returns a map of Permission to a ResourceServer
// capable of serving each of the b Config's resources.
func ConfigResources(b config.Config) map[Permission]ResourceRegistrar {
	s := newConfigResourceServer(b)
	return map[Permission]ResourceRegistrar{
		PermissionConfig: s,
	}
}

type nopCloser struct {
}

func (c nopCloser) Close() error {
	return nil
}
