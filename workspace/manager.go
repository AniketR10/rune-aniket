package workspace

import (
	"errors"
	"fmt"

	"github.com/ernestrc/blue/logging"
	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/debug"
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

	schemes    map[string]func(config.Config, URI) (Scheme, error)
	workspaces map[string]managerWorkspace
}

// adds remove on Close
type managerWorkspace struct {
	m   *Manager
	uri URI
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
	m.schemes = make(map[string]func(config.Config, URI) (Scheme, error))
	m.workspaces = make(map[string]managerWorkspace)
}

// RegisterScheme registers a new scheme for the given scheme and uses fn
// to allocate it for new workspaces. It returns an error if there's already
// a scheme registered for the given scheme.
func (m *Manager) RegisterScheme(scheme string, fn SchemeFunc) error {
	_, ok := m.schemes[scheme]
	if ok {
		return fmt.Errorf("scheme %q already registered", scheme)
	}
	m.log(log.DebugLevel, "RegisterScheme %q", scheme)
	if debug.StandardLogger().IsLevelEnabled(log.TraceLevel) {
		fn = LoggingScheme(scheme, fn)
	}
	m.schemes[scheme] = fn
	return nil
}

func (m *Manager) removeWorkspace(uri URI) {
	delete(m.workspaces, uri.String())
}

// Scheme returns a SchemeFunc for the given URI.
func (m *Manager) Scheme(uri URI) (SchemeFunc, error) {
	schemeFn, ok := m.schemes[uri.parsed.Scheme]
	if !ok {
		return nil, fmt.Errorf("scheme not registered %q", uri.parsed.Scheme)
	}
	return schemeFn, nil
}

// AddWorkspace returns a Workspace capable of managing resources on
// the given URI. It returns an error if no Scheme has been registered
// (previously via RegisterScheme) for the given workspace's scheme. Note that
// the given URI can be a file URI, in which case the workspace will default
// to the file's directory as the workspace URI.
//
// If a workspace has already been added for the given URI, then this
// method returns it. This method does not follow the same semantics as
// Workspace as the latter uses IsWorkspaceURI semantics and this
// will create a new workspace if the uri strings are different.
func (m *Manager) AddWorkspace(uri URI) (Workspace, error) {
	// In theory, we might create more workspaces than needed if one
	// is open with file URI, and another with its dir, but it's
	// preferable over returning the same for all local file schemes,
	// as it breaks plugins that need an executor with a Cmd.Dir set
	// to the workspace dir.
	// In practice, this is only worrying for one-off Load of non-workspace
	// file-scheme files, but these requests are managed by multi_workspace
	// which uses IsWorkspaceURI semantics and so it returns any file-scheme.
	if w, ok := m.workspaces[uri.String()]; ok {
		return w, nil
	}

	schemeFn, ok := m.schemes[uri.parsed.Scheme]
	if !ok {
		return nil, fmt.Errorf("scheme not registered %q", uri.parsed.Scheme)
	}
	cfg, err := m.cfg.GetConfig(uri.parsed.Scheme)
	if err != nil && err != config.ErrNotFound {
		return nil, fmt.Errorf("unable to load scheme config: %s", err)
	}
	if cfg == nil {
		cfg = config.NopConfig()
	}
	scheme, err := schemeFn(cfg, uri)
	if err != nil {
		return nil, fmt.Errorf("could not add new workspace %q: %w", uri, err)
	}

	workspace := NewSchemeWorkspace(uri, scheme)
	managerWorkspace := managerWorkspace{uri: uri, m: m, Workspace: workspace}
	m.workspaces[uri.String()] = managerWorkspace

	m.log(log.DebugLevel, "AddWorkspace(%q)", uri.String())

	return managerWorkspace, nil
}

// Workspace returns a Workspace suitable for the given file
// or false if there's currently no Workspace initialized.
func (m *Manager) Workspace(file URI) (Workspace, bool, error) {
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
	debug.StandardLogger().
		WithField(logging.KeyClass, "workspace.Manager").Logf(level, msg, args...)
}
