package document

import (
	"context"
	"syscall"

	workspaceapi "unstable.build/go-tui/api/workspace"
)

type unimplementedExecutor struct {
}

func (s unimplementedExecutor) StartCommand(ctx context.Context, cmd workspaceapi.Cmd) (
	workspaceapi.Pid, error,
) {
	return 0, errUnimplemented
}

func (s unimplementedExecutor) Signal(workspaceapi.Pid, syscall.Signal) error {
	return errUnimplemented
}

type unimplementedTerminal struct {
}

func (s unimplementedTerminal) NewPty(context.Context) (workspaceapi.Pty, error) {
	return workspaceapi.Pty{}, errUnimplemented
}

func (s unimplementedTerminal) SetPtySize(p workspaceapi.Pty, width, height int) error {
	return errUnimplemented
}
