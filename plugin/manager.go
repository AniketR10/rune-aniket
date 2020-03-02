package plugin

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
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
	doneCh   chan struct{}
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
	clients map[string]granteeClientWrap

	// resource mutex used to synchronize term event loop with
	// access to resources by plugins.
	rmu *sync.Mutex

	// used to abstract out go-plugin specific functionality
	builder pluginBuilder

	config managerConfig
}

// NewManager allocates storage for a new Manager and initializes it.
func NewManager(grantor Grantor, opts ...Option) *Manager {
	ret := new(Manager)
	ret.builder = goPluginGranteeBuilder
	ret.Init(grantor, opts...)
	return ret
}

// Init initializes this manager with grantor.
func (m *Manager) Init(grantor Grantor, opts ...Option) {
	m.grantor = grantor
	m.clients = make(map[string]granteeClientWrap)
	m.rmu = new(sync.Mutex)

	m.config = defaultManagerConfig
	for _, o := range opts {
		o(&m.config)
	}
}

func (m *Manager) log(msg string, args ...interface{}) {
	if m.config.logger == nil {
		return
	}
	m.config.logger.Debugf(msg, args...)
}

func (m *Manager) runPlugin(pluginID, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, err := m.builder(pluginID, path, m.grantor, m.config.logger)
	if err != nil {
		return err
	}

	clientWrap := granteeClientWrap{
		id:      pluginID,
		path:    path,
		running: false,
		doneCh:  make(chan struct{}),
		client:  client,
	}

	m.clients[pluginID] = clientWrap

	go m.handshake(pluginID, clientWrap)

	return nil
}

// Run runs the plugin at path with identifier pluginID. It returns an error
// if something went wrong when finding and executing the plugin executable.
// Once the plugin is up and running, errors can be retrieved with Stat.
func (m *Manager) Run(pluginID, path string) error {
	m.mu.Lock()
	if _, ok := m.clients[pluginID]; ok {
		m.mu.Unlock()
		return fmt.Errorf("Manager: already connected plugin with id: '%s'", pluginID)
	}
	m.mu.Unlock()
	return m.runPlugin(pluginID, path)
}

func (m *Manager) doGrant(
	ctx context.Context,
	pluginID string,
	client granteeClientWrap,
	perms []*proto.Permission,
) error {
	var denied []*proto.Permission
	var granted []*proto.PermissionGrant

	for _, p := range perms {
		srv, ok := m.grantor.Grant(pluginID, Permission(p.GetId()))
		if !ok {
			denied = append(denied, p)
			continue
		}

		broker := client.client.broker()
		grantID := broker.NextId()

		// for now this is fine, but once we have many more resources, this will
		// become very inefficient. We should refactor this interface
		// such that one plugin => one grpc server for all the resources
		// requested. Right now, each call to serve, spins a new listener
		// and a new GRPC server.
		go srv.Serve(pluginID, grantID, broker, m.rpcMutex())

		grant := &proto.PermissionGrant{
			Id:      p.Id,
			GrantId: grantID,
		}
		granted = append(granted, grant)
	}

	return client.client.sendGrants(ctx, denied, granted)
}

func (m *Manager) doCloseClient(reason string, client granteeClientWrap) {
	var err error
	func() {
		m.mu.Lock()
		defer m.mu.Unlock()

		err = client.client.shutdown(reason)
	}()
	if err != nil {
		m.addClientErr(client.id, err)
	}
}

func (m *Manager) checkHealth(client granteeClientWrap) (
	done bool, err error,
) {
	ctx, closeFn := context.WithTimeout(context.Background(), m.config.healthCheckTicker)
	defer closeFn()

	waitCh := make(chan error)
	go func() {
		select {
		case waitCh <- client.client.health(ctx):
		case <-ctx.Done():
		case <-client.doneCh:
		}
	}()

	select {
	case <-ctx.Done():
		err = errors.New("Manager: plugin health timeout")
	case err = <-waitCh:
	case <-client.doneCh:
		done = true
	}
	m.log("health check on plugin '%s': err=%v; done=%v", client.id, err, done)
	return
}

func (m *Manager) monitor(client granteeClientWrap) {
	const stopCalledReason = "Stop was called on plugin Manager"

	m.log("checking health of plugin '%s' every %+v",
		client.id, m.config.healthCheckTicker)

	timer := time.NewTicker(m.config.healthCheckTicker)

	triesLeft := 1 + m.config.healthRetries
	for {
		select {
		case <-timer.C:
			done, err := m.checkHealth(client)
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
			if done {
				m.doCloseClient(stopCalledReason, client)
				return
			}
		case <-client.doneCh:
			m.doCloseClient(stopCalledReason, client)
			return
		}
	}
}

func (m *Manager) addClientErr(pluginID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	clientWrap := m.clients[pluginID]
	clientWrap.errors = append(clientWrap.errors, err)
	m.clients[pluginID] = clientWrap

	m.log("plugin '%s' error: %s", pluginID, err)
}

func (m *Manager) setRunning(pluginID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	client := m.clients[pluginID]
	client.running = true
	client.activeAt = time.Now()
	m.clients[pluginID] = client
}

func (m *Manager) handshake(pluginID string, client granteeClientWrap) {
	ctx := context.Background()
	ctx, closeFn := context.WithTimeout(ctx, m.config.handshakeTimeout)
	defer closeFn()

	m.log("starting handshake for plugin '%s'", pluginID)

	doneCh := make(chan error)
	go func() {

		perms, err := client.client.permissions(ctx)
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
	defer m.mu.Unlock()

	client, ok := m.clients[pluginID]
	if !ok {
		return fmt.Errorf("Manager: not connected to plugin with id '%s'", pluginID)
	}

	m.log("stopping plugin '%s'", pluginID)

	close(client.doneCh)
	delete(m.clients, pluginID)
	return nil
}

func buildStat(client granteeClientWrap) Stat {
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
	m.mu.Lock()
	defer m.mu.Unlock()

	m.log("Close: stopping all plugins")

	for _, client := range m.clients {
		close(client.doneCh)
	}

	m.clients = make(map[string]granteeClientWrap)
	return nil
}

// ResourceLocker returns a Locker that synchronizes access to
// resources that have been shared with this manager.
func (m *Manager) ResourceLocker() sync.Locker {
	return m.rmu
}

type rpcMutex Manager

func (m *rpcMutex) Lock() {
	m.rmu.Lock()
}

func (m *rpcMutex) Unlock() {
	// force redraw after waking up polling gorouting
	term.Interrupt()
	m.rmu.Unlock()
}

func (m *Manager) rpcMutex() sync.Locker {
	return (*rpcMutex)(m)
}
