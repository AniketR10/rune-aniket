package plugin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/ernestrc/blue/logging"
	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/config"
	pluginpb "unstable.build/go-tui/plugin/rpc"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/util"
)

const (
	defaultHandshakeTimeout  = 5 * time.Second
	defaultHealthCheckTicker = 15 * time.Second
	defaultHealthRetries     = 2
	defaultRunRetries        = 2
)

var defaultManagerConfig = managerConfig{
	handshakeTimeout:  defaultHandshakeTimeout,
	healthCheckTicker: defaultHealthCheckTicker,
	healthRetries:     defaultHealthRetries,
	locker:            new(sync.Mutex),
	dataDir:           os.TempDir(),
}

type granteeClientWrap struct {
	id        string
	path      string
	running   bool
	activeAt  time.Time
	errors    []error
	doneCh    chan *sync.WaitGroup
	client    *granteeClient
	resources []io.Closer
	lis       net.Listener
	ctx       context.Context
	cancelCtx func()
}

// Stat represents the status of a Plugin.
type Stat struct {
	ActiveAt time.Time
	Errors   []error
}

type managerConfig struct {
	handshakeTimeout  time.Duration
	healthCheckTicker time.Duration
	healthRetries     int
	locker            sync.Locker
	workspace         workspaceapi.URI
	dataDir           string
}

// Option is a configuration option for a manager.
type Option func(cfg *managerConfig)

type pluginBuilder func(pluginID, path string,
	grantor Grantor) (*granteeClient, error)

// Manager manages the lifecycle of plugins.
type Manager struct {
	mu      sync.Mutex
	grantor Grantor
	clients map[string]*granteeClientWrap

	// resource mutex used to synchronize term event loop with
	// access to resources by plugins.
	rmu sync.Locker

	// used to abstract out go-plugin specific functionality
	builder pluginBuilder

	broker proto.MuxBroker
	config managerConfig
}

