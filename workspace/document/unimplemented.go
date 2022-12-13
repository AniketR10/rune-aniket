package document

import (
	"io"
	"syscall"
	workspaceapi "unstable.build/go-tui/api/workspace"
)

type unimplementedExecutor struct {
}

func (s unimplementedExecutor) Command(name string, arg ...string) (workspaceapi.Pid, error) {
	return 0, errUnimplemented
}

func (s unimplementedExecutor) Start(workspaceapi.Pid) error {
	return errUnimplemented
}

func (s unimplementedExecutor) Signal(workspaceapi.Pid, syscall.Signal) error {
	return errUnimplemented
}

func (s unimplementedExecutor) StderrPipe(workspaceapi.Pid) (io.ReadCloser, error) {
	return nil, errUnimplemented
}

func (s unimplementedExecutor) StdinPipe(workspaceapi.Pid) (io.WriteCloser, error) {
	return nil, errUnimplemented
}

func (s unimplementedExecutor) StdoutPipe(workspaceapi.Pid) (io.ReadCloser, error) {
	return nil, errUnimplemented
}

func (s unimplementedExecutor) Wait(workspaceapi.Pid) error {
	return errUnimplemented
}

type unimplementedTerminal struct {
}

func (s unimplementedTerminal) NewPty() (workspaceapi.Pty, error) {
	return workspaceapi.Pty{}, errUnimplemented
}

func (s unimplementedTerminal) SetPtySize(p workspaceapi.Pty, width, height int) error {
	return errUnimplemented
}
