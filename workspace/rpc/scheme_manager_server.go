package rpc

import (
	"context"
	"sync"
	"time"

	"github.com/ernestrc/blue/logging"
	log "github.com/sirupsen/logrus"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/config"

	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/workspace"
)

var (
	_ ManagerServer = (*SchemeManagerServer)(nil)
)

// SchemeManagerServer exposes a workspace.SchemeManager over the wire and
// satisfies SchemeServer grpc interface.
type SchemeManagerServer struct {
	UnimplementedManagerServer

	failureTimeout time.Duration
	broker         proto.MuxBroker

	// manager locker
	locker  sync.Locker
	manager workspace.SchemeManager
	schemes []string
}

// NewSchemeManagerServer allocates storage for a new SchemeManagerServer and initializes it
// with the given SchemeManager.
func NewSchemeManagerServer(
	broker proto.MuxBroker, manager workspace.SchemeManager,
	locker sync.Locker,
) *SchemeManagerServer {
	ret := new(SchemeManagerServer)
	ret.failureTimeout = defaultTimeout
	ret.Init(broker, manager, locker)
	return ret
}

// Init initializes this SchemeServerImpl with the given scheme.
func (s *SchemeManagerServer) Init(
	broker proto.MuxBroker, manager workspace.SchemeManager,
	locker sync.Locker,
) {
	s.manager = manager
	s.broker = broker
	s.locker = locker
}

func (c *SchemeManagerServer) log(level log.Level, msg string, args ...interface{}) {
	log.
		WithField(logging.KeyClass, "SchemeManagerServer").Logf(level, msg, args...)
}

func (s *SchemeManagerServer) dialScheme(proxyID uint32, cfg config.Config, uri workspaceapi.URI) (
	workspace.Scheme, error,
) {
	client, err := initializeSchemeThroughProxy(cfg, uri, s.broker, proxyID)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// Register satisfies ManagerServer.
func (s *SchemeManagerServer) RegisterScheme(ctx context.Context, req *RegisterSchemeRequest) (
	res *RegisterSchemeResponse, err error,
) {
	s.log(log.TraceLevel, "RegisterScheme: %d %s", req.GetProxyId(), req.GetScheme())
	defer s.log(log.TraceLevel, "RegisterScheme: %d %s: err=%s", req.GetProxyId(), req.GetScheme(), err)

	s.locker.Lock()
	defer s.locker.Unlock()

	proxyID := req.GetProxyId()
	err = s.manager.RegisterScheme(req.GetScheme(),
		func(_ context.Context, cfg config.Config, uri workspaceapi.URI) (
			workspace.Scheme, error,
		) {
			return s.dialScheme(proxyID, cfg, uri)
		})
	if err != nil {
		return
	}
	s.schemes = append(s.schemes, req.GetScheme())

	res = new(RegisterSchemeResponse)
	return
}

// Close closes all resources associated with this manager.
func (s *SchemeManagerServer) Close() (ret error) {
	/* TODO once plugin scheme is moved to api
	for _, scheme := range s.schemes {
		if err := s.manager.UnregisterScheme(scheme); err != nil {
			ret = multierr.Append(ret, err)
		}
	}*/
	return ret
}