// NewManager allocates storage for a new Manager and initializes it.
func NewManager(grantor Grantor, opts ...Option) (*Manager, error) {
	ret := new(Manager)
	err := ret.Init(grantor, opts...)
	ret.builder = goPluginGranteeBuilder(ret, ret.config.dataDir)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// Init initializes this manager with grantor.
func (m *Manager) Init(grantor Grantor, opts ...Option) (err error) {
	// enable registrants to inject other registrants as dependencies
	// where passing same instance is important due to the stateful nature
	// of some resource servers.
	m.grantor = cachingGrantor(grantor)
	m.clients = make(map[string]*granteeClientWrap)

	m.config = defaultManagerConfig
	for _, o := range opts {
		o(&m.config)
	}
	m.rmu = m.config.locker

	m.broker, err = initHostBroker(m.config, m.config.dataDir)
	return
}

func (m *Manager) log(level log.Level, msg string, args ...interface{}) {
	log.
		WithField(logging.KeyClass, "plugin.Manager").Logf(level, msg, args...)
}

func (m *Manager) runPlugin(pluginID, path string, config config.Config) error {
	client, err := m.builder(pluginID, path, m.grantor)
	if err != nil {
		return err
	}

	m.log(log.InfoLevel, "Running plugin %q at path %q", pluginID, path)

	clientWrap := &granteeClientWrap{
		id:      pluginID,
		path:    path,
		running: false,
		doneCh:  make(chan *sync.WaitGroup),
		client:  client,
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.clients[pluginID] = clientWrap

	go m.handshake(pluginID, clientWrap, config)

	return nil
}

// Run runs the plugin at path with identifier pluginID. It returns an error
// if something went wrong when finding and executing the plugin executable.
// Once the plugin is up and running, errors can be retrieved with Stat.
// Note that config is an optional argument.
func (m *Manager) Run(pluginID, path string, config config.Config) error {
	m.mu.Lock()
	_, ok := m.clients[pluginID]
	m.mu.Unlock()
	if ok {
		return fmt.Errorf("Manager: already connected plugin with id: '%s'", pluginID)
	}
	return m.runPlugin(pluginID, path, config)
}

func (m *Manager) doGrant(
	ctx context.Context,
	pluginID string,
	client *granteeClientWrap,
	perms []*pluginpb.Permission,
) error {
	lis, err := m.broker.NewChannel(pluginID)
	if err != nil {
		return fmt.Errorf("accept: %v", err)
	}
	opts := []grpc.ServerOption{}

	srv := proto.GRPCServer(opts...)
	if log.IsLevelEnabled(log.TraceLevel) {
		srv = proto.LoggingGRPCServer(srv)
	}

	// for denied permissions we do not call ResourceRegistrar.Register so
	// if a malicious or otherwise client attempts to get a resource that was not granted
	// they'll receive an unimplemented status code.
	var denied []*pluginpb.Permission
	var serverResources []io.Closer
	granted := make(map[string]*pluginpb.PermissionGrant)
	for _, p := range perms {
		permissionID := util.SanitizeLine(p.GetId())
		if _, granted := granted[permissionID]; granted {
			continue
		}

		registrar, ok := m.grantor.Grant(pluginID, Permission(permissionID))
		if !ok {
			denied = append(denied, p)
			continue
		}

		resource, err := registrar.Register(pluginID, m.grantor,
			srv.Registrar(), m.broker, m.rmu)
		if err != nil {
			log.Errorf("Could not register %s plugin %q: %v", permissionID, pluginID, err)
			denied = append(denied, p) // at least communicate to plugin
			continue
		}

		granted[permissionID] = &pluginpb.PermissionGrant{
			Id: permissionID,
			// TODO pass temporary token that can be used
			// by client and server auth middleware
			Address: lis.Addr().String(),
		}

		serverResources = append(serverResources, resource)
	}

	m.mu.Lock()
	client.resources = serverResources
	client.ctx, client.cancelCtx = context.WithCancel(context.Background())
	ctx = client.ctx
	m.mu.Unlock()

	go srv.Serve(ctx, lis)

	return client.client.sendGrants(ctx, denied, granted)
}

func (m *Manager) doCloseClient(reason string, client *granteeClientWrap) (
	ret chan *sync.WaitGroup,
) {
	m.mu.Lock()
	if client.doneCh == nil {
		m.mu.Unlock()
		return nil
	}
	ret = client.doneCh
	client.doneCh = nil
	m.mu.Unlock()

	var clientErr error
	if err := client.client.shutdown(reason); err != nil {
		clientErr = multierr.Append(clientErr, err)
	}

	m.mu.Lock()
	if client.cancelCtx != nil {
		client.cancelCtx()
		client.cancelCtx = nil
	}
	resources := client.resources
	client.resources = nil
	m.mu.Unlock()

	for _, resource := range resources {
		if err := resource.Close(); err != nil {
			clientErr = multierr.Append(clientErr, err)
		}
	}

	level := log.InfoLevel
	if clientErr != nil {
		level = log.ErrorLevel
	}
	m.log(level, "stopped plugin with id '%s': err=%v", client.id, clientErr)
	return
}

func (m *Manager) checkHealth(ctx context.Context, client *granteeClientWrap) (
	err error,
) {
	ctx, closeFn := context.WithTimeout(ctx, m.config.healthCheckTicker)
	defer closeFn()

	waitCh := make(chan error)
	go func() {
		select {
		case waitCh <- client.client.health(ctx):
		case <-ctx.Done():
		}
	}()

	select {
	case <-ctx.Done():
		err = ctx.Err()
	case err = <-waitCh:
	}
	m.log(log.TraceLevel, "health check on plugin '%s': err=%v", client.id, err)
	return err
}

func (m *Manager) monitor(client *granteeClientWrap) {

	m.log(log.DebugLevel, "checking health of plugin '%s' every %+v",
		client.id, m.config.healthCheckTicker)

	timer := time.NewTicker(m.config.healthCheckTicker)

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	triesLeft := 1 + m.config.healthRetries
	for {
		select {
		case <-timer.C:
			err := m.checkHealth(ctx, client)
			if err != nil {
				triesLeft--
				m.addClientErr(client.id, err)
			} else {
				triesLeft = 1 + m.config.healthRetries
			}
			if triesLeft == 0 {
				doneCh := m.doCloseClient("exhausted health check retries", client)
				if doneCh != nil {
					// if someone grabs doneCh while we're doing a health check
					// make sure we notify receiver
					select {
					case wg := <-doneCh:
						wg.Done()
					default:
					}
				}
			}
		case wg := <-client.doneCh:
			defer wg.Done()
			m.doCloseClient("Stop/Close was called on plugin Manager", client)
			return
		}
	}
}

func (m *Manager) addClientErr(pluginID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	clientWrap := m.clients[pluginID]
	clientWrap.errors = append(clientWrap.errors, err)

	m.log(log.ErrorLevel, "plugin '%s' error: %s", pluginID, err)
}

func (m *Manager) setRunning(pluginID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	client := m.clients[pluginID]
	client.running = true
	client.activeAt = time.Now()
}

func (m *Manager) handshake(
	pluginID string, client *granteeClientWrap, config config.Config,
) {
	ctx := context.Background()
	ctx, closeFn := context.WithTimeout(ctx, m.config.handshakeTimeout)
	defer closeFn()

	m.log(log.TraceLevel, "starting handshake for plugin '%s'", pluginID)

	doneCh := make(chan error)
	go func() {

		perms, err := client.client.permissions(ctx, config)
		if err != nil {
			select {
			case doneCh <- err:
			default:
			}
			return
		}

		err = m.doGrant(ctx, pluginID, client, perms)
		if err != nil {
			select {
			case doneCh <- err:
			default:
			}
			return
		}

		close(doneCh)
	}()

	var err error

	select {
	case err = <-doneCh:
	case <-ctx.Done():
		err = ctx.Err()
	}

	if err != nil {
		m.addClientErr(pluginID, err)
		m.doCloseClient(err.Error(), client)
		return
	}

	m.setRunning(pluginID)
	m.monitor(client)
}

// Stop stops the plugin registered with identifier pluginID.
// It removes the plugin from the internal storage so further calls to Stat
// will return false. See Stat.
func (m *Manager) Stop(pluginID string) error {
	m.mu.Lock()
	client, ok := m.clients[pluginID]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("Manager: not connected to plugin with id '%s'", pluginID)
	}

	if client.doneCh == nil {
		return errors.New("already stopped")
	}

	m.log(log.DebugLevel, "stopping plugin '%s'", pluginID)

	var wg sync.WaitGroup
	wg.Add(1)
	client.doneCh <- &wg
	wg.Wait()

	return nil
}

func buildStat(client *granteeClientWrap) Stat {
	status := Stat{
		Errors: client.errors,
	}
	if client.running {
		status.ActiveAt = client.activeAt
	}
	return status
}

// Stat returns the stats of the plugin registered with identifier pluginID.
// It returns false if no plugin with that identifer could be found.
func (m *Manager) Stat(pluginID string) (Stat, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, ok := m.clients[pluginID]
	if !ok {
		return Stat{}, false
	}

	status := buildStat(client)
	return status, true
}

// Stats returns a map of pluginID to Stat.
func (m *Manager) Stats() map[string]Stat {
	m.mu.Lock()
	defer m.mu.Unlock()

	ret := make(map[string]Stat)

	for id, client := range m.clients {
		ret[id] = buildStat(client)
	}

	return ret
}

// Close closes all plugins and resources associated with this Manager.
func (m *Manager) Close() error {
	m.log(log.InfoLevel, "Close: stopping all plugins")

	m.mu.Lock()
	clients := m.clients
	m.mu.Unlock()

	var wg sync.WaitGroup
	for _, client := range clients {
		m.mu.Lock()
		doneCh := client.doneCh
		m.mu.Unlock()
		if doneCh != nil {
			wg.Add(1)
			go func() { doneCh <- &wg }()
		}
	}

	wg.Wait()

	return m.broker.Close()
}
