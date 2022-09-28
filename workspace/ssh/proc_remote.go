package ssh

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"syscall"

	"github.com/ernestrc/go-multierror"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/workspace"
)

var (
	errInvalidPID = errors.New("invalid PID")
)

type procRemote struct {
	pid      int
	mu       sync.Mutex
	cmd      string
	args     []string
	executor workspace.Executor
	session  *procSession
}

type procSession struct {
	sshCmd   string
	sshArgs  []string
	executor workspace.Executor

	pid workspace.Pid
}

func validateProcRemote(c sshConfig, uri workspace.URI) error {
	if c.command == "" {
		return errors.New("empty command")
	}
	_, err := workspace.CurrentUserHostURI(".")
	return err
}

func newProcRemote(cfg sshConfig, uri workspace.URI) (
	remote, error,
) {
	port := uri.Port()
	if port == "" {
		port = "22"
	}
	cmd := strings.ReplaceAll(cfg.command, "%p", port)

	userHost := uri.Host()
	if uri.User() != "" {
		userHost = fmt.Sprintf("%s@%s", uri.User(), userHost)
	}
	cmd = strings.ReplaceAll(cmd, "%h", userHost)

	args := strings.Split(cmd, " ")

	// make sure NewFileScheme will not return an error
	uri, err := workspace.CurrentUserHostURI(".")
	if err != nil {
		return nil, fmt.Errorf("could not get current user host URI: %s", err)
	}

	// local because we are going to execute commands locally with command ssh sessions
	localExecutor, err := workspace.NewFileScheme(config.NopConfig(), uri)
	if err != nil {
		return nil, fmt.Errorf("could initialize local executor on URI %q: %s", uri.String(), err)
	}

	return &procRemote{pid: -1, executor: localExecutor, cmd: args[0], args: args[1:]}, nil
}

func (m *procRemote) NewSession() (workspace.Executor, error) {
	if m.session != nil {
		return nil, errors.New("session already started and can create only one session")
	}
	ses := &procSession{
		sshCmd:   m.cmd,
		sshArgs:  m.args,
		executor: m.executor,
	}
	m.session = ses
	return ses, nil
}

func (m *procRemote) Close() (ret error) {
	if err := m.executor.Signal(m.session.pid, syscall.SIGTERM); err != nil {
		ret = multierror.Append(ret, err)
	}
	if err := m.executor.Close(); err != nil {
		ret = multierror.Append(ret, err)
	}
	return ret
}

func (s *procSession) Command(name string, arg ...string) (workspace.Pid, error) {
	if name == "" {
		return 0, errors.New("invalid empty command")
	}
	if s.pid != -1 {
		panic("Command called more than once on an ssh session")

	}

	args := fmt.Sprintf("%s %s %s %s",
		s.sshCmd, strings.Join(s.sshArgs, " "),
		name, strings.Join(arg, " "))
	argv := strings.Split(args, " ")
	name = argv[0]
	arg = argv[1:]
	pid, err := s.executor.Command(name, arg...)
	if err != nil {
		return workspace.Pid(0), fmt.Errorf("Failed to create ssh command: %s", err)
	}
	s.pid = pid
	return pid, nil
}

func (s *procSession) Start(pid workspace.Pid) error {
	if pid != s.pid {
		return errInvalidPID
	}
	return s.executor.Start(s.pid)
}

func (s *procSession) Signal(pid workspace.Pid, sig syscall.Signal) error {
	if pid != s.pid {
		return errInvalidPID
	}
	return s.executor.Signal(pid, sig)
}

func (s *procSession) StderrPipe(pid workspace.Pid) (io.ReadCloser, error) {
	if pid != s.pid {
		return nil, errInvalidPID
	}
	return s.executor.StderrPipe(pid)
}

func (s *procSession) StdinPipe(pid workspace.Pid) (io.WriteCloser, error) {
	if pid != s.pid {
		return nil, errInvalidPID
	}
	return s.executor.StdinPipe(pid)
}

func (s *procSession) StdoutPipe(pid workspace.Pid) (io.ReadCloser, error) {
	if pid != s.pid {
		return nil, errInvalidPID
	}
	return s.executor.StdoutPipe(pid)
}

func (s *procSession) Wait(pid workspace.Pid) error {
	if pid != s.pid {
		return errInvalidPID
	}
	return s.executor.Wait(pid)
}

func (s *procSession) Close() error {
	return nil
}
