package ssh

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"syscall"

	bluectx "github.com/ernestrc/blue/context"
	"github.com/ernestrc/go-multierror"
	"unstable.build/go-tui/api/config"
	schemeapi "unstable.build/go-tui/api/scheme"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/workspace"
)

var (
	errInvalidPID = errors.New("invalid PID")
)

type procRemote struct {
	cmd  string
	args []string

	executor  schemeapi.Executor
	sessions  []*procSession
	ctx       context.Context
	cancelCtx func()
}

type procSession struct {
	sshCmd   string
	sshArgs  []string
	executor schemeapi.Executor
	ctx      context.Context
}

func newProcRemote(ctx context.Context, cfg sshConfig, uri workspaceapi.URI) (
	remote, error,
) {
	port := uri.Port()
	cmd := cfg.command
	if port == "" {
		port = "22"
	} else {
		if !strings.Contains(cfg.command, "%p") {
			return nil, fmt.Errorf("unable to set custom port: 'command' value is missing %%p")
		}
		cmd = strings.ReplaceAll(cfg.command, "%p", port)
	}

	host := uri.Hostname()
	if uri.User() != "" {
		host = fmt.Sprintf("%s@%s", uri.User(), host)
	}
	cmd = strings.ReplaceAll(cmd, "%h", host)

	args := strings.Split(cmd, " ")

	// make sure NewFileScheme will not return an error
	uri, err := workspaceapi.CurrentUserHostURI(".")
	if err != nil {
		return nil, fmt.Errorf("could not get current user host URI: %s", err)
	}

	// local because we are going to execute commands locally with command ssh sessions
	localExecutor, err := workspace.NewFileScheme(ctx, config.NopConfig(), uri)
	if err != nil {
		return nil, fmt.Errorf("could initialize local executor on URI %q: %s", uri.String(), err)
	}

	ret := &procRemote{executor: localExecutor, cmd: args[0], args: args[1:]}
	ret.ctx, ret.cancelCtx = context.WithCancel(context.Background())
	return ret, nil
}

func (m *procRemote) NewSession() (schemeapi.Executor, error) {
	ses := &procSession{
		sshCmd:   m.cmd,
		sshArgs:  m.args,
		executor: m.executor,
		ctx:      m.ctx,
	}

	m.sessions = append(m.sessions, ses)
	return ses, nil
}

func (m *procRemote) Close() (ret error) {
	m.cancelCtx()
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

func (s *procSession) StartCommand(ctx context.Context, cmd workspaceapi.Cmd) (
	workspaceapi.Pid, error,
) {
	if cmd.Path == "" {
		return 0, errors.New("no command")
	}

	cmd.Path, cmd.Args = s.CommandString(cmd.Path, cmd.Args...)
	var cancelFn func()
	ctx, cancelFn = context.WithCancel(ctx)
	cmd.Watcher = newWrapWatcher(cmd.Watcher, cancelFn)
	pid, err := s.executor.StartCommand(bluectx.First(s.ctx, ctx), cmd)
	if err != nil {
		return workspaceapi.Pid(0), fmt.Errorf("Failed to create ssh command: %s", err)
	}
	return pid, nil
}

func (s *procSession) Signal(pid workspaceapi.Pid, sig syscall.Signal) error {
	return s.executor.Signal(pid, sig)
}

func (s *procSession) Close() (ret error) {
	return ret
}
