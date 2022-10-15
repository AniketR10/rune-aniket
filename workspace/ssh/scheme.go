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
	"strings"
	"time"

	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/workspace"
	workspacepb "unstable.build/go-tui/workspace/rpc"
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

func (s *scheme) runAndWait(remote remote, cmd string) (bool, error) {
	ses, err := remote.NewSession()
	if err != nil {
		return false, err
	}

	if s.cfg.shell != "" {
		cmd = fmt.Sprintf("%s -c '%s'", s.cfg.shell, cmd)
	}
	pid, err := ses.Command(cmd)
	if err != nil {
		return false, err
	}

	err = ses.Start(pid)
	if err != nil {
		return false, err
	}

	err = ses.Wait(pid)
	if err != nil {
		return false, nil
	}

	return true, nil
}

func (s *scheme) whichCommand(remote remote, cmd string) error {
	avail, err := s.runAndWait(remote, fmt.Sprintf("which %s", cmd))
	if err != nil {
		return fmt.Errorf("could not check if %s executable is in PATH: %w", cmd, err)
	}
	if !avail {
		return fmt.Errorf("%s executable was not found on remote. "+
			"Make sure it's installed and available via $PATH to a non-interactive shell", cmd)
	}
	return nil
}

func (s *scheme) workspaceExists(remote remote, uri workspace.URI) error {
	ok, err := s.runAndWait(remote, fmt.Sprintf("ls %s", uri.Path()))
	if err != nil {
		return fmt.Errorf("could not check if workspace path %q exists: %w", uri.Path(), err)
	}
	if !ok {
		return fmt.Errorf("path %q does not exists", uri.Path())
	}
	return nil
}

func (s *scheme) connectScheme(uri workspace.URI, closeHook func(error)) (workspace.Scheme, error) {
	const six = "six"

	sshPath := s.basePath
	if sshPath == "" {
		sshPath = "."
	}

	remote, err := s.remoteFn(s.cfg, uri)
	if err != nil {
		return nil, fmt.Errorf("could not initialize remote: %w", err)
	}

	// NOTE: the next checks are to avoid error messages getting lost when
	// trying to connect so we can provide a better error messages

	err = s.whichCommand(remote, six)
	if err != nil {
		return nil, err
	}

	err = s.workspaceExists(remote, uri)
	if err != nil {
		return nil, err
	}

	ses, err := remote.NewSession()
	if err != nil {
		return nil, err
	}

	var extraArgs string
	if debug.StandardLogger().IsLevelEnabled(log.TraceLevel) {
		extraArgs = "-o six-workspace-server.log"
	}

	cmd := fmt.Sprintf("%s -x %s %s", six, sshPath, extraArgs)
	if s.cfg.shell != "" {
		cmd = fmt.Sprintf("%s -c '%s'", s.cfg.shell, cmd)
	}

	pid, err := ses.Command(cmd)
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

// override to provide user with an error message that guides to a solution
func (s *scheme) Start(pid workspace.Pid) error {
	err := s.Scheme.Start(pid)
	if err != nil && strings.Contains(err.Error(), "executable file not found in $PATH") {
		return fmt.Errorf("%w. Make sure that $PATH is configured "+
			"even for non-interactive shells (i.e. .bashrc, .profile, etc.)", err)
	}
	return err
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

func (s *scheme) NewPty() (workspace.Pty, error) {
	return s.Scheme.NewPty()
}

func (s *scheme) SetPtySize(pty workspace.Pty, width, height int) error {
	return s.Scheme.SetPtySize(pty, width, height)
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

func (s *scheme) expandPath(path string) (string, error) {
	return workspace.ExpandPath(path, func() (*user.User, error) {
		return &user.User{Username: s.user, HomeDir: s.homedir}, nil
	}, func() (string, error) {
		return s.basePath, nil
	})
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
