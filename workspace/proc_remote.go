package workspace

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"syscall"

	"github.com/ernestrc/go-multierror"
)

var (
	errInvalidPID = errors.New("invalid PID")
)

type managerClient struct {
	mu       sync.Mutex
	cmd      string
	args     []string
	m        *Manager
	sessions []*procSession
}

type procSession struct {
	sshCmd   string
	sshArgs  []string
	m        *Manager
	waitHook func()

	pid Pid
}

func (m *managerClient) NewSession() (Executor, error) {
	ses := &procSession{
		sshCmd:  m.cmd,
		sshArgs: m.args,
		m:       m.m,
	}
	ses.waitHook = func() {
		idx := -1
		for i, _ses := range m.sessions {
			if _ses == ses {
				idx = i
				break
			}
		}
		if idx == -1 {
			panic("could not find proc session in managerClient")
		}
		m.sessions = append(m.sessions[:idx], m.sessions[idx+1:]...)
	}
	m.sessions = append(m.sessions, ses)
	return ses, nil
}

func (m *managerClient) Close() (ret error) {
	for _, s := range m.sessions {
		err := m.m.Signal(s.pid, syscall.SIGTERM)
		if err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	return
}

func (s *procSession) Command(name string, arg ...string) (Pid, error) {
	args := fmt.Sprintf("%s %s %s %s",
		s.sshCmd, strings.Join(s.sshArgs, " "),
		name, strings.Join(arg, " "))
	argv := strings.Split(args, " ")
	name = argv[0]
	arg = argv[1:]
	pid, err := s.m.Command(name, arg...)
	if err != nil {
		return Pid(0), fmt.Errorf("Failed to create ssh command: %s", err)
	}
	s.pid = pid
	return pid, nil
}

func (s *procSession) Start(pid Pid) error {
	if pid != s.pid {
		return errInvalidPID
	}
	return s.m.Start(s.pid)
}

func (s *procSession) Signal(pid Pid, sig syscall.Signal) error {
	if pid != s.pid {
		return errInvalidPID
	}
	return s.m.Signal(pid, sig)
}

func (s *procSession) StderrPipe(pid Pid) (io.ReadCloser, error) {
	if pid != s.pid {
		return nil, errInvalidPID
	}
	return s.m.StderrPipe(pid)
}

func (s *procSession) StdinPipe(pid Pid) (io.WriteCloser, error) {
	if pid != s.pid {
		return nil, errInvalidPID
	}
	return s.m.StdinPipe(pid)
}

func (s *procSession) StdoutPipe(pid Pid) (io.ReadCloser, error) {
	if pid != s.pid {
		return nil, errInvalidPID
	}
	return s.m.StdoutPipe(pid)
}

func (s *procSession) Wait(pid Pid) error {
	if pid != s.pid {
		return errInvalidPID
	}
	if s.waitHook == nil {
		return nil
	}
	err := s.m.Wait(pid)
	s.waitHook()
	s.waitHook = nil
	return err
}

func (m *Manager) connectOverProcSSH() (
	sshClient, error,
) {
	port := m.workspace.parsed.Port()
	if port == "" {
		port = "22"
	}
	cmd := strings.ReplaceAll(m.managerCfg.sshCommand, "%p", port)

	userHost := m.workspace.parsed.Host
	if m.workspace.parsed.User != nil {
		userHost = fmt.Sprintf("%s@%s",
			m.workspace.parsed.User.Username(), m.workspace.parsed.Host)
	}
	cmd = strings.ReplaceAll(cmd, "%h", userHost)

	args := strings.Split(cmd, " ")
	return &managerClient{m: m, cmd: args[0], args: args[1:]}, nil
}
