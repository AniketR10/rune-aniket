// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.
package process

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	"unstable.build/go-tui/api/config"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/component/notifications"
	"unstable.build/go-tui/extension"
	extensionpb "unstable.build/go-tui/extension/rpc"
	"unstable.build/go-tui/rpc"
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
	ctx       context.Context
	cancelCtx func()
}

// Stat represents the status of a Extension.
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
	pkg               string
	version           string
	notifications     browser.Notifications
}

// Option is a configuration option for a manager.
type Option func(cfg *managerConfig)

type extensionBuilder func(extensionID, path string,
	grantor extension.Grantor) (*granteeClient, error)

// Manager manages the lifecycle of extensions.
type Manager struct {
	mu      sync.Mutex
	grantor extension.Grantor
	clients map[string]*granteeClientWrap

	// resource mutex used to synchronize term event loop with
	// access to resources by extensions.
	rmu sync.Locker

	// used to abstract out go-plugin specific functionality
	builder extensionBuilder

	broker rpc.MuxBroker
	config managerConfig

	// srvs lifecycle ctx
	ctx       context.Context
	cancelCtx func()
	ctxWg     sync.WaitGroup
}

// NewManager allocates storage for a new Manager and initializes it.
func NewManager(grantor extension.Grantor, opts ...Option) (*Manager, error) {
	ret := new(Manager)
	err := ret.Init(grantor, opts...)
	ret.builder = goExtensionGranteeBuilder(ret, ret.config.dataDir)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// Init initializes this manager with grantor.
func (m *Manager) Init(grantor extension.Grantor, opts ...Option) (err error) {
	// enable registrants to inject other registrants as dependencies
	// where passing same instance is important due to the stateful nature
	// of some resource servers.
	m.grantor = extension.CachingGrantor(grantor)
	m.clients = make(map[string]*granteeClientWrap)

	m.config = defaultManagerConfig
	for _, o := range opts {
		o(&m.config)
	}
	m.rmu = m.config.locker

	m.broker, err = initHostBroker(m.config, m.config.dataDir, m.config.pkg, m.config.version)
	m.ctx, m.cancelCtx = context.WithCancel(context.Background())
	return
}

func (m *Manager) log(level log.Level, msg string, args ...interface{}) {
	if level <= log.WarnLevel && m.config.notifications != nil {
		// notifications runs async, consume args now since it's an error anyway
		var notiLevel notifications.Level
		switch level {
		case log.WarnLevel:
			notiLevel = notifications.LevelWarn
		default:
			notiLevel = notifications.LevelError
		}
		// we might or might not be calling this from a goroutine that's
		// already locked by the main mutex. Run in a separate goroutine
		// to avoid deadlocks. This should not be too numerous so it should be ok
		// to do this.
		// nolint:errcheck
		go m.config.notifications.Notify(notiLevel, msg, args...)
	}
	log.WithField(logging.KeyClass, "extension.Manager").Logf(level, msg, args...)
}

func (m *Manager) runExtension(extensionID, path string, config config.Config) error {
	client, err := m.builder(extensionID, path, m.grantor)
	if err != nil {
		return err
	}

	m.log(log.InfoLevel, "Running extension %q at path %q", extensionID, path)

	clientWrap := &granteeClientWrap{
		id:      extensionID,
		path:    path,
		running: false,
		doneCh:  make(chan *sync.WaitGroup),
		client:  client,
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.clients[extensionID] = clientWrap

	go m.handshake(extensionID, clientWrap, config)

	return nil
}

// Run runs the extension at path with identifier extensionID. It returns an error
// if something went wrong when finding and executing the extension executable.
// Once the extension is up and running, errors can be retrieved with Stat.
// Note that config is an optional argument.
func (m *Manager) Run(extensionID, path string, config config.Config) error {
	m.mu.Lock()
	_, ok := m.clients[extensionID]
	m.mu.Unlock()
	if ok {
		return fmt.Errorf("Manager: already connected extension with id: '%s'", extensionID)
	}
	if path == "" {
		return fmt.Errorf("Manager: cannot run extension %s: missing executable path", extensionID)
	}
	return m.runExtension(extensionID, path, config)
}

func (m *Manager) doGrant(
	ctx context.Context,
	extensionID string,
	client *granteeClientWrap,
	perms []*extensionpb.Permission,
) error {
	srv, err := m.broker.NewChannel(extensionID)
	if err != nil {
		return fmt.Errorf("new channel: %w", err)
	}
	if log.IsLevelEnabled(log.TraceLevel) {
		srv = rpc.LoggingGRPCServer(srv)
	}

	// for denied permissions we do not call ResourceRegistrar.Register so
	// if a malicious or otherwise client attempts to get a resource that was not granted
	// they'll receive an unimplemented status code.
	var denied []*extensionpb.Permission
	var serverResources []io.Closer
	granted := make(map[string]*extensionpb.PermissionGrant)
	for _, p := range perms {
		permissionID := util.SanitizeLine(p.GetId())
		if _, granted := granted[permissionID]; granted {
			continue
		}

		registrar, ok := m.grantor.Grant(extensionID, extension.Permission(permissionID))
		if !ok {
			denied = append(denied, p)
			continue
		}

		resource, err := registrar.Register(extensionID, m.grantor,
			srv.Registrar(), m.broker, m.rmu)
		if err != nil {
			m.log(log.ErrorLevel, "could not register %s for extension '%s': %v",
				permissionID, extensionID, err)
			denied = append(denied, p) // at least communicate to extension
			continue
		}

		granted[permissionID] = &extensionpb.PermissionGrant{
			Id: permissionID,
			// TODO pass temporary token that can be used
			// by client and server auth middleware
			Address: srv.Addr().String(),
		}

		serverResources = append(serverResources, resource)
	}

	m.mu.Lock()
	client.resources = serverResources
	client.ctx, client.cancelCtx = context.WithCancel(context.Background())
	ctx = client.ctx
	m.mu.Unlock()

	go func() {
		if err := srv.Serve(ctx); err != nil {
			log.Errorf("mux server serve: %v", err)
		}
	}()

	m.ctxWg.Add(1)
	go func() {
		defer m.ctxWg.Done()
		<-ctx.Done()
		m.log(log.DebugLevel, "serve context is done: stopping mux server %v", srv.Addr())
		srv.Stop()
	}()

	return client.client.sendGrants(ctx, denied, granted)
}

func (m *Manager) doCloseClient(reason string, client *granteeClientWrap) (
	ret chan *sync.WaitGroup,
) {
	m.mu.Lock()
	extensionID := client.id
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

	for _, resource := range resources {
		if err := resource.Close(); err != nil {
			clientErr = multierr.Append(clientErr, err)
		}
	}
	// Close might actually call to workspace/browser/editor/etc resources
	// so unfortunately we cannot parallelize it amongst different clients
	m.mu.Unlock()

	m.log(log.WarnLevel, "stopping extension '%s' due to %s. Reload workspace to restart it.",
		extensionID, reason)

	if clientErr != nil {
		m.log(log.DebugLevel, "error stopping extension '%s': %v", extensionID, clientErr)
	}
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
	m.log(log.TraceLevel, "health check on extension '%s': err=%v", client.id, err)
	return err
}

func (m *Manager) monitor(client *granteeClientWrap) {

	m.log(log.DebugLevel, "checking health of extension '%s' every %+v",
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
				m.addClientErr(client.id, fmt.Errorf("health check: %w", err))
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
				return
			}
		case wg := <-client.doneCh:
			defer wg.Done()
			m.doCloseClient("extension manager is closing", client)
			return
		}
	}
}

