package workspace

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	bluectx "github.com/ernestrc/blue/context"
	"github.com/ernestrc/blue/logging"
	multierr "github.com/ernestrc/go-multierror"
	"github.com/ernestrc/sensible/find"
	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/api/config"
	schemeapi "unstable.build/go-tui/api/scheme"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/term/pty"
)

const (
	// FileScheme represents the local file URL scheme.
	FileScheme         = "file"
	watcherWaitTimeout = 2 * time.Minute
)

// NewFileScheme returns a Scheme that manages resources
// on the local file system.
func NewFileScheme(
	ctx context.Context, cfg config.Config, workspace workspaceapi.URI,
) (schemeapi.Scheme, error) {
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

// OpenFile opens the given filename using the default current working
// directory's file scheme. This is preferrable over os.OpenFile, because
// it does expansion of paths (i.e. ~ is expanded to the current user's home directory).
func OpenFile(filename string, flag int, perm os.FileMode) (workspaceapi.File, error) {
	cwdURI, _ := workspaceapi.CurrentUserHostURI(".")
	fs, err := NewFileScheme(context.Background(), config.NopConfig(), cwdURI)
	if err != nil {
		return nil, fmt.Errorf("file scheme: %v", err)
	}
	f, werr := fs.Open(filename, flag, perm)
	if werr != nil {
		return nil, werr.ToError()
	}
	return f, nil
}

// ReeadFile reads the file named by filename and returns the contents.
// See os.ReadFile for more details.
func ReadFile(filename string) ([]byte, error) {
	f, err := OpenFile(filename, os.O_RDONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	return ioutil.ReadAll(f)
}

type fileScheme struct {
	osStat     func(path string) (os.FileInfo, error)
	getUser    func() (*user.User, error)
	lookupUser func(string) (*user.User, error)
	workspace  workspaceapi.URI
	ctx        context.Context
	cancelCtx  func()
	cmds       sync.Map // map[workspaceapi.Pid]struct{}

	// This is important to prevent runtime finalizers
	// running on files that are garbage collected on host
	// but that clients hold references to.
	//
	// Technically we could leak files if clients
	// never close files, but once Server is garbage
	// collected, all files that are orhpaned will be closed.
	files sync.Map // map[uintptr]workspaceapi.File
}

func (p *fileScheme) init(cfg config.Config, workspace workspaceapi.URI) error {
	if workspace.Host() != "" || workspace.User() != "" || workspace.Scheme() != FileScheme {
		return errors.New("invalid file URI")
	}
	workspacewd := workspace.Path()
	fs, err := p.osStat(workspacewd)
	if err != nil {
		return err
	}
	if !fs.IsDir() {
		return fmt.Errorf("workspaceapi.URI does not refer to a directory: %s", workspace.String())
	}
	p.workspace = workspace
	p.ctx, p.cancelCtx = context.WithCancel(context.Background())
	return nil
}

func (p *fileScheme) Open(path string, flag int, perm os.FileMode) (workspaceapi.File, *workspaceapi.Error) {
	var err error
	path, err = workspaceapi.ExpandPath(path, p.getUserOrLookup, func() (string, error) {
		return p.workspace.Path(), nil
	})
	if err != nil {
		return nil, workspaceapi.NopError(err)
	}
	f, err := os.OpenFile(path, flag, perm)
	if err != nil {
		return nil, &workspaceapi.Error{
			Err:          err,
			IsPermission: os.IsPermission(err),
			IsExist:      os.IsExist(err),
			IsNotExist:   os.IsNotExist(err),
		}
	}

	ret := &fileSchemeFile{File: f, p: p}
	p.files.Store(f.Fd(), ret)

	return ret, nil
}

func (p *fileScheme) NewFile(fd uintptr, filename string) workspaceapi.File {
	f, ok := p.files.Load(fd)
	if !ok {
		return nil
	}
	return f.(workspaceapi.File)
}

func (p *fileScheme) Remove(path string) error {
	var err error
	path, err = workspaceapi.ExpandPath(path, p.getUserOrLookup, func() (string, error) {
		return p.workspace.Path(), nil
	})
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func (p *fileScheme) Rename(old, new string) error {
	var err error
	old, err = workspaceapi.ExpandPath(old, p.getUserOrLookup, func() (string, error) {
		return p.workspace.Path(), nil
	})
	if err != nil {
		return err
	}
	new, err = workspaceapi.ExpandPath(new, p.getUserOrLookup, func() (string, error) {
		return p.workspace.Path(), nil
	})
	if err != nil {
		return err
	}
	return os.Rename(old, new)
}

func (p *fileScheme) Stat(path string) (os.FileInfo, error) {
	var err error
	path, err = workspaceapi.ExpandPath(path, p.getUserOrLookup, func() (string, error) {
		return p.workspace.Path(), nil
	})
	if err != nil {
		return nil, err
	}
	return os.Stat(path)
}

func (p *fileScheme) ReadDir(name string) ([]os.DirEntry, error) {
	var err error
	name, err = workspaceapi.ExpandPath(name, p.getUserOrLookup, func() (string, error) {
		return p.workspace.Path(), nil
	})
	if err != nil {
		return nil, err
	}
	return os.ReadDir(name)
}

func (p *fileScheme) Lstat(path string) (os.FileInfo, error) {
	var err error
	path, err = workspaceapi.ExpandPath(path, p.getUserOrLookup, func() (string, error) {
		return p.workspace.Path(), nil
	})
	if err != nil {
		return nil, err
	}
	return os.Lstat(path)
}

func (p *fileScheme) ReadLink(path string) (string, error) {
	var err error
	path, err = workspaceapi.ExpandPath(path, p.getUserOrLookup, func() (string, error) {
		return p.workspace.Path(), nil
	})
	if err != nil {
		return "", err
	}
	return os.Readlink(path)
}

func (p *fileScheme) getUserOrLookup() (*user.User, error) {
	if p.workspace.User() == "" {
		return p.getUser()
	}
	username := p.workspace.User()
	return p.lookupUser(username)
}

func (p *fileScheme) URI(path string) (workspaceapi.URI, error) {
	absPath, err := workspaceapi.ExpandPath(path, p.getUserOrLookup, func() (string, error) {
		return p.workspace.Path(), nil
	})
	if err != nil {
		return workspaceapi.URI{}, err
	}
	return makeLocalURI(absPath)
}

func (p *fileScheme) log(level log.Level, msg string, args ...interface{}) {
	log.
		WithField(logging.KeyClass, "fileScheme").
		WithField("URI", p.workspace.String()).
		Logf(level, msg, args...)
}

func (p *fileScheme) StartCommand(ctx context.Context, cmd workspaceapi.Cmd) (
	workspaceapi.Pid, error,
) {
	var err error
	path := cmd.Path
	if filepath.Base(cmd.Path) == cmd.Path {
		path, err = find.Executable(cmd.Path)
		if err != nil {
			p.log(log.WarnLevel,
				"find.Executable: could not find executable of '%s' in path. "+
					"Falling back to shell expanding it: %v", cmd.Path, err)
			path = cmd.Path
		}
	}
	// ensure that if file scheme is closed, all commands are cleaned up
	var cancelFn func()
	ctx, cancelFn = context.WithCancel(ctx)
	ctx = bluectx.First(p.ctx, ctx)
	stdcmd := exec.CommandContext(ctx, path, cmd.Args...)
	stdcmd.Dir = p.workspace.Path()
	stdcmd.Env = cmd.Env
	stdcmd.Stdout = cmd.Stdout
	stdcmd.Stderr = cmd.Stderr
	stdcmd.Stdin = cmd.Stdin
	stdcmd.SysProcAttr = cmd.SysProcAttr

	err = stdcmd.Start()
	if err != nil {
		cancelFn()
		return 0, fmt.Errorf("start: %w", err)
	}

	pid := workspaceapi.Pid(stdcmd.Process.Pid)
	go func() {
		defer cancelFn()
		defer func() {
			p.cmds.Delete(pid)
		}()

		start := time.Now()
		p.log(log.TraceLevel, "exec.Command: Wait: pid=%d", stdcmd.Process.Pid)
		err := stdcmd.Wait()
		p.log(log.DebugLevel, "exec.Command: Wait returned: cmd=%v pid=%d, err=%v"+
			", duration=%s",
			stdcmd.Args, stdcmd.Process.Pid, err, time.Since(start).String())

		// set a timeout to how long we wait for a watcher
		// to drain the error. This is just to avoid
		// buggy watchers to cause this goroutine to block forever,
		// so the timeout should be in the order of minutes.
		// use a new context so the cancelation of the command doesn't
		// prevent watcher from being called.
		ctx, cancel := context.WithTimeout(p.ctx,
			watcherWaitTimeout)
		defer cancel()

		if cmd.Watcher != nil && cmd.Watcher.Watch() != nil {
			select {
			case cmd.Watcher.Watch() <- err:
			case <-ctx.Done():
				p.log(log.WarnLevel, "could not deliver error to watcher chan: "+
					"watcher not ready for too long")
			}
		}
	}()

	p.log(log.DebugLevel, "exec.Command: (%#v, pid=%d)", cmd, stdcmd.Process.Pid)

	p.cmds.Store(pid, struct{}{})

	return pid, nil
}

func (p *fileScheme) Signal(pid workspaceapi.Pid, signal syscall.Signal) error {
	_, ok := p.cmds.Load(pid)
	if !ok {
		return errProcNotFound
	}

	err := syscall.Kill(int(pid), signal)
	if err != nil {
		return fmt.Errorf("syscall.Kill: %w", err)
	}
	return nil
}

func (p *fileScheme) NewPty(ctx context.Context) (workspaceapi.Pty, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	// setup command
	cmd := workspaceapi.Cmd{
		Path: shell,
		// NOTE: this should probably be an option
		Env: append(os.Environ(), "TERM=xterm-256color"),
		SysProcAttr: &syscall.SysProcAttr{
			Setsid:  true,
			Setctty: true,
		},
	}

	// open master/slave files
	pty, tty, err := pty.Open()
	if err != nil {
		return workspaceapi.Pty{}, fmt.Errorf("pty.Open: %v", err)
	}

	// NOTE: consider setting a better default pty size
	// so when we open a terminal there's no race between the plugin
	// setting the size and the tui component drawing to the screen.
	cmd.Stdout = tty
	cmd.Stderr = tty
	cmd.Stdin = tty

	// start process
	var retErr error
	pid, err := p.StartCommand(ctx, cmd)
	if err != nil {
		retErr = multierr.Append(retErr, err)
		if err := pty.Close(); err != nil {
			retErr = multierr.Append(retErr, err)
		}
	}
	if err := tty.Close(); err != nil {
		retErr = multierr.Append(retErr, err)
	}
	if retErr != nil {
		return workspaceapi.Pty{}, retErr
	}

	ret := &fileSchemeFile{File: pty, p: p}
	p.files.Store(pty.Fd(), ret)

	return workspaceapi.Pty{
		Pid:    pid,
		Master: ret,
		Slave:  tty.Name(),
	}, nil
}

func (p *fileScheme) SetPtySize(pp workspaceapi.Pty, width, height int) error {
	ptyFile, ok := pp.Master.(*fileSchemeFile)
	if !ok {
		return fmt.Errorf("extraneous Pty: %#v", pp)
	}

	err := pty.Setsize(ptyFile.File, &pty.Winsize{
		Rows: uint16(height),
		Cols: uint16(width),
	})
	if err != nil {
		return fmt.Errorf("pty.Setsize: %v", err)
	}
	return nil
}

func (p *fileScheme) Close() error {
	p.cancelCtx()
	return nil
}

// enables overriding Close to delete from map.
type fileSchemeFile struct {
	*os.File
	p *fileScheme
}

func (f *fileSchemeFile) Close() error {
	f.p.files.Delete(f.Fd())
	return f.File.Close()
}

func makeLocalURI(path string) (workspaceapi.URI, error) {
	uriStr := "file://" + path
	return workspaceapi.ParseURI(uriStr)
}
