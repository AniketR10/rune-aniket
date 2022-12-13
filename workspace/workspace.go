package workspace

import (
	"io"
	os "os"
	"syscall"

	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/config"
)

// Workspace binds a Loader and a Scheme together for use in internal
// packages that both need to share an API with external resources
// and use a Loader to load resources into buffers.
type Workspace interface {
	Loader
	Scheme
}

// Loader abstracts the ability to load resource data into a working buffer
// and provide a FlusherCloser to manage flushing data to storage.
type Loader interface {
	Load(file workspaceapi.URI, buf *cell.Buffer, swapDir workspaceapi.URI, readOnly bool) (FlusherCloser, error)
	Recover(file, swapFilePath workspaceapi.URI, buf *cell.Buffer, force bool) (FlusherCloser, error)
}

// Scheme abstracts internal workspace scheme-based gouroutine-safe implementations.
type Scheme interface {
	Executor
	Terminal

	URI(path string) (workspaceapi.URI, error)

	Open(path string, flag int, perm os.FileMode) (workspaceapi.File, *workspaceapi.Error)
	Remove(path string) error
	Rename(old, new string) error
	Stat(path string) (os.FileInfo, error)
	Lstat(path string) (os.FileInfo, error)
	ReadLink(path string) (string, error)
	ReadDir(string) ([]os.DirEntry, error)
}

// SchemeFunc represents a Scheme constructor.
type SchemeFunc func(config.Config, workspaceapi.URI) (Scheme, error)

// SchemeManager abstracts the ability to register new URI schemes.
type SchemeManager interface {
	RegisterScheme(string, SchemeFunc) error
}

// WorkspaceManager abstracts the ability to register schemes and workspaces.
type WorkspaceManager interface {
	SchemeManager
	AddWorkspace(workspaceapi.URI) (Workspace, error)
}

// Terminal abstracts the ability to manage pseudoterminals.
type Terminal interface {
	// NewPty creates a new pseudoterminal.
	NewPty() (workspaceapi.Pty, error)

	// SetPtySize sets the width and height in columns and rows of
	// a pseudoterminal.
	SetPtySize(p workspaceapi.Pty, width, height int) error
}

// Executor is the public facing API of a workspace's command execution.
type Executor interface {
	// Command returns the Pid to execute the named program with the given
	// arguments. For more details see exec.Command.
	Command(name string, arg ...string) (workspaceapi.Pid, error)
	// Start starts the specified command but does not wait for it to complete.
	// The Wait method will return an error if there's any while running command
	// and release associated resources.
	Start(workspaceapi.Pid) error
	// Signal sends a signal to the running process.
	Signal(workspaceapi.Pid, syscall.Signal) error
	// StderrPipe returns a pipe that will be connected to the command's standard
	// error when the command starts. See exec.Cmd.StderrPipe for more details.
	StderrPipe(workspaceapi.Pid) (io.ReadCloser, error)
	// StdinPipe returns a pipe that will be connected to the command's standard
	// input when the command starts. See exec.Cmd.StdinPipe for more details.
	StdinPipe(workspaceapi.Pid) (io.WriteCloser, error)
	// StdoutPipe returns a pipe that will be connected to the command's standard
	// output when the command starts. See exec.Cmd.StdoutPipe for more details.
	StdoutPipe(workspaceapi.Pid) (io.ReadCloser, error)
	// Wait waits for the command to exit and waits for any copying to stdin or
	// opying from stdout or stderr to complete.
	// The command must have been started by Start.
	// The returned error is nil if the command runs, has no problems copying
	// stdin, stdout, and stderr, and exits with a zero exit status.
	Wait(workspaceapi.Pid) error

	io.Closer
}

// FlusherCloser wraps Flush and Close methods to be used
// in conjunction with a cell.Buffer as file buffer abstractions.
type FlusherCloser interface {
	Flush() error
	io.Closer
}
