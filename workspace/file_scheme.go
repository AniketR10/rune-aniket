package workspace

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/ernestrc/blue/iterator"
	"github.com/ernestrc/blue/logging"
	multierr "github.com/ernestrc/go-multierror"
	"github.com/ernestrc/sensible/find"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/term/pty"
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

// NewFileScheme returns a Scheme that manages resources
// on the local file system.
func NewFileScheme(cfg config.Config, workspace URI) (Scheme, error) {
	ret := new(fileScheme)
	ret.getUser = user.Current
	ret.lookupUser = user.Lookup
	ret.osStat = os.Stat
	err := ret.init(cfg, workspace)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

var defaultWorkers int

func init() {
	maxProcs := runtime.GOMAXPROCS(0)
	numCPU := runtime.NumCPU()
	defaultWorkers = int(math.Min(float64(maxProcs), float64(numCPU)))
}

type fileScheme struct {
	osStat     func(path string) (os.FileInfo, error)
	getUser    func() (*user.User, error)
	lookupUser func(string) (*user.User, error)
	workspace  URI
	workers    int
	cmds       sync.Map
	nextPid    int32
}

type execCmd struct {
	// serialize access to a cmd.Exec
	pid atomic.Int64
	mu  sync.Mutex
	*exec.Cmd
}

func (p *fileScheme) init(cfg config.Config, workspace URI) error {
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
	p.workers, err = cfg.GetInt("workers")
	if err == config.ErrNotFound {
		p.workers = defaultWorkers
		err = nil
	}
	if err != nil {
		return err
	}
	if p.workers == 0 {
		return errors.New("invalid configuration: cannot set 'workers' to 0")
	}
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
	path, err := find.Executable(name)
	if err != nil {
		// let Command fail and return the os error
		path = name
	}
	cmd := exec.Command(path, arg...)
	cmd.Dir = m.workspace.Path()
	nextPid := atomic.AddInt32(&m.nextPid, 1)

	debug.StandardLogger().
		WithField(logging.KeyClass, "fileScheme").
		WithField("URI", m.workspace.String()).
		Debugf("exec.Command: (%#v, pid=%d)", cmd, nextPid)

	m.cmds.Store(Pid(nextPid), &execCmd{Cmd: cmd})
	return Pid(nextPid), nil
}

func (m *fileScheme) getCmdForPid(pid Pid) (*execCmd, bool) {
	c, ok := m.cmds.Load(pid)
	if ok {
		return c.(*execCmd), true
	}
	return nil, false
}

func (m *fileScheme) Start(pid Pid) error {
	c, ok := m.getCmdForPid(pid)
	if !ok {
		return errProcNotFound
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	err := c.Start()
	if err != nil {
		return fmt.Errorf("Cmd.Start: %w", err)
	}

	c.pid.Add(int64(c.Process.Pid))
	return nil
}

func (m *fileScheme) Signal(pid Pid, signal syscall.Signal) error {
	c, ok := m.getCmdForPid(pid)
	if !ok {
		return errProcNotFound
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Process == nil {
		return errProcNotRunning
	}
	err := syscall.Kill(int(c.Process.Pid), signal)
	if err != nil {
		return fmt.Errorf("syscall.Kill: %w", err)
	}
	return nil
}

func (m *fileScheme) StderrPipe(pid Pid) (io.ReadCloser, error) {
	c, ok := m.getCmdForPid(pid)
	if !ok {
		return nil, errProcNotFound
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	pipe, err := c.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("Cmd.StderrPipe: %w", err)
	}
	return pipe, err
}

func (m *fileScheme) StdinPipe(pid Pid) (io.WriteCloser, error) {
	c, ok := m.getCmdForPid(pid)
	if !ok {
		return nil, errProcNotFound
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	pipe, err := c.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("Cmd.StdinPipe: %w", err)
	}
	return pipe, err
}

func (m *fileScheme) StdoutPipe(pid Pid) (io.ReadCloser, error) {
	c, ok := m.getCmdForPid(pid)
	if !ok {
		return nil, errProcNotFound
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	pipe, err := c.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("Cmd.StdoutPipe: %w", err)
	}
	return pipe, err
}

func (m *fileScheme) Wait(pid Pid) error {
	c, ok := m.getCmdForPid(pid)
	if !ok {
		return errProcNotFound
	}
	defer m.cmds.Delete(pid)

	c.mu.Lock()
	defer c.mu.Unlock()

	err := c.Wait()
	if err != nil {
		return fmt.Errorf("Cmd.Wait: %w", err)
	}
	return err
}

func (m *fileScheme) NewPty() (Pty, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	// setup command
	pid, _ := m.Command(shell)
	cmd, _ := m.getCmdForPid(pid)
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
	cmd.SysProcAttr.Setctty = true
	// NOTE: this should probably be an option
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	// open master/slave files
	pty, tty, err := pty.Open()
	if err != nil {
		return Pty{}, fmt.Errorf("pty.Open: %v", err)
	}
	cmd.Stdout = tty
	cmd.Stderr = tty
	cmd.Stdin = tty

	// start process
	var startErr error
	if err := cmd.Start(); err != nil {
		startErr = multierr.Append(startErr, err)
		if err := pty.Close(); err != nil {
			startErr = multierr.Append(startErr, err)
		}
	}
	if err := tty.Close(); err != nil {
		startErr = multierr.Append(startErr, err)
	}
	if startErr != nil {
		return Pty{}, startErr
	}

	return Pty{
		Pid:    pid,
		Master: pty,
		Slave:  tty.Name(),
	}, nil
}

func (m *fileScheme) SetPtySize(p Pty, width, height int) error {
	ptyFile, ok := p.Master.(*os.File)
	if !ok {
		return fmt.Errorf("extraneous Pty: %#v", p)
	}

	err := pty.Setsize(ptyFile, &pty.Winsize{
		Rows: uint16(height),
		Cols: uint16(width),
	})
	if err != nil {
		return fmt.Errorf("pty.Setsize: %v", err)
	}
	return nil
}

func worker(ctx context.Context, wg *sync.WaitGroup, ch, workerCh chan string) {
	for {
		select {
		case <-ctx.Done():
			return
		case path := <-workerCh:
			dirTraversal(ctx, path, wg, ch, workerCh)
		}
	}
}

func dirTraversal(
	ctx context.Context, path string,
	wg *sync.WaitGroup, ch, workerCh chan string,
) error {
	defer wg.Done()

	dirNames, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	var ret error
	for _, info := range dirNames {
		p := filepath.Join(path, info.Name())
		if !info.IsDir() {
			// ensure dirTraversal returns
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ch <- p:
				continue
			}
		}

		wg.Add(1)
		select {
		// ensure dirTraversal returns
		case <-ctx.Done():
			wg.Done()
			return ctx.Err()
		case workerCh <- p:
		default:
			// the rest of workers are busy, keep going
			dirTraversal(ctx, p, wg, ch, workerCh)
		}
	}
	return ret
}

type listFilesIterator struct {
	ctx context.Context
	ch  chan string
}

func (l listFilesIterator) Next() (string, bool, error) {
	select {
	case <-l.ctx.Done():
		return "", false, l.ctx.Err()
	case path, ok := <-l.ch:
		return path, ok, nil
	}
}

func (m *fileScheme) ListFiles(ctx context.Context) (iterator.Iterator[string], error) {
	var wg sync.WaitGroup
	ch := make(chan string)
	workerCh := make(chan string)

	for i := 0; i < m.workers; i++ {
		go worker(ctx, &wg, ch, workerCh)
	}

	wg.Add(1)
	workerCh <- m.workspace.Path()

	go func() {
		wg.Wait()
		close(ch)
	}()

	return listFilesIterator{ctx: ctx, ch: ch}, nil
}

func (m *execCmd) Close() error {
	// NOTE do not lock here or else we risk deadlock
	// as any client could could call Wait and we would be holding
	// this execCmd lock undefinetly. Sending a signal will terminate
	// the process which then will force Wait to return, freeing the lock.
	if pid := m.pid.Load(); pid != 0 {
		err := syscall.Kill(int(pid), syscall.SIGKILL)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *fileScheme) Close() error {
	var ret error

	var copyCmds []*execCmd
	var keys []interface{}
	m.cmds.Range(func(key, cmd interface{}) bool {
		copyCmds = append(copyCmds, cmd.(*execCmd))
		keys = append(keys, key)
		return true
	})

	var wg sync.WaitGroup
	wg.Add(len(copyCmds))
	for _, cmd := range copyCmds {
		go func(cmd *execCmd) {
			defer wg.Done()
			_ = cmd.Close()
		}(cmd)
	}
	wg.Wait()

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
