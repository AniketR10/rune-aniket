package ssh

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	os "os"
	"os/user"
	"path/filepath"
	"time"

	multierr "github.com/ernestrc/go-multierror"
	"github.com/ernestrc/go-tui/config"
	"github.com/ernestrc/go-tui/workspace"
	workspacepb "github.com/ernestrc/go-tui/workspace/rpc"
	"google.golang.org/grpc"
)

const (
	// Scheme represents the URL scheme that this package implements
	Scheme = "ssh"
)

// New returns a workspace.Scheme capable of managing
// files over an ssh connection.
func New(cfg config.Config, uri workspace.URI) (workspace.Scheme, error) {
	return newScheme(cfg, uri)
}

type remote interface {
	NewSession() (workspace.Executor, error)
	Close() error
}

type scheme struct {
	cfg      sshConfig
	hostPort string
	user     string
	homedir  string
	basePath string

	getUser         func() (*user.User, error)
	remoteFn        func(sshConfig, workspace.URI) (remote, error)
	connectSchemeFn connectSchemeFn

	workspace.Scheme
}

func newScheme(ccfg config.Config, uri workspace.URI) (*scheme, error) {
	ret := new(scheme)

	cc, err := fromConfig(ccfg)
	if err != nil {
		return nil, err
	}

	ret.getUser = user.Current
	if cc.command == "" {
		ret.remoteFn = newStdRemote
	} else {
		ret.remoteFn = newProcRemote
	}

	ret.connectSchemeFn = ret.connectScheme
	err = ret.init(cc, uri)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func parseWorkspaceURI(u workspace.URI, getUser func() (*user.User, error)) (
	username, homedir, hostPort, basePath string, err error,
) {
	username = u.User()

	// respect empty username for URI creation
	// but we need its implicit value for ~ expansion to work
	usernameForHomeDir := username
	if usernameForHomeDir == "" {
		var u *user.User
		u, err = getUser()
		if err != nil {
			return
		}
		usernameForHomeDir = u.Username
	}
	// I doubt we'll ever ssh into a non-linux host
	homedir = filepath.Join("/", "home", usernameForHomeDir)

	basePath, err = workspace.ExpandPath(u.Path(), func() (*user.User, error) {
		return &user.User{Username: username, HomeDir: homedir}, nil
	}, func() (string, error) {
		// return host's base path, but this should never happen
		return "/", nil
	})
	if err != nil {
		return
	}

	hostPort = u.Host()
	if hostPort == "" {
		err = errors.New("ssh scheme with empty host is invalid")
		return
	}

	return
}

func (s *scheme) connectScheme(uri workspace.URI, closeHook func(error)) (workspace.Scheme, error) {
	sshPath := s.basePath
	if sshPath == "" {
		sshPath = "."
	}

	remote, err := s.remoteFn(s.cfg, uri)
	if err != nil {
		return nil, fmt.Errorf("could not initialize remote: %w", err)
	}

	ses, err := remote.NewSession()
	if err != nil {
		return nil, err
	}

	pid, err := ses.Command(fmt.Sprintf("six -x %s", sshPath))
	if err != nil {
		return nil, fmt.Errorf("could not create command: %s", err)
	}

	stdout, stderr, stdin, err := startProc(ses, pid)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.Dial("", grpc.WithInsecure(),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return newStdConn(stdout, stdin, func() {
				closeHook(errors.New("ssh connection closed unexpectedly"))
			}), nil
		}))
	if err != nil {
		return nil, err
	}

	go func() {
		err := ses.Wait(pid)
		defer remote.Close()
		defer ses.Close()
		if err != nil {
			stderrStr, rerr := ioutil.ReadAll(stderr)
			if rerr != nil {
				err = fmt.Errorf("could not read error from stderr but there was"+
					"an error executing remote six server over SSH: %s", err)
			} else {
				err = fmt.Errorf("error executing remote six server over SSH: %s: %s", err, stderrStr)
			}
		}
		closeHook(err)
	}()

	return workspacepb.NewScheme(conn), nil
}

func (s *scheme) init(cc sshConfig, uri workspace.URI) (err error) {
	if uri.Scheme() != Scheme {
		return errors.New("invalid non-ssh scheme")
	}

	s.cfg = cc
	s.user, s.homedir, s.hostPort, s.basePath, err = parseWorkspaceURI(uri, s.getUser)
	if err != nil {
		return fmt.Errorf("could not parse ssh workspace URI: %s", err)
	}

	s.Scheme = newRemoteScheme(s.connectSchemeFn, uri)

	cwdpath, err := s.expandPath(uri.Path())
	if err != nil {
		return err
	}
	fi, err := s.Scheme.Stat(cwdpath)
	if err != nil {
		return err
	}

	if !fi.IsDir() {
		uri = workspace.Dir(uri)
		_ = s.Scheme.Close()
		// re-initialize with dir uri
		return s.init(cc, uri)
	}

	return nil
}

func (s *scheme) Open(path string, flag int, perm os.FileMode) (workspace.File, *workspace.Error) {
	return s.Scheme.Open(path, flag, perm)
}

func (s *scheme) Remove(path string) error {
	return s.Scheme.Remove(path)
}

func (s *scheme) Rename(oldpath, newpath string) error {
	return s.Scheme.Rename(oldpath, newpath)
}

func (s *scheme) Stat(path string) (os.FileInfo, error) {
	return s.Scheme.Stat(path)
}

func (s *scheme) Lstat(path string) (os.FileInfo, error) {
	return s.Scheme.Lstat(path)
}

func (s *scheme) ReadLink(path string) (string, error) {
	return s.Scheme.ReadLink(path)
}

func (s *scheme) expandPath(path string) (string, error) {
	return workspace.ExpandPath(path, func() (*user.User, error) {
		return &user.User{Username: s.user, HomeDir: s.homedir}, nil
	}, func() (string, error) {
		return s.basePath, nil
	})
}

func (s *scheme) URI(path string) (workspace.URI, error) {
	absPath, err := s.expandPath(path)
	if err != nil {
		return workspace.URI{}, err
	}

	var uriStr string
	if s.user != "" {
		uriStr = fmt.Sprintf("ssh://%s@%s%s", s.user, s.hostPort, absPath)
	} else {
		uriStr = fmt.Sprintf("ssh://%s%s", s.hostPort, absPath)
	}

	return workspace.ParseURI(uriStr)
}

func (s *scheme) Close() (ret error) {
	if err := s.Scheme.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	return ret
}

func startProc(exec workspace.Executor, pid workspace.Pid) (
	stdout io.ReadCloser, stderr io.ReadCloser, stdin io.WriteCloser, err error,
) {
	stdout, err = exec.StdoutPipe(pid)
	if err != nil {
		err = fmt.Errorf("could not get stdout pipe: %s", err)
		return
	}

	stdin, err = exec.StdinPipe(pid)
	if err != nil {
		err = fmt.Errorf("could not get stdin pipe: %s", err)
		return
	}

	stderr, err = exec.StderrPipe(pid)
	if err != nil {
		err = fmt.Errorf("could not get stderr pipe: %s", err)
		return
	}

	err = exec.Start(pid)
	if err != nil {
		err = fmt.Errorf("could not start remote command: %s", err)
		return
	}
	return
}
