package rpc

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/ernestrc/blue/logging"
	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/debug"
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
	mu             sync.Mutex
	manager        workspace.SchemeManager
	broker         proto.MuxBroker
	clients        map[uint64]io.Closer
}

// ties together all resources into an io.Closer
type schemeManagerResource struct {
	conn          proto.MuxConn
	client        workspace.Scheme
	cancelMonitor func()
}

func (s *schemeManagerResource) Close() (ret error) {
	if s.cancelMonitor == nil {
		return nil
	}
	s.cancelMonitor()
	s.cancelMonitor = nil
	if err := s.client.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	if err := s.conn.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	return ret
}

// NewSchemeManagerServer allocates storage for a new SchemeManagerServer and initializes it
// with the given SchemeManager.
func NewSchemeManagerServer(broker proto.MuxBroker, manager workspace.SchemeManager) *SchemeManagerServer {
	ret := new(SchemeManagerServer)
	ret.failureTimeout = defaultTimeout
	ret.Init(broker, manager)
	return ret
}

// Init initializes this SchemeServerImpl with the given scheme.
func (s *SchemeManagerServer) Init(broker proto.MuxBroker, manager workspace.SchemeManager) {
	s.manager = manager
	s.broker = broker
	s.clients = make(map[uint64]io.Closer)
}

func (s *SchemeManagerServer) getClients() map[uint64]io.Closer {
	return s.clients
}

func (s *SchemeManagerServer) safeForceCloseHandler(brokerID uint64, reason string) error {
	_, err := proto.ForceCloseResource(s.broker, brokerID, s.getClients, &s.mu)
	return err
}

func (c *SchemeManagerServer) log(level log.Level, msg string, args ...interface{}) {
	debug.StandardLogger().
		WithField(logging.KeyClass, "SchemeManagerServer").Logf(level, msg, args...)
}

func (s *SchemeManagerServer) dialScheme(proxyID uint32, cfg config.Config, uri workspace.URI) (
	workspace.Scheme, error,
) {
	client, conn, err := initializeSchemeThroughProxy(cfg, uri, s.broker, proxyID)
	if err != nil {
		return nil, err
	}

	ctx, cancelFn := context.WithCancel(context.Background())
	go proto.MonitorConnection(ctx, s.failureTimeout, conn,
		func(reason string) {
			s.safeForceCloseHandler(uint64(proxyID),
				fmt.Sprintf("rpc.SchemeManagerServer(proxyID=%d): %s", proxyID, reason))
		})

	s.mu.Lock()
	defer s.mu.Unlock()

	s.clients[uint64(proxyID)] = &schemeManagerResource{
		conn:          conn,
		client:        client,
		cancelMonitor: cancelFn,
	}

	return client, nil
}

// Register satisfies ManagerServer.
func (s *SchemeManagerServer) RegisterScheme(ctx context.Context, req *RegisterSchemeRequest) (
	res *RegisterSchemeResponse, err error,
) {
	s.log(log.TraceLevel, "RegisterScheme: %d %s", req.GetProxyId(), req.GetScheme())
	defer s.log(log.TraceLevel, "RegisterScheme: %d %s: err=%s", req.GetProxyId(), req.GetScheme(), err)

	s.mu.Lock()
	defer s.mu.Unlock()

	proxyID := req.GetProxyId()
	err = s.manager.RegisterScheme(req.GetScheme(),
		func(cfg config.Config, uri workspace.URI) (workspace.Scheme, error) {
			return s.dialScheme(proxyID, cfg, uri)
		})
	if err != nil {
		return
	}

	res = new(RegisterSchemeResponse)
	return
}
