package workspace

import (
	"context"
	"io"
	os "os"
	"syscall"

	"github.com/ernestrc/blue/iterator"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/config"
)

// API abstract the public-facing API of a workspace.
type API interface {
	// Getwd gets the current workspace URI.
	Getwd() (URI, error)

	// URI builds a URI from a path in the current workspace.
	URI(path string) (URI, error)

	// Open opens a file at path with the given flag and mode.
	Open(path string, flag int, mode os.FileMode) (File, *Error)

	// Remove removes the file at path.
	Remove(path string) error

	// ListFiles traverses the workspace directory and returns
	// an iterator that returns all file paths. If errors
	// are encountered while reading the contents of directories
	// those errors will be aggregated and reported by the iterator's
	// Err method.
	ListFiles(ctx context.Context) (iterator.Iterator[string], error)

	Terminal

	Executor
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

// SchemeFunc represents a Scheme constructor.
type SchemeFunc func(config.Config, URI) (Scheme, error)

// SchemeManager abstracts the ability to register new URI schemes.
type SchemeManager interface {
	RegisterScheme(string, SchemeFunc) error
}

// WorkspaceManager abstracts the ability to register schemes and workspaces.
type WorkspaceManager interface {
	SchemeManager
	AddWorkspace(URI) (Workspace, error)
}

// Pid is an Executor's command identifier. It doesn't necessarily translate
// to an os.Process.Pid.
type Pid int32

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

/* internal API */

// Workspace binds a Loader and an API together for use in internal
// packages that both need to share API with external resources
// and use a Loader to load resources into buffers.
type Workspace interface {
	API
	Loader
}

// FlusherCloser wraps Flush and Close methods to be used
// in conjunction with a cell.Buffer as file buffer abstractions.
type FlusherCloser interface {
	Flush() error
	io.Closer
}

// Loader abstracts the ability to load resource data into a working buffer
// and provide a FlusherCloser to manage flushing data to storage.
type Loader interface {
	URI(string) (URI, error)

	Load(file URI, buf *cell.Buffer, swapDir URI, readOnly bool) (FlusherCloser, error)
	Recover(file, swapFilePath URI, buf *cell.Buffer, force bool) (FlusherCloser, error)
}

// Scheme abstracts internal workspace scheme-based gouroutine-safe implementations.
type Scheme interface {
	Executor
	Terminal

	URI(path string) (URI, error)

	Open(path string, flag int, perm os.FileMode) (File, *Error)
	Remove(path string) error
	Rename(old, new string) error
	Stat(path string) (os.FileInfo, error)
	Lstat(path string) (os.FileInfo, error)
	ReadLink(path string) (string, error)
	ListFiles(context.Context) (iterator.Iterator[string], error)
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

// Error is used to abstract os.Is(.*) functions
type Error struct {
	Err          error
	IsPermission bool
	IsExist      bool
	IsNotExist   bool
}

func (e Error) ToError() error {
	if e.IsPermission {
		return os.ErrPermission
	}
	if e.IsNotExist {
		return os.ErrNotExist
	}
	if e.IsExist {
		return os.ErrExist
	}
	if e.Err != nil {
		return e.Err
	}
	panic("workspace.Error with nil Error")
}

// NopError returns an Error that simply wraps err.
func NopError(err error) *Error {
	return &Error{Err: err}
}
