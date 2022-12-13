package ssh

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"syscall"

	"github.com/ernestrc/go-multierror"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/workspace"
)

var (
	errInvalidPID = errors.New("invalid PID")
)

type procRemote struct {
	cmd  string
	args []string

	executor workspace.Executor
	sessions []*procSession
}

type procSession struct {
	sshCmd   string
	sshArgs  []string
	executor workspace.Executor
	pids     map[workspace.Pid]struct{}
}

func newProcRemote(cfg sshConfig, uri workspaceapi.URI) (
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
	uri, err := workspaceapi.CurrentUserHostURI(".")
	if err != nil {
		return nil, fmt.Errorf("could not get current user host URI: %s", err)
	}

	// local because we are going to execute commands locally with command ssh sessions
	localExecutor, err := workspace.NewFileScheme(config.NopConfig(), uri)
	if err != nil {
		return nil, fmt.Errorf("could initialize local executor on URI %q: %s", uri.String(), err)
	}

	return &procRemote{executor: localExecutor, cmd: args[0], args: args[1:]}, nil
}

func (m *procRemote) NewSession() (workspace.Executor, error) {
	ses := &procSession{
		sshCmd:   m.cmd,
		sshArgs:  m.args,
		executor: m.executor,
		pids:     make(map[workspace.Pid]struct{}),
	}

	m.sessions = append(m.sessions, ses)
	return ses, nil
}

func (m *procRemote) Close() (ret error) {
	for _, ses := range m.sessions {
		if err := ses.Close(); err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	m.sessions = nil

	if err := m.executor.Close(); err != nil {
		ret = multierror.Append(ret, err)
	}

	return ret
}

func (m *procRemote) CommandString(name string, arg ...string) (string, []string) {
	ses := procSession{
		sshCmd:  m.cmd,
		sshArgs: m.args,
	}
	return ses.CommandString(name, arg...)
}

func (s *procSession) CommandString(name string, arg ...string) (string, []string) {
	arg = append(s.sshArgs, append([]string{name}, arg...)...)
	name = s.sshCmd
	return name, arg
}

func (s *procSession) Command(name string, arg ...string) (workspace.Pid, error) {
	if name == "" {
		return 0, errors.New("invalid empty command")
	}

	name, arg = s.CommandString(name, arg...)
	pid, err := s.executor.Command(name, arg...)
	if err != nil {
		return workspace.Pid(0), fmt.Errorf("Failed to create ssh command: %s", err)
	}
	s.pids[pid] = struct{}{}
	return pid, nil
}

func (s *procSession) Start(pid workspace.Pid) error {
	return s.executor.Start(pid)
}

func (s *procSession) Signal(pid workspace.Pid, sig syscall.Signal) error {
	return s.executor.Signal(pid, sig)
}

func (s *procSession) StderrPipe(pid workspace.Pid) (io.ReadCloser, error) {
	return s.executor.StderrPipe(pid)
}

func (s *procSession) StdinPipe(pid workspace.Pid) (io.WriteCloser, error) {
	return s.executor.StdinPipe(pid)
}

func (s *procSession) StdoutPipe(pid workspace.Pid) (io.ReadCloser, error) {
	return s.executor.StdoutPipe(pid)
}

func (s *procSession) Wait(pid workspace.Pid) error {
	err := s.executor.Wait(pid)
	delete(s.pids, pid)
	return err
}

func (s *procSession) Close() (ret error) {
	for pid := range s.pids {
		if err := s.executor.Signal(pid, syscall.SIGTERM); err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	s.pids = nil
	return ret
}
