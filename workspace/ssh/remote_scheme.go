package ssh

import (
	"context"
	"errors"
	"os"
	"runtime"
	"sync"
	"syscall"
	"time"

	bluectx "github.com/ernestrc/blue/context"
	"github.com/ernestrc/blue/logging"
	"github.com/ernestrc/blue/retry"
	log "github.com/sirupsen/logrus"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/workspace"
)

var (
	retryStrategy = retry.ExponentialStrategy(100*time.Millisecond, 5*time.Second)
)

type connectSchemeFn func(ctx context.Context,
	uri workspaceapi.URI, closeHook func(error)) (workspace.Scheme, error)

// wraps another workspace.Scheme to be resilient against intermitent connection failures
type remoteScheme struct {
	locker           sync.Locker
	closeChan        chan struct{}
	lastSessionError error
	scheme           workspace.Scheme
	ctx              context.Context
	cancelCtx        func()

	// NOTE this is not a regular map because
	// otherwise we need to worry about synchronizing deletes
	// on runtime finalizer's
	files sync.Map
}

func (s *remoteScheme) maintainConnection(
	connect connectSchemeFn, uri workspaceapi.URI,
	closeChan chan struct{}, sema *sync.Mutex,
) {
	logger := log.WithField(logging.KeyClass, "ssh")

	var initSema bool
	retry.Retry(s.ctx, retryStrategy,
		func(ctx context.Context) (bool, error) {

			// cancel if connection is closed for some reason
			// so we can retry below
			ctx, cancel := context.WithCancel(ctx)

			logger.Debugf("attempting to connect to %s", uri)

			// block Scheme API until we're connected
			s.locker.Lock()
			if !initSema {
				// unlock initialization semaphore
				// so next call to Scheme API blocks until
				// we're connected
				sema.Unlock()
				initSema = true
			}
			if s.scheme != nil {
				err := s.scheme.Close()
				logger.Tracef("closed previous remote scheme: %v", err)
			}

			// close hook could be called multiple times
			s.scheme, s.lastSessionError = connect(ctx, uri, func(err error) {
				defer cancel()

				select {
				case <-ctx.Done():
					return
				default:
				}
				logger.Warnf("lost connectivity to %s: %s", uri, err)
				s.locker.Lock()
				defer s.locker.Unlock()
				s.lastSessionError = err
				return
			})

			if s.lastSessionError != nil {
				cancel()
				logger.Warnf("failed to connect to %s: %s", uri, s.lastSessionError)
				s.locker.Unlock()
				return true, s.lastSessionError
			}

			logger.Infof("connected to %s", uri)

			s.locker.Unlock()

			select {
			case <-ctx.Done():
				logger.Debugf("connection context to %s is done", uri)
				s.locker.Lock()
				defer s.locker.Unlock()
				if s.lastSessionError == nil {
					s.lastSessionError = errors.New("lost connection to remote")
					logger.Warn(s.lastSessionError)
				}
				return true, s.lastSessionError
			case <-closeChan:
				return false, nil
			}
		})

	logger.Debugf("stopped trying to re-connect to remote %s", uri)
}

func newRemoteScheme(
	ctx context.Context, connect connectSchemeFn, uri workspaceapi.URI,
) workspace.Scheme {
	ret := &remoteScheme{
		closeChan:        make(chan struct{}),
		lastSessionError: errors.New("not connected yet"),
	}
	var ok bool
	ret.locker, ok = workspace.LockerFromContext(ctx)
	if !ok {
		ret.locker = new(sync.Mutex)
		ret.locker.Lock() // make compatible with passing locker in ctx
		defer ret.locker.Unlock()
	}
	ret.ctx, ret.cancelCtx = context.WithCancel(ctx)

	ret.locker.Unlock()
	defer ret.locker.Lock()

	// ensure that we return once we have attempted
	// to connect at least once.
	var sema sync.Mutex
	sema.Lock()
	go ret.maintainConnection(connect, uri, ret.closeChan, &sema)
	sema.Lock()
	defer sema.Unlock()
	return ret
}

// this should only be called from within event loop,
// otherwhise need to sync first with locker.
func (s *remoteScheme) state() (err error, scheme workspace.Scheme) {
	err = s.lastSessionError
	scheme = s.scheme
	return
}

