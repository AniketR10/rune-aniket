package ssh

import (
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/ernestrc/blue/logging"
	"github.com/ernestrc/blue/retry"
	"github.com/ernestrc/go-tui/debug"
	"github.com/ernestrc/go-tui/workspace"
)

// retry forever, starting with every 10 ms up until every 5s
var retryStrategy = retry.ExponentialStrategy(10*time.Millisecond, 5*time.Second)

type connectSchemeFn func(uri workspace.URI, closeHook func(error)) (workspace.Scheme, error)

// wraps another workspace.Scheme to be resilient to intermitent connection failures
type remoteScheme struct {
	mu               sync.Mutex
	closeChan        chan struct{}
	lastSessionError error
	scheme           workspace.Scheme
}

func (s *remoteScheme) maintainConnection(
	connect connectSchemeFn, uri workspace.URI,
	closeChan chan struct{}, sema *sync.Mutex,
) {
	logger := debug.StandardLogger().WithField(logging.KeyClass, "ssh")

	var unlockedSema bool
	for {
		ctx, cancel := context.WithCancel(context.Background())

		retry.Retry(ctx, retryStrategy, func(ctx context.Context) (bool, error) {
			logger.Debugf("attempting to connect to %s", uri)

			// block Scheme API until we're connected
			s.mu.Lock()
			if !unlockedSema {
				sema.Unlock()
				unlockedSema = true
			}
			if s.scheme != nil {
				_ = s.scheme.Close()
			}

			var canceled bool // close hook could be called multiple times
			s.scheme, s.lastSessionError = connect(uri, func(err error) {
				s.mu.Lock()
				defer s.mu.Unlock()
				if canceled {
					return
				}
				canceled = true
				s.lastSessionError = err
				logger.Warnf("lost connectivity to %s: %s", uri, err)
				cancel() // unlock outer loop to retry connecting
			})

			if s.lastSessionError != nil {
				logger.Warnf("failed to connect to %s: %s", uri, s.lastSessionError)
				s.mu.Unlock()
				return true, s.lastSessionError
			}

			logger.Infof("connected to %s", uri)

			s.mu.Unlock()

			select {
			case <-ctx.Done():
				s.mu.Lock()
				defer s.mu.Unlock()
				return true, s.lastSessionError
			case <-closeChan:
				return false, nil
			}
		})

		// wait until connection is closed either via Close
		// or due to connectivity issues
		select {
		case <-ctx.Done():
		case <-closeChan:
			return
		}
	}
}

func newRemoteScheme(connect connectSchemeFn, uri workspace.URI) workspace.Scheme {
	ret := &remoteScheme{
		closeChan:        make(chan struct{}),
		lastSessionError: errors.New("not connected yet"),
	}
	var sema sync.Mutex
	sema.Lock()
	go ret.maintainConnection(connect, uri, ret.closeChan, &sema)
	sema.Lock()
	defer sema.Unlock()
	return ret
}

func (s *remoteScheme) state() (err error, scheme workspace.Scheme) {
	s.mu.Lock()
	defer s.mu.Unlock()
	err = s.lastSessionError
	scheme = s.scheme
	return
}

func (s *remoteScheme) Open(path string, flag int, perm os.FileMode) (workspace.File, *workspace.Error) {
	err, scheme := s.state()
	if err != nil {
		werr := workspace.NopError(err)
		return nil, werr
	}
	f, werr := scheme.Open(path, flag, perm)
	if werr != nil {
		return nil, werr
	}
	return f, nil
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

func (s *remoteScheme) URI(path string) (workspace.URI, error) {
	panic("unused")
}

func (s *remoteScheme) Command(name string, arg ...string) (workspace.Pid, error) {
	err, scheme := s.state()
	if err != nil {
		return 0, err
	}
	return scheme.Command(name, arg...)
}

func (s *remoteScheme) Start(p workspace.Pid) error {
	err, scheme := s.state()
	if err != nil {
		return err
	}
	return scheme.Start(p)
}

func (s *remoteScheme) Signal(p workspace.Pid, signal syscall.Signal) error {
	err, scheme := s.state()
	if err != nil {
		return err
	}
	return scheme.Signal(p, signal)
}

func (s *remoteScheme) StderrPipe(p workspace.Pid) (io.ReadCloser, error) {
	err, scheme := s.state()
	if err != nil {
		return nil, err
	}
	return scheme.StderrPipe(p)
}

func (s *remoteScheme) StdinPipe(p workspace.Pid) (io.WriteCloser, error) {
	err, scheme := s.state()
	if err != nil {
		return nil, err
	}
	return scheme.StdinPipe(p)
}

func (s *remoteScheme) StdoutPipe(p workspace.Pid) (io.ReadCloser, error) {
	err, scheme := s.state()
	if err != nil {
		return nil, err
	}
	return scheme.StdoutPipe(p)
}

func (s *remoteScheme) Wait(p workspace.Pid) error {
	err, scheme := s.state()
	if err != nil {
		return err
	}
	return scheme.Wait(p)
}

func (s *remoteScheme) Close() (ret error) {
	s.mu.Lock()
	s.lastSessionError = errors.New("remote closed")
	scheme := s.scheme
	s.scheme = nil
	s.mu.Unlock()
	if scheme != nil {
		scheme.Close()
		close(s.closeChan)
	}
	return
}
