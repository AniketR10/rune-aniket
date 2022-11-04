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
	broker         proto.MuxBroker

	// manager locker
	locker  sync.Locker
	manager workspace.SchemeManager

	// resources locker; needed because we AddWorkspace
	// is called from the main goroutine, which calls dialScheme
	// and read/writes clients, at the same time we run a goroutine
	// to monitor the underlying connection, which also writes
	// to clients
	rmu     sync.Mutex
	clients map[uint64]io.Closer
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
	s.clients = make(map[uint64]io.Closer)
}

func (s *SchemeManagerServer) getClients() map[uint64]io.Closer {
	return s.clients
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
			reason = fmt.Sprintf("rpc.SchemeManagerServer(proxyID=%d): %s", proxyID, reason)
			_, _ = proto.ForceCloseResource(s.broker, uint64(proxyID), s.getClients, &s.rmu)
		})

	s.rmu.Lock()
	defer s.rmu.Unlock()

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

	s.locker.Lock()
	defer s.locker.Unlock()

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

// Close closes all resources associated with this manager.
func (s *SchemeManagerServer) Close() (ret error) {
	for _, client := range s.clients {
		if err := client.Close(); err != nil {
			ret = multierr.Append(ret, err)
		}
	}
	return ret
}