func (m *Manager) addClientErr(extensionID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	clientWrap := m.clients[extensionID]
	clientWrap.errors = append(clientWrap.errors, err)

	m.log(log.ErrorLevel, "extension '%s' error: %v", extensionID, err)
}

func (m *Manager) setRunning(extensionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	client := m.clients[extensionID]
	client.running = true
	client.activeAt = time.Now()
}

func (m *Manager) handshake(
	extensionID string, client *granteeClientWrap, config config.Config,
) {
	ctx := context.Background()
	ctx, closeFn := context.WithTimeout(ctx, m.config.handshakeTimeout)
	defer closeFn()

	m.log(log.TraceLevel, "starting handshake for extension '%s'", extensionID)

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

		err = m.doGrant(ctx, extensionID, client, perms)
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
		err = fmt.Errorf("handshake: %w", err)
		m.addClientErr(extensionID, err)
		m.doCloseClient("handshake error", client)
		return
	}

	m.setRunning(extensionID)
	m.monitor(client)
}

// Stop stops the extension registered with identifier extensionID.
// It removes the extension from the internal storage so further calls to Stat
// will return false. See Stat.
func (m *Manager) Stop(extensionID string) error {
	m.mu.Lock()
	client, ok := m.clients[extensionID]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("Manager: not connected to extension with id '%s'", extensionID)
	}

	if client.doneCh == nil {
		return errors.New("already stopped")
	}

	m.log(log.DebugLevel, "stopping extension '%s'", extensionID)

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

// Stat returns the stats of the extension registered with identifier extensionID.
// It returns false if no extension with that identifer could be found.
func (m *Manager) Stat(extensionID string) (Stat, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, ok := m.clients[extensionID]
	if !ok {
		return Stat{}, false
	}

	status := buildStat(client)
	return status, true
}

// Stats returns a map of extensionID to Stat.
func (m *Manager) Stats() map[string]Stat {
	m.mu.Lock()
	defer m.mu.Unlock()

	ret := make(map[string]Stat)

	for id, client := range m.clients {
		ret[id] = buildStat(client)
	}

	return ret
}

// Close closes all extensions and resources associated with this Manager.
func (m *Manager) Close() error {
	m.log(log.DebugLevel, "Close: stopping all extensions")

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

	m.cancelCtx()
	m.ctxWg.Wait()

	return m.broker.Close()
}
