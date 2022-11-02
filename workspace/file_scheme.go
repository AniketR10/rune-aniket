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
	log "github.com/sirupsen/logrus"
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
	quitCh     chan struct{}
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
	p.quitCh = make(chan struct{})
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
	debug.StandardLogger().
		WithField(logging.KeyClass, "fileScheme").
		Debugf("init: initialized with %d ListFiles workers", p.workers)

	return nil
}

func (p *fileScheme) Open(path string, flag int, perm os.FileMode) (File, *Error) {
	var err error
	path, err = ExpandPath(path, p.getUserOrLookup, func() (string, error) {
		return p.workspace.Path(), nil
	})
	if err != nil {
		return nil, NopError(err)
	}
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
	var err error
	path, err = ExpandPath(path, p.getUserOrLookup, func() (string, error) {
		return p.workspace.Path(), nil
	})
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func (p *fileScheme) Rename(old, new string) error {
	var err error
	old, err = ExpandPath(old, p.getUserOrLookup, func() (string, error) {
		return p.workspace.Path(), nil
	})
	if err != nil {
		return err
	}
	new, err = ExpandPath(new, p.getUserOrLookup, func() (string, error) {
		return p.workspace.Path(), nil
	})
	if err != nil {
		return err
	}
	return os.Rename(old, new)
}

func (p *fileScheme) Stat(path string) (os.FileInfo, error) {
	var err error
	path, err = ExpandPath(path, p.getUserOrLookup, func() (string, error) {
		return p.workspace.Path(), nil
	})
	if err != nil {
		return nil, err
	}
	return os.Stat(path)
}

func (p *fileScheme) Lstat(path string) (os.FileInfo, error) {
	var err error
	path, err = ExpandPath(path, p.getUserOrLookup, func() (string, error) {
		return p.workspace.Path(), nil
	})
	if err != nil {
		return nil, err
	}
	return os.Lstat(path)
}

func (p *fileScheme) ReadLink(path string) (string, error) {
	var err error
	path, err = ExpandPath(path, p.getUserOrLookup, func() (string, error) {
		return p.workspace.Path(), nil
	})
	if err != nil {
		return "", err
	}
	return os.Readlink(path)
}

func (p *fileScheme) getUserOrLookup() (*user.User, error) {
	if p.workspace.parsed.User == nil {
		return p.getUser()
	}
	username := p.workspace.parsed.User.Username()
	return p.lookupUser(username)
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

func (p *fileScheme) log(level log.Level, msg string, args ...interface{}) {
	debug.StandardLogger().
		WithField(logging.KeyClass, "fileScheme").
		WithField("URI", p.workspace.String()).
		Logf(level, msg, args...)
}

func (p *fileScheme) Command(name string, arg ...string) (Pid, error) {
	path, err := find.Executable(name)
	if err != nil {
		p.log(log.WarnLevel,
			"find.Executable: could not find executable of '%s' in path. Falling back to shell expanding it: %v",
			name, err)
		path = name
	}
	cmd := exec.Command(path, arg...)
	cmd.Dir = p.workspace.Path()
	nextPid := atomic.AddInt32(&p.nextPid, 1)

	p.log(log.DebugLevel, "exec.Command: (%#v, pid=%d)", cmd, nextPid)

	p.cmds.Store(Pid(nextPid), &execCmd{Cmd: cmd})
	return Pid(nextPid), nil
}

func (p *fileScheme) getCmdForPid(pid Pid) (*execCmd, bool) {
	c, ok := p.cmds.Load(pid)
	if ok {
		return c.(*execCmd), true
	}
	return nil, false
}

func (p *fileScheme) Start(pid Pid) error {
	c, ok := p.getCmdForPid(pid)
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

func (p *fileScheme) Signal(pid Pid, signal syscall.Signal) error {
	c, ok := p.getCmdForPid(pid)
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

func (p *fileScheme) StderrPipe(pid Pid) (io.ReadCloser, error) {
	c, ok := p.getCmdForPid(pid)
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

func (p *fileScheme) StdinPipe(pid Pid) (io.WriteCloser, error) {
	c, ok := p.getCmdForPid(pid)
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

func (p *fileScheme) StdoutPipe(pid Pid) (io.ReadCloser, error) {
	c, ok := p.getCmdForPid(pid)
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

func (p *fileScheme) Wait(pid Pid) error {
	c, ok := p.getCmdForPid(pid)
	if !ok {
		return errProcNotFound
	}
	defer p.cmds.Delete(pid)

	c.mu.Lock()
	defer c.mu.Unlock()

	err := c.Wait()
	if err != nil {
		return fmt.Errorf("Cmd.Wait: %w", err)
	}
	return err
}

func (p *fileScheme) NewPty() (Pty, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	// setup command
	pid, _ := p.Command(shell)
	cmd, _ := p.getCmdForPid(pid)
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

func (p *fileScheme) SetPtySize(pp Pty, width, height int) error {
	ptyFile, ok := pp.Master.(*os.File)
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

func traverseDirWorker(
	ctx context.Context, wg *sync.WaitGroup,
	ch, workerCh chan string, cwd string, mu *sync.Mutex, err *error,
	quitCh chan struct{},
) {
	for {
		select {
		case <-quitCh:
			return
		case <-ctx.Done():
			return
		case path := <-workerCh:
			dirErr := dirTraversal(ctx, cwd, path, wg, ch, workerCh, quitCh)
			if dirErr != nil {
				mu.Lock()
				*err = multierr.Append(*err, dirErr)
				mu.Unlock()
			}
		}
	}
}

func dirTraversal(
	ctx context.Context, cwd, dirname string,
	wg *sync.WaitGroup, ch, workerCh chan string,
	quitCh chan struct{},
) error {
	defer wg.Done()
	absPath := filepath.Join(cwd, dirname)

	dirNames, err := os.ReadDir(absPath)
	if err != nil {
		return err
	}

	var ret error
	for _, info := range dirNames {
		path := filepath.Join(dirname, info.Name())
		if !info.IsDir() {
			// ensure dirTraversal returns
			select {
			case <-quitCh:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			case ch <- path:
				continue
			}
		}

		wg.Add(1)
		select {
		// ensure dirTraversal returns
		case <-ctx.Done():
			wg.Done()
			return ctx.Err()
		case workerCh <- path:
		default:
			// the rest of workers are busy, keep going
			err := dirTraversal(ctx, cwd, path, wg, ch, workerCh, quitCh)
			if err != nil {
				ret = multierr.Append(ret, err)
			}
		}
	}
	return ret
}

type listFilesIterator struct {
	mu  sync.Mutex
	err error
	ctx context.Context
	ch  chan string
}

func (l *listFilesIterator) Next() (string, bool) {
	select {
	case <-l.ctx.Done():
		l.mu.Lock()
		defer l.mu.Unlock()
		l.err = multierr.Append(l.err, l.ctx.Err())
		return "", false
	case path, ok := <-l.ch:
		return path, ok
	}
}

func (l *listFilesIterator) Err() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.err == nil {
		return l.ctx.Err()
	}
	if l.ctx.Err() == nil {
		return l.err
	}
	return multierr.Append(l.err, l.ctx.Err())
}

func (p *fileScheme) ListFiles(ctx context.Context) (iterator.Iterator[string], error) {
	var wg sync.WaitGroup
	ch := make(chan string)
	workerCh := make(chan string)
	errors := make([]error, p.workers)
	iterator := &listFilesIterator{ctx: ctx, ch: ch}

	for i := 0; i < p.workers; i++ {
		go traverseDirWorker(ctx, &wg, ch, workerCh,
			p.workspace.Path(), &iterator.mu, &errors[i], p.quitCh)
	}

	wg.Add(1)
	workerCh <- "."

	go func() {
		wg.Wait()
		iterator.mu.Lock()
		defer iterator.mu.Unlock()
		for _, err := range errors {
			if err != nil {
				iterator.err = multierr.Append(iterator.err, err)
			}
		}
		close(ch)
	}()

	return iterator, nil
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

func (p *fileScheme) Close() error {
	var ret error

	var copyCmds []*execCmd
	var keys []interface{}
	p.cmds.Range(func(key, cmd interface{}) bool {
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
		p.cmds.Delete(key)
	}
	close(p.quitCh)
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
