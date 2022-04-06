package workspace

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ernestrc/go-tui/cell"
	"golang.org/x/crypto/ssh"
)

// Manager manages resources on a workspace. It satisfies ResourceOpener.
type Manager struct {
	managerCfg
	workspace URI

	mu              sync.Mutex
	sshConn         *ssh.Client
	sshErr          error
	workspaceClient *Client

	osChdir func(string) error
	osGetwd func() (string, error)
}

type managerCfg struct {
	sshPrivateKeys []string
	sshTimeout     time.Duration
}

func isFileURI(file URI) bool {
	return strings.HasPrefix(file.uri, "file://")
}

func isSSHURI(file URI) bool {
	return strings.HasPrefix(file.uri, "ssh://")
}

// NewManager allocates storage fore a new Manage and initializes it with workspace.
func NewManager(workspace URI, opts ...Option) (*Manager, error) {
	ret := new(Manager)
	err := ret.Init(workspace, opts...)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// Init initializes m with workspace and calles os.Chdir to the new workspace
// if current working directory is not already equal to the given workspace.
func (m *Manager) Init(workspace URI, opts ...Option) error {
	m.osChdir = os.Chdir
	m.osGetwd = os.Getwd
	return m.init(workspace, opts...)
}

func (m *Manager) init(workspace URI, opts ...Option) error {
	m.workspace = workspace
	for _, o := range opts {
		o(&m.managerCfg)
	}
	if isFileURI(workspace) {
		return m.initLocal()
	}
	if isSSHURI(workspace) {
		return m.initRemote()
	}
	return fmt.Errorf("unknown scheme: %s", workspace)
}

func (m *Manager) initLocal() error {
	cwd, err := m.osGetwd()
	if err != nil {
		return fmt.Errorf("Failed to get working directory: %s", err)
	}
	absCwd, err := extractAbsPath(cwd)
	if err != nil {
		return err
	}
	workspacewd, err := localPath(m.workspace)
	if err != nil {
		return err
	}
	absWorkspacewd, err := extractAbsPath(workspacewd)
	if err != nil {
		return err
	}
	if absCwd != absWorkspacewd {
		err = m.osChdir(absWorkspacewd)
		if err != nil {
			return fmt.Errorf("failed to change to workspace directory %s: %s",
				absWorkspacewd, err)
		}
	}
	return nil
}

func (m *Manager) initRemote() (err error) {
	m.sshConn, err = connectOverSSH(m.managerCfg, m.workspace)
	if err != nil {
		return err
	}
	return m.initWorkspaceClient()
}

// Recover recovers the file with the swap file.
func (m *Manager) Recover(file, swapFile URI, buf *cell.Buffer) (
	FlusherCloser, error,
) {
	if isFileURI(file) {
		return recoverLocalFile(file, swapFile, buf)
	}
	if isSSHURI(file) {
		return m.recoverRemoteFile(file, swapFile, buf)
	}
	return nil, fmt.Errorf("unknown scheme: %s", file.uri)
}

// Open opens the file at the given URI and initializes buf with the contents of it.
// It uses swapDir as the file recovery and swap directory.
func (m *Manager) Open(
	file URI, buf *cell.Buffer, swapDir URI, readOnly bool,
) (
	FlusherCloser, error,
) {
	if isFileURI(file) {
		return openLocalFile(file, buf, swapDir, readOnly)
	}
	if isSSHURI(file) {
		return m.openRemoteFile(file, buf, swapDir, readOnly)
	}
	return nil, fmt.Errorf("unknown scheme: %s", file.uri)
}

// Close closes all resources associated with this Manager.
func (m *Manager) Close() error {
	if m.sshConn != nil {
		return m.sshConn.Close()
	}
	return nil
}
