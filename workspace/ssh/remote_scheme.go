package ssh

import (
	"context"
	"errors"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
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

type state struct {
	lastSessionError error
	scheme           workspace.Scheme
}

// wraps another workspace.Scheme to be resilient against
// intermitent connection failures
type remoteScheme struct {
	closeChan chan struct{}
	ctx       context.Context
	cancelCtx func()
	currState atomic.Value

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

			// close hook could be called multiple times
			scheme, err := connect(ctx, uri, func(err error) {
				defer cancel()

				select {
				case <-ctx.Done():
					return
				default:
				}
				logger.Warnf("lost connectivity to %s: %s", uri, err)
				s.setError(logger, err)
				return
			})

			prevState := s.currState.Swap(state{scheme: scheme,
				lastSessionError: err})
			if prevState != nil && prevState.(state).scheme != nil {
				err := prevState.(state).scheme.Close()
				logger.Tracef("closed previous remote scheme: %v", err)
			}

			if !initSema {
				// unlock initialization semaphore
				// so constructor returns after first attempt to connect
				sema.Unlock()
				initSema = true
			}

			if err != nil {
				cancel()
				logger.Warnf("failed to connect to %s: %s", uri, err)
				return true, err
			}

			logger.Infof("connected to %s", uri)

			select {
			case <-ctx.Done():
				logger.Debugf("connection context to %s is done", uri)
				currState := s.currState.Load()
				var err error
				if currState != nil {
					st := currState.(state)
					if st.lastSessionError == nil {
						err = errors.New("lost connection to remote")
						logger.Warn(err)
						s.setError(logger, err)
					} else {
						err = st.lastSessionError
					}
				}
				return true, err
			case <-closeChan:
				return false, nil
			}
		})

	logger.Debugf("stopped trying to re-connect to remote %s", uri)
}

func (s *remoteScheme) setError(logger *log.Entry, err error) {
	prevState := s.currState.Swap(state{lastSessionError: err})
	if prevState != nil && prevState.(state).scheme != nil {
		err := prevState.(state).scheme.Close()
		logger.Tracef("closed previous remote scheme: %v", err)
	}
}

func newRemoteScheme(
	ctx context.Context, connect connectSchemeFn, uri workspaceapi.URI,
) workspace.Scheme {
	ret := &remoteScheme{
		closeChan: make(chan struct{}),
	}
	ret.currState.Store(state{lastSessionError: errors.New("not connected yet")})
	ret.ctx, ret.cancelCtx = context.WithCancel(ctx)

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
	currState := s.currState.Load().(state)
	err = currState.lastSessionError
	scheme = currState.scheme
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
	prevState := s.currState.Swap(state{lastSessionError: errors.New("remote closed")})
	if prevState != nil && prevState.(state).scheme != nil {
		// no need to close files before closing scheme to avoid
		// closing connection before telling the remote workspace to close
		// files: remote workspace process is going to do that anyway
		// as the process will be shutdown.
		close(s.closeChan)
		prevState.(state).scheme.Close()
	}
	s.cancelCtx()
	return
}
