package workspace

import (
	"io"
	"syscall"
)

// Pid is an Executor's command identifier. It doesn't necessarily translate
// to an os.Process.Pid.
type Pid int32

// Executor is the public facing API of a Manager's process execution capabilities.
type Executor interface {
	Command(name string, arg ...string) (Pid, error)
	Start(Pid) error
	Signal(Pid, syscall.Signal) error
	StderrPipe(Pid) (io.ReadCloser, error)
	StdinPipe(Pid) (io.WriteCloser, error)
	StdoutPipe(Pid) (io.ReadCloser, error)
	Wait(Pid) error
}
