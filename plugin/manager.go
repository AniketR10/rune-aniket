package plugin

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/ernestrc/blue/datastore/document"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/util"
	log "github.com/sirupsen/logrus"
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
	logger:            &pluginLogger,
}

type granteeClientWrap struct {
	id       string
	path     string
	running  bool
	activeAt time.Time
	errors   []error
	doneCh   chan *sync.WaitGroup
	client   *granteeClient
}

// Stat represents the status of a Plugin.
type Stat struct {
	ActiveAt time.Time
	Errors   []error
}

type managerConfig struct {
	logger *log.Logger

	handshakeTimeout  time.Duration
	healthCheckTicker time.Duration
	healthRetries     int
}

// Option is a configuration option for a manager.
type Option func(cfg *managerConfig)

type pluginBuilder func(pluginID, path string,
	grantor Grantor, log *log.Logger) (*granteeClient, error)

// Manager manages the lifecycle of plugins.
type Manager struct {
	mu      sync.Mutex
	grantor Grantor
	clients map[string]*granteeClientWrap

	// resource mutex used to synchronize term event loop with
	// access to resources by plugins.
	rmu *sync.Mutex

	// used to abstract out go-plugin specific functionality
	builder pluginBuilder

	brokerServer *document.Server
	broker       proto.MuxBroker
	brokerAddr   net.Addr
	config       managerConfig
}

// NewManager allocates storage for a new Manager and initializes it.
func NewManager(grantor Grantor, opts ...Option) (*Manager, error) {
	ret := new(Manager)
	ret.builder = goPluginGranteeBuilder(ret)
	err := ret.Init(grantor, opts...)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// Init initializes this manager with grantor.
func (m *Manager) Init(grantor Grantor, opts ...Option) (err error) {
	m.grantor = grantor
	m.clients = make(map[string]*granteeClientWrap)
	m.rmu = new(sync.Mutex)

	m.config = defaultManagerConfig
	for _, o := range opts {
		o(&m.config)
	}

	cache := document.NewInMemoryCache()
	m.brokerServer = document.NewServer(cache)
	m.broker, m.brokerAddr, err = initHostBroker(m.config, cache, m.brokerServer)
	if err != nil {
		return
	}

	return
}

func (m *Manager) log(msg string, args ...interface{}) {
	if m.config.logger == nil {
		return
	}
	m.config.logger.Debugf(msg, args...)
}

func (m *Manager) runPlugin(pluginID, path string, config Config) error {
	client, err := m.builder(pluginID, path, m.grantor, m.config.logger)
	if err != nil {
		return err
	}

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
func (m *Manager) Run(pluginID, path string, config Config) error {
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
	perms []*proto.Permission,
) error {
	var denied []*proto.Permission
	var granted []*proto.PermissionGrant

	for _, p := range perms {
		permissionID := util.SanitizeLine(p.GetId())
		srv, ok := m.grantor.Grant(pluginID, Permission(permissionID))
		if !ok {
			denied = append(denied, p)
			continue
		}

		grantID := m.broker.NextId()

		// for now this is fine, but once we have many more resources, this will
		// become very inefficient. We should refactor this interface
		// such that one plugin => one grpc server for all the resources
		// requested. Right now, each call to serve, spins a new listener
		// and a new GRPC server.
		go srv.Serve(pluginID, grantID, m.broker, m.config.logger, m.rmu)

		grant := &proto.PermissionGrant{
			Id:      p.Id,
			GrantId: grantID,
		}
		granted = append(granted, grant)
	}

	return client.client.sendGrants(ctx, denied, granted)
}

func (m *Manager) doCloseClient(reason string, client *granteeClientWrap) {
	m.mu.Lock()
	if client.doneCh == nil {
		return
	}
	client.doneCh = nil
	m.mu.Unlock()

	err := client.client.shutdown(reason)
	if err != nil {
		m.addClientErr(client.id, err)
	}
	m.log("stopped plugin with id '%s': err=%v", client.id, err)
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
	m.log("health check on plugin '%s': err=%v", client.id, err)
	return err
}

func (m *Manager) monitor(client *granteeClientWrap) {

	m.log("checking health of plugin '%s' every %+v",
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
				m.doCloseClient("exhausted health check retries", client)
				return
			}
		case wg := <-client.doneCh:
			defer wg.Done()
			m.doCloseClient("Stop was called on plugin Manager", client)
			return
		}
	}
}

func (m *Manager) addClientErr(pluginID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	clientWrap := m.clients[pluginID]
	clientWrap.errors = append(clientWrap.errors, err)

	m.log("plugin '%s' error: %s", pluginID, err)
}

func (m *Manager) setRunning(pluginID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	client := m.clients[pluginID]
	client.running = true
	client.activeAt = time.Now()
}

func (m *Manager) handshake(
	pluginID string, client *granteeClientWrap, config Config,
) {
	ctx := context.Background()
	ctx, closeFn := context.WithTimeout(ctx, m.config.handshakeTimeout)
	defer closeFn()

	m.log("starting handshake for plugin '%s'", pluginID)

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

	m.log("stopping plugin '%s'", pluginID)

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
	m.log("Close: stopping all plugins")

	m.mu.Lock()
	clients := m.clients
	m.mu.Unlock()

	var wg sync.WaitGroup
	for _, client := range clients {
		if client.doneCh != nil {
			wg.Add(1)
			client.doneCh <- &wg
		}
	}

	wg.Wait()

	err1 := m.brokerServer.Close()
	err2 := m.broker.Close()

	if err1 != nil {
		return err1
	}
	return err2
}

// ResourceLocker returns a Locker that synchronizes access to
// resources that have been shared with this manager.
func (m *Manager) ResourceLocker() sync.Locker {
	return (*rpcMutex)(m)
}

type rpcMutex Manager

func (m *rpcMutex) Lock() {
	m.rmu.Lock()
}

func (m *rpcMutex) Unlock() {
	m.rmu.Unlock()
}
