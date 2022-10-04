package workspace

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/ernestrc/blue/logging"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/debug"
)

const (
	// FileScheme represents the local file URL scheme.
	FileScheme = "file"
)

// CurrentUserHostURI builds a URI from a path. If path is relative
// it uses the current working directory as the base of the path and
// if ~ is used to identify the home directory, the current user's home
// directory is used as the base.
// This should only used instead of Manager.URI before a workspace.Manager is
// constructed or for other advanced uses cases.
func CurrentUserHostURI(path string) (URI, error) {
	absPath, err := ExpandPath(path, user.Current, os.Getwd)
	if err != nil {
		return URI{}, err
	}
	return makeLocalURI(absPath)
}

type fileScheme struct {
	osStat     func(path string) (os.FileInfo, error)
	getUser    func() (*user.User, error)
	lookupUser func(string) (*user.User, error)
	workspace  URI
	cmds       sync.Map
	nextPid    int32
}

// NewFileScheme returns a Scheme that manages resources
// on the local file system.
func NewFileScheme(cfg config.Config, workspace URI) (Scheme, error) {
	ret := new(fileScheme)
	ret.getUser = user.Current
	ret.lookupUser = user.Lookup
	ret.osStat = os.Stat
	err := ret.init(workspace)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (p *fileScheme) init(workspace URI) error {
	if workspace.Host() != "" || workspace.User() != "" || workspace.Scheme() != FileScheme {
		return errors.New("invalid file URI")
	}
	workspacewd := workspace.Path()
	fs, err := p.osStat(workspacewd)
	if err != nil {
		return err
	}
	if !fs.IsDir() {
		workspace = Dir(workspace)
	}
	p.workspace = workspace
	return nil
}

func (p *fileScheme) Open(path string, flag int, perm os.FileMode) (File, *Error) {
	f, err := os.OpenFile(path, flag, perm)
	if err != nil {
		return nil, &Error{
			Err:          err,
			IsPermission: os.IsPermission(err),
			IsExist:      os.IsExist(err),
			IsNotExist:   os.IsNotExist(err),
		}
	}
	return f, nil
}

func (p *fileScheme) Remove(path string) error {
	return os.Remove(path)
}

func (p *fileScheme) Rename(old, new string) error {
	return os.Rename(old, new)
}

func (p *fileScheme) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (p *fileScheme) Lstat(path string) (os.FileInfo, error) {
	return os.Lstat(path)
}

func (p *fileScheme) ReadLink(path string) (string, error) {
	return os.Readlink(path)
}

func (m *fileScheme) getUserOrLookup() (*user.User, error) {
	if m.workspace.parsed.User == nil {
		return m.getUser()
	}
	username := m.workspace.parsed.User.Username()
	return m.lookupUser(username)
}

func (p *fileScheme) URI(path string) (URI, error) {
	absPath, err := ExpandPath(path, p.getUserOrLookup, func() (string, error) {
		return p.workspace.Path(), nil
	})
	if err != nil {
		return URI{}, err
	}
	return makeLocalURI(absPath)
}

func (m *fileScheme) Command(name string, arg ...string) (Pid, error) {
	cmd := exec.Command(name, arg...)
	cmd.Dir = m.workspace.Path()
	nextPid := atomic.AddInt32(&m.nextPid, 1)

	debug.StandardLogger().
		WithField(logging.KeyClass, "fileScheme").
		WithField("URI", m.workspace.String()).
		Debugf("exec.Command: (%#v, pid=%d)", cmd, nextPid)

	m.cmds.Store(Pid(nextPid), cmd)
	return Pid(nextPid), nil
}

func (m *fileScheme) getCmdForPid(pid Pid) (*exec.Cmd, bool) {
	f, ok := m.cmds.Load(pid)
	if ok {
		return f.(*exec.Cmd), true
	}
	return nil, false
}

func (m *fileScheme) Start(pid Pid) error {
	f, ok := m.getCmdForPid(pid)
	if !ok {
		return errProcNotFound
	}
	err := f.Start()
	if err != nil {
		return fmt.Errorf("Cmd.Start: %w", err)
	}
	return nil
}

func (m *fileScheme) Signal(pid Pid, signal syscall.Signal) error {
	f, ok := m.getCmdForPid(pid)
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

func (m *fileScheme) StderrPipe(pid Pid) (io.ReadCloser, error) {
	f, ok := m.getCmdForPid(pid)
	if !ok {
		return nil, errProcNotFound
	}
	pipe, err := f.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("Cmd.StderrPipe: %w", err)
	}
	return pipe, err
}

func (m *fileScheme) StdinPipe(pid Pid) (io.WriteCloser, error) {
	f, ok := m.getCmdForPid(pid)
	if !ok {
		return nil, errProcNotFound
	}
	pipe, err := f.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("Cmd.StdinPipe: %w", err)
	}
	return pipe, err
}

func (m *fileScheme) StdoutPipe(pid Pid) (io.ReadCloser, error) {
	f, ok := m.getCmdForPid(pid)
	if !ok {
		return nil, errProcNotFound
	}
	pipe, err := f.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("Cmd.StdoutPipe: %w", err)
	}
	return pipe, err
}

func (m *fileScheme) Wait(pid Pid) error {
	f, ok := m.getCmdForPid(pid)
	if !ok {
		return errProcNotFound
	}
	defer m.cmds.Delete(pid)
	err := f.Wait()
	if err != nil {
		return fmt.Errorf("Cmd.Wait: %w", err)
	}
	return err
}

func (m *fileScheme) Close() error {
	var ret error

	var copyCmds []*exec.Cmd
	var keys []interface{}
	m.cmds.Range(func(key, cmd interface{}) bool {
		copyCmds = append(copyCmds, cmd.(*exec.Cmd))
		keys = append(keys, key)
		return true
	})
	for _, cmd := range copyCmds {
		if cmd.Process != nil {
			err := syscall.Kill(cmd.Process.Pid, syscall.SIGTERM)
			if err != nil {
				ret = err
			}
		}
	}

	for _, key := range keys {
		m.cmds.Delete(key)
	}
	return ret
}

func makeLocalURI(path string) (URI, error) {
	uriStr := "file://" + path
	u, err := url.Parse(uriStr)
	if err != nil {
		return URI{}, err
	}

	return makeFileURI(u)
}
