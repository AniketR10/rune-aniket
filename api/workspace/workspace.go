package api

import (
	"io"
	"os"
	"syscall"
)

// Pid is an Executor's command identifier. It doesn't necessarily translate
// to an os.Process.Pid.
type Pid int32

// Workspace abstracts the public-facing API of a workspace.
type Workspace interface {
	URI(path string) (URI, error)

	// Open opens a file at path with the given flag and mode.
	Open(path string, flag int, mode os.FileMode) (File, *Error)

	// Remove removes the file at path.
	Remove(path string) error

	// Stat returns a FileInfo describing the named file.
	Stat(path string) (os.FileInfo, error)

	// ReadDir reads the named directory, returning all its directory entries.
	ReadDir(name string) ([]os.DirEntry, error)

	Terminal

	Executor
}

// Executor is the public facing API of a workspace's command execution.
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

	io.Closer
}

// Terminal abstracts the ability to manage pseudoterminals.
type Terminal interface {
	// NewPty creates a new pseudoterminal.
	NewPty() (Pty, error)

	// SetPtySize sets the width and height in columns and rows of
	// a pseudoterminal.
	SetPtySize(p Pty, width, height int) error
}

// Pty is a pseudoterminal on a Workspace.
type Pty struct {
	// Pid of the underlying command.
	Pid
	// Master is the pty master device file.
	Master File
	// Slave is the pseudoterminal slave device path.
	Slave string
}

// File abstracts a subset of os.File
type File interface {
	Name() string
	Stat() (os.FileInfo, error)
	Sync() error
	Truncate(size int64) error

	io.Seeker
	io.Reader
	io.Closer
	io.Writer
}
