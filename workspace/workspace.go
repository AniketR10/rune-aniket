package workspace

import (
	"context"
	"io"
	"os"
	"syscall"

	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/api/config"
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

// Scheme abstracts internal workspace scheme implementations.
type Scheme interface {
	URI(path string) (workspaceapi.URI, error)

	NewFile(fd uintptr, name string) workspaceapi.File
	Open(path string, flag int, perm os.FileMode) (workspaceapi.File, *workspaceapi.Error)
	Remove(path string) error
	Rename(old, new string) error
	Stat(path string) (os.FileInfo, error)
	Lstat(path string) (os.FileInfo, error)
	ReadLink(path string) (string, error)
	ReadDir(string) ([]os.DirEntry, error)

	Executor

	Terminal
}

// SchemeFunc represents a Scheme constructor.
type SchemeFunc func(context.Context, config.Config, workspaceapi.URI) (Scheme, error)

// SchemeManager abstracts the ability to register new URI schemes.
type SchemeManager interface {
	RegisterScheme(string, SchemeFunc) error
	// TODO once scheme plugin is moved to api
	// UnregisterScheme(string) error
}

// WorkspaceManager abstracts the ability to register schemes and workspaces.
type WorkspaceManager interface {
	SchemeManager
	AddWorkspace(context.Context, workspaceapi.URI) (Workspace, error)
}

// Terminal abstracts the ability to manage pseudoterminals.
type Terminal interface {
	// NewPty creates a new pseudoterminal.
	// The provided context is used to kill the process (by calling
	// os.Process.Kill) if the context becomes done before the command completes on
	// its own.
	NewPty(context.Context) (workspaceapi.Pty, error)

	// SetPtySize sets the width and height in columns and rows of
	// a pseudoterminal.
	SetPtySize(p workspaceapi.Pty, width, height int) error
}

// Executor is the public facing API of a workspace's command execution.
type Executor interface {
	// StartCommand starts the given cmd and returns the Pid of the underlying
	// process. The provided context is used to kill the process (by calling
	// os.Process.Kill) if the context.Done channel is closed  before the command
	// completes on its own.
	// Implementations must clean all resources associated with a command
	// once the process exits.
	StartCommand(ctx context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error)

	// Signal sends a signal to the running process.
	Signal(workspaceapi.Pid, syscall.Signal) error

	io.Closer
}

// FlusherCloser wraps Flush and Close methods to be used
// in conjunction with a cell.Buffer as file buffer abstractions.
type FlusherCloser interface {
	Flush() error
	io.Closer
}
