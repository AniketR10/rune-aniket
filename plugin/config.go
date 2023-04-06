package plugin

import (
	"io"
	"sync"

	"unstable.build/go-tui/config"
	configpb "unstable.build/go-tui/config/rpc"
	"unstable.build/go-tui/proto"
)

const (
	// PermissionConfig requests access to read the loaded configuration.
	PermissionConfig Permission = "_PermConfig"
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
	pluginID string, grantor Grantor, registrar proto.ServiceRegistrar,
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

func dialConfig(grant Grant, broker proto.MuxBroker) (
	config.Config, error,
) {
	conn, err := broker.DialChannel(grant.Token)
	if err != nil {
		return nil, err
	}
	c, err := configpb.FetchConfig(conn)
	if err != nil {
		return nil, err
	}
	return c, conn.Close()
}

// FetchConfig acquires the loaded config with the given permission token.
func FetchConfig(grant Grant, broker proto.MuxBroker) (
	config.Config, error,
) {
	return dialConfig(grant, broker)
}