func (s *remoteScheme) Open(path string, flag int, perm os.FileMode) (
	workspaceapi.File, *workspaceapi.Error,
) {
	err, scheme := s.state()
	if err != nil {
		werr := workspaceapi.NopError(err)
		return nil, werr
	}
	f, werr := scheme.Open(path, flag, perm)
	if werr != nil {
		return nil, werr
	}
	// clear workspace client finalizer
	// so we can manage lifecycle manually,
	// accross clients of the remote workspace,
	// as we potentially recycle through reconnections
	runtime.SetFinalizer(f, nil)
	rf := newRemoteFile(s, f.Fd(), f.Name())
	s.files.Store(rf.Fd(), rf)
	return rf, nil
}

func (s *remoteScheme) NewFile(fd uintptr, filename string) workspaceapi.File {
	f, _ := s.files.Load(fd)
	return f.(workspaceapi.File)
}

func (s *remoteScheme) Remove(path string) error {
	err, scheme := s.state()
	if err != nil {
		return err
	}
	return scheme.Remove(path)
}

func (s *remoteScheme) Rename(oldpath, newpath string) error {
	err, scheme := s.state()
	if err != nil {
		return err
	}
	return scheme.Rename(oldpath, newpath)
}

func (s *remoteScheme) Stat(path string) (os.FileInfo, error) {
	err, scheme := s.state()
	if err != nil {
		return nil, err
	}
	return scheme.Stat(path)
}

func (s *remoteScheme) Lstat(path string) (os.FileInfo, error) {
	err, scheme := s.state()
	if err != nil {
		return nil, err
	}
	return scheme.Lstat(path)
}

func (s *remoteScheme) ReadLink(path string) (string, error) {
	err, scheme := s.state()
	if err != nil {
		return "", err
	}
	return scheme.ReadLink(path)
}

func (s *remoteScheme) URI(path string) (workspaceapi.URI, error) {
	panic("unused")
}

func (s *remoteScheme) StartCommand(ctx context.Context, cmd workspaceapi.Cmd) (
	workspaceapi.Pid, error,
) {
	err, scheme := s.state()
	if err != nil {
		return 0, err
	}
	return scheme.StartCommand(bluectx.First(s.ctx, ctx), cmd)
}

func (s *remoteScheme) Signal(p workspaceapi.Pid, signal syscall.Signal) error {
	err, scheme := s.state()
	if err != nil {
		return err
	}
	return scheme.Signal(p, signal)
}

func (s *remoteScheme) NewPty(ctx context.Context) (workspaceapi.Pty, error) {
	err, scheme := s.state()
	if err != nil {
		return workspaceapi.Pty{}, err
	}
	pty, err := scheme.NewPty(bluectx.First(s.ctx, ctx))
	if err == nil {
		runtime.SetFinalizer(pty.Master, nil)
		f := newRemoteFile(s, pty.Master.Fd(), pty.Master.Name())
		s.files.Store(f.Fd(), f)
		pty.Master = f
	}
	return pty, err
}

func (s *remoteScheme) ReadDir(name string) ([]os.DirEntry, error) {
	err, scheme := s.state()
	if err != nil {
		return nil, err
	}
	return scheme.ReadDir(name)
}

func (s *remoteScheme) SetPtySize(pty workspaceapi.Pty, width, height int) error {
	err, scheme := s.state()
	if err != nil {
		return err
	}
	// unwrap for underlying scheme to avoid unexpected type assertions panics
	pty.Master = scheme.NewFile(pty.Master.Fd(), pty.Master.Name())
	// file is transient, do not Close on GC
	runtime.SetFinalizer(pty.Master, nil)
	return scheme.SetPtySize(pty, width, height)
}

func (s *remoteScheme) Close() (ret error) {
	s.lastSessionError = errors.New("remote closed")
	scheme := s.scheme
	s.scheme = nil
	// no need to close files before closing scheme to avoid
	// closing connection before telling the remote workspace to close
	// files: remote workspace process is going to do that anyway
	// as the process will be shutdown.
	if scheme != nil {
		close(s.closeChan)
		scheme.Close()
	}
	s.cancelCtx()
	return
}
