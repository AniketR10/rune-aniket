package workspace

import (
	"io"
	"syscall"

	"github.com/ernestrc/go-tui/cell"
)

// FlusherCloser wraps Flush and Close methods to be used
// as editor file abstractions.
type FlusherCloser interface {
	Flush() error
	io.Closer
}

// ResourceOpener abstract resource openining and recovering for abstracting
// a Manager.
type ResourceOpener interface {
	Open(file URI, buf *cell.Buffer, swapDir URI, readOnly bool) (FlusherCloser, error)
	Recover(file, swapFilePath URI, buf *cell.Buffer) (FlusherCloser, error)
}

// Workspace abstract the public-facing API of a workspace.
type Workspace interface {
	Identifier
	Executor
}

// Pid is an Executor's command identifier. It doesn't necessarily translate
// to an os.Process.Pid.
type Pid int32

// Identifier abstracts the basic method URI.
type Identifier interface {
	// URI builds a URI from a path in the current workspace.
	URI(string) (URI, error)
}

// Executor is the public facing API of a Manager's process execution capabilities.
type Executor interface {
	// Command returns the Pid to execute the named program with the given
	// arguments. For more details see exec.Command.
	Command(name string, arg ...string) (Pid, error)
	// Start starts the specified command but does not wait for it to complete.
	// The Wait method will return an error if there's any while running command
	// and release associated resources.
	Start(Pid) error
	// Signal sends a signal to the running process.
	Signal(Pid, syscall.Signal) error
	// StderrPipe returns a pipe that will be connected to the command's standard
	// error when the command starts. See exec.Cmd.StderrPipe for more details.
	StderrPipe(Pid) (io.ReadCloser, error)
	// StdinPipe returns a pipe that will be connected to the command's standard
	// input when the command starts. See exec.Cmd.StdinPipe for more details.
	StdinPipe(Pid) (io.WriteCloser, error)
	// StdoutPipe returns a pipe that will be connected to the command's standard
	// output when the command starts. See exec.Cmd.StdoutPipe for more details.
	StdoutPipe(Pid) (io.ReadCloser, error)
	// Wait waits for the command to exit and waits for any copying to stdin or
	// opying from stdout or stderr to complete.
	// The command must have been started by Start.
	// The returned error is nil if the command runs, has no problems copying
	// stdin, stdout, and stderr, and exits with a zero exit status.
	Wait(Pid) error
}
