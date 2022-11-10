package document

import (
	"io"
	"syscall"

	"unstable.build/go-tui/workspace"
)

type unimplementedExecutor struct {
}

func (s unimplementedExecutor) Command(name string, arg ...string) (workspace.Pid, error) {
	return 0, errUnimplemented
}

func (s unimplementedExecutor) Start(workspace.Pid) error {
	return errUnimplemented
}

func (s unimplementedExecutor) Signal(workspace.Pid, syscall.Signal) error {
	return errUnimplemented
}

func (s unimplementedExecutor) StderrPipe(workspace.Pid) (io.ReadCloser, error) {
	return nil, errUnimplemented
}

func (s unimplementedExecutor) StdinPipe(workspace.Pid) (io.WriteCloser, error) {
	return nil, errUnimplemented
}

func (s unimplementedExecutor) StdoutPipe(workspace.Pid) (io.ReadCloser, error) {
	return nil, errUnimplemented
}

func (s unimplementedExecutor) Wait(workspace.Pid) error {
	return errUnimplemented
}

type unimplementedTerminal struct {
}

func (s unimplementedTerminal) NewPty() (workspace.Pty, error) {
	return workspace.Pty{}, errUnimplemented
}

func (s unimplementedTerminal) SetPtySize(p workspace.Pty, width, height int) error {
	return errUnimplemented
}
