package workspace

import (
	"errors"
	"fmt"
	"os"

	multierr "github.com/ernestrc/go-multierror"
	"github.com/ernestrc/go-tui/config"
)

var (
	errProcNotFound   = errors.New("process not found")
	errProcNotRunning = errors.New("process not running")
)

// Manager manages resources for a collection of workspaces.
// It allows clients to register new Schemes and add new workspaces.
type Manager struct {
	cfg config.Config

	schemes    map[string]func(config.Config, URI) (Scheme, error)
	workspaces map[string]Workspace
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
	m.workspaces = make(map[string]Workspace)
}

// RegisterScheme registers a new scheme for the given scheme and uses fn
// to allocate it for new workspaces. It returns an error if there's already
// a scheme registered for the given scheme.
func (m *Manager) RegisterScheme(scheme string, fn func(config.Config, URI) (Scheme, error)) error {
	_, ok := m.schemes[scheme]
	if ok {
		return fmt.Errorf("scheme %q already registered", scheme)
	}
	m.schemes[scheme] = fn
	return nil
}

func (m *Manager) removeWorkspace(uri URI) {
	delete(m.workspaces, uri.String())
}

// AddWorkspace returns a Workspace capable of managing resources on
// the given URI. It returns an error if no Scheme has been registered
// (previously via RegisterScheme) for the given workspace's scheme or
// if this workspace has already been added for the given URI. Note that
// the given URI can be a file URI, in which case the workspace will default
// to the file's directory as the workspace URI.
func (m *Manager) AddWorkspace(uri URI) (Workspace, error) {
	if _, ok := m.WorkspaceFile(uri); ok {
		return nil, errors.New("workspace already added")
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
	// wrap to provide multi-scheme support
	// for one-off requests to open a file out of the current
	workspace = newMulti(m, uri, workspace)
	workspace = managerWorkspace{uri: uri, m: m, Workspace: workspace}
	m.workspaces[uri.String()] = workspace
	return workspace, nil
}

// WorkspaceFile returns a Workspace suitable for the given file
// or false if there's currently no Workspace initialized.
func (m *Manager) WorkspaceFile(file URI) (Workspace, bool) {
	for _, workspace := range m.workspaces {
		if IsWorkspaceURI(workspace, file) {
			return workspace, true
		}
	}
	return nil, false
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

func mapErrors(err error) error {
	osErr, ok := err.(*Error)
	if !ok {
		return err
	}
	if osErr.IsPermission {
		return os.ErrPermission
	}
	if osErr.IsNotExist {
		return os.ErrNotExist
	}
	if osErr.IsExist {
		return os.ErrExist
	}

	return osErr.Err
}
