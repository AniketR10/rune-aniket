package workspace

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ernestrc/go-tui/cell"
	"golang.org/x/crypto/ssh"
)

var (
	_                 Executor = (*Manager)(nil)
	errProcNotFound            = errors.New("process not found")
	errProcNotRunning          = errors.New("process not running")
)

// Manager manages resources on a workspace. It satisfies ResourceOpener.
type Manager struct {
	managerCfg
	workspace URI
	cmds      map[Pid]*exec.Cmd
	nextPid   int32

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
	m.cmds = make(map[Pid]*exec.Cmd)
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

func (m *Manager) commandLocal(name string, arg ...string) (Pid, error) {
	cmd := exec.Command(name, arg...)
	m.nextPid++
	m.cmds[Pid(m.nextPid)] = cmd
	return Pid(m.nextPid), nil
}

func (m *Manager) startLocal(pid Pid) error {
	f, ok := m.cmds[pid]
	if !ok {
		return errProcNotFound
	}
	err := f.Start()
	if err != nil {
		return fmt.Errorf("Cmd.Start: %w", err)
	}
	return nil
}

func (m *Manager) signalLocal(pid Pid, signal syscall.Signal) error {
	f, ok := m.cmds[pid]
	if !ok {
		return errProcNotFound
	}
	if f.Process == nil {
		return errProcNotRunning
	}
	err := syscall.Kill(int(f.Process.Pid), signal)
	if err != nil {
		return fmt.Errorf("syscall.Kill: %w", err)
	}
	return nil
}

func (m *Manager) stderrPipeLocal(pid Pid) (io.ReadCloser, error) {
	f, ok := m.cmds[pid]
	if !ok {
		return nil, errProcNotFound
	}
	pipe, err := f.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("Cmd.StderrPipe: %w", err)
	}
	return pipe, err
}

func (m *Manager) stdinPipeLocal(pid Pid) (io.WriteCloser, error) {
	f, ok := m.cmds[pid]
	if !ok {
		return nil, errProcNotFound
	}
	pipe, err := f.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("Cmd.StdinPipe: %w", err)
	}
	return pipe, err
}

func (m *Manager) stdoutPipeLocal(pid Pid) (io.ReadCloser, error) {
	f, ok := m.cmds[pid]
	if !ok {
		return nil, errProcNotFound
	}
	pipe, err := f.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("Cmd.StdoutPipe: %w", err)
	}
	return pipe, err
}

func (m *Manager) waitLocal(pid Pid) error {
	f, ok := m.cmds[pid]
	if !ok {
		return errProcNotFound
	}
	err := f.Wait()
	if err != nil {
		return fmt.Errorf("Cmd.Wait: %w", err)
	}
	return err
}

func (m *Manager) Command(name string, arg ...string) (Pid, error) {
	if isFileURI(m.workspace) {
		return m.commandLocal(name, arg...)
	}
	if isSSHURI(m.workspace) {
		return m.workspaceClient.Command(name, arg...)
	}
	panic("manager has an invalid workspace URI")
}

func (m *Manager) Start(pid Pid) error {
	if isFileURI(m.workspace) {
		return m.startLocal(pid)
	}
	if isSSHURI(m.workspace) {
		return m.workspaceClient.Start(pid)
	}
	panic("manager has an invalid workspace URI")
}

func (m *Manager) Signal(pid Pid, sig syscall.Signal) error {
	if isFileURI(m.workspace) {
		return m.signalLocal(pid, sig)
	}
	if isSSHURI(m.workspace) {
		return m.workspaceClient.Signal(pid, sig)
	}
	panic("manager has an invalid workspace URI")
}

func (m *Manager) StderrPipe(pid Pid) (io.ReadCloser, error) {
	if isFileURI(m.workspace) {
		return m.stderrPipeLocal(pid)
	}
	if isSSHURI(m.workspace) {
		return m.workspaceClient.StderrPipe(pid)
	}
	panic("manager has an invalid workspace URI")
}

func (m *Manager) StdinPipe(pid Pid) (io.WriteCloser, error) {
	if isFileURI(m.workspace) {
		return m.stdinPipeLocal(pid)
	}
	if isSSHURI(m.workspace) {
		return m.workspaceClient.StdinPipe(pid)
	}
	panic("manager has an invalid workspace URI")
}

func (m *Manager) StdoutPipe(pid Pid) (io.ReadCloser, error) {
	if isFileURI(m.workspace) {
		return m.stdoutPipeLocal(pid)
	}
	if isSSHURI(m.workspace) {
		return m.workspaceClient.StdoutPipe(pid)
	}
	panic("manager has an invalid workspace URI")
}

func (m *Manager) Wait(pid Pid) error {
	if isFileURI(m.workspace) {
		return m.waitLocal(pid)
	}
	if isSSHURI(m.workspace) {
		return m.workspaceClient.Wait(pid)
	}
	panic("manager has an invalid workspace URI")
}

// Close closes all resources associated with this Manager.
func (m *Manager) Close() error {
	var ret error
	if m.sshConn != nil {
		ret = m.sshConn.Close()
	}
	for _, cmd := range m.cmds {
		if cmd.Process != nil {
			err := syscall.Kill(cmd.Process.Pid, syscall.SIGTERM)
			if err != nil {
				ret = err
			}
		}
	}
	return ret
}
