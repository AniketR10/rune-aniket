package workspace

import (
	"context"
	"errors"
	"fmt"

	"github.com/unstablebuild/blue/logging"
	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/api/config"
	schemeapi "unstable.build/go-tui/api/scheme"
	workspaceapi "unstable.build/go-tui/api/workspace"
)

var (
	errProcNotFound   = errors.New("process not found")
	errProcNotRunning = errors.New("process not running")
)

var _ SchemeManager = (*Manager)(nil)

// Manager manages resources for a collection of workspaces.
// It allows clients to register new Schemes and add new workspaces.
type Manager struct {
	cfg config.Config

	schemes    map[string]schemeapi.SchemeFunc
	workspaces map[string]managerWorkspace
}

// adds remove on Close
type managerWorkspace struct {
	m   *Manager
	uri workspaceapi.URI
	Workspace
}

func (w managerWorkspace) Close() error {
	ret := w.Workspace.Close()
	w.m.removeWorkspace(w.uri)
	return ret
}

// NewManager allocates storage for a new Manager and initializes it.
// See Manager.Init for more details.
func NewManager(cfg config.Config) *Manager {
	ret := new(Manager)
	ret.Init(cfg)
	return ret
}

// Init initializes m and register a default implementation for local file management
// under the file:// scheme.
func (m *Manager) Init(cfg config.Config) {
	m.cfg = cfg
	m.schemes = make(map[string]schemeapi.SchemeFunc)
	m.workspaces = make(map[string]managerWorkspace)
}

// RegisterScheme registers a new scheme for the given scheme and uses fn
// to allocate it for new workspaces. It returns ErrSchemeAlreadyRegistered if there's
// already a scheme registered for the given scheme.
func (m *Manager) RegisterScheme(scheme string, fn schemeapi.SchemeFunc) error {
	_, ok := m.schemes[scheme]
	if ok {
		return schemeapi.ErrSchemeAlreadyRegistered
	}
	m.log(log.DebugLevel, "RegisterScheme %q", scheme)
	if log.IsLevelEnabled(log.TraceLevel) {
		fn = LoggingScheme(scheme, fn)
	}
	m.schemes[scheme] = fn
	return nil
}

// UnregisterScheme unregisters the given scheme.
func (m *Manager) UnregisterScheme(scheme string) error {
	_, ok := m.schemes[scheme]
	if !ok {
		return fmt.Errorf("scheme %q not registered", scheme)
	}
	m.log(log.DebugLevel, "UnregisterScheme %q", scheme)
	delete(m.schemes, scheme)
	return nil
}

func (m *Manager) removeWorkspace(uri workspaceapi.URI) {
	delete(m.workspaces, uri.String())
}

// Scheme returns a SchemeFunc for the given URI.
func (m *Manager) Scheme(uri workspaceapi.URI) (schemeapi.SchemeFunc, error) {
	schemeFn, ok := m.schemes[uri.Scheme()]
	if !ok {
		return nil, fmt.Errorf("scheme not registered %q", uri.Scheme())
	}
	return schemeFn, nil
}

// AddWorkspace returns a Workspace capable of managing resources on
// the given URI. It returns an error if no Scheme has been registered
// (previously via RegisterScheme) for the given workspace's scheme. Note that
// the given URI should not be a file URI.
//
// If a workspace has already been added for the given URI, then this
// method returns it. This method does not follow the same semantics as
// Workspace as the latter uses IsWorkspaceURI semantics and this
// will create a new workspace if the uri strings are different.
func (m *Manager) AddWorkspace(
	ctx context.Context, uri workspaceapi.URI,
) (Workspace, error) {
	if w, ok := m.workspaces[uri.String()]; ok {
		return w, nil
	}

	schemeFn, ok := m.schemes[uri.Scheme()]
	if !ok {
		return nil, fmt.Errorf("scheme not registered %q", uri.Scheme())
	}
	cfg, err := m.cfg.GetConfig(uri.Scheme())
	if err != nil && err != config.ErrNotFound {
		return nil, fmt.Errorf("unable to load scheme config: %s", err)
	}
	if cfg == nil {
		cfg = config.NopConfig()
	}
	scheme, err := schemeFn(ctx, cfg, uri)
	if err != nil {
		return nil, fmt.Errorf("new workspace %q: %w", uri, err)
	}

	workspace := NewSchemeWorkspace(uri, scheme)
	managerWorkspace := managerWorkspace{uri: uri, m: m, Workspace: workspace}
	m.workspaces[uri.String()] = managerWorkspace

	m.log(log.DebugLevel, "AddWorkspace(%q)", uri.String())

	return managerWorkspace, nil
}

// Workspace returns a Workspace suitable for the given file
// or false if there's currently no Workspace initialized.
func (m *Manager) Workspace(file workspaceapi.URI) (Workspace, bool, error) {
	for _, workspace := range m.workspaces {
		is, err := IsWorkspaceURI(workspace, file)
		if err != nil {
			return nil, false, fmt.Errorf("IsWorkspaceURI: %s", err)
		}
		if is {
			return workspace, true, nil
		}
	}
	return nil, false, nil
}

// Close closes all resources associated with this Manager.
func (m *Manager) Close() error {
	var ret error
	for _, workspace := range m.workspaces {
		if err := workspace.Close(); err != nil {
			ret = multierr.Append(ret, err)
		}
	}
	return ret
}

func (m *Manager) log(level log.Level, msg string, args ...interface{}) {
	log.WithField(logging.KeyClass, "workspace.Manager").
		Logf(level, msg, args...)
}
