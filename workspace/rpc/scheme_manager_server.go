package rpc

import (
	"context"
	"sync"
	"time"

	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	"unstable.build/go-tui/api/config"
	schemeapi "unstable.build/go-tui/api/scheme"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/workspace"

	"unstable.build/go-tui/proto"
)

var (
	_ ManagerServer = (*SchemeManagerServer)(nil)
)

// SchemeManagerServer exposes a schemeapi.SchemeManager over the wire and
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

func (s *SchemeManagerServer) dialScheme(
	ctx context.Context, proxyID string, cfg config.Config, uri workspaceapi.URI,
) (schemeapi.Scheme, error) {
	client, err := initializeSchemeThroughProxy(ctx, cfg, uri, s.broker, proxyID)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// Register satisfies ManagerServer.
func (s *SchemeManagerServer) RegisterScheme(ctx context.Context, req *RegisterSchemeRequest) (
	res *RegisterSchemeResponse, err error,
) {
	s.log(log.TraceLevel, "RegisterScheme: %s %s", req.GetProxyId(), req.GetScheme())
	defer s.log(log.TraceLevel, "RegisterScheme: %s %s: err=%s", req.GetProxyId(), req.GetScheme(), err)

	s.locker.Lock()
	defer s.locker.Unlock()

	proxyID := req.GetProxyId()
	err = s.manager.RegisterScheme(req.GetScheme(),
		func(ctx context.Context, cfg config.Config, uri workspaceapi.URI) (
			schemeapi.Scheme, error,
		) {
			return s.dialScheme(ctx, proxyID, cfg, uri)
		})
	if err != nil {
		if err == schemeapi.ErrSchemeAlreadyRegistered {
			err = status.Error(codes.AlreadyExists, "scheme already registered")
		}
		return
	}
	s.schemes = append(s.schemes, req.GetScheme())

	res = new(RegisterSchemeResponse)
	return
}

// Close closes all resources associated with this manager.
func (s *SchemeManagerServer) Close() (ret error) {
	for _, scheme := range s.schemes {
		if err := s.manager.UnregisterScheme(scheme); err != nil {
			ret = multierr.Append(ret, err)
		}
	}
	return ret
}
