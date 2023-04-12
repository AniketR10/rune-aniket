package scheme

import (
	"context"
	"io"
	"os"
	"syscall"

	"unstable.build/go-tui/api/config"
	workspaceapi "unstable.build/go-tui/api/workspace"
)

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

// SchemeFunc represents a Scheme constructor.
type SchemeFunc func(context.Context, config.Config, workspaceapi.URI) (Scheme, error)

// SchemeManager abstracts the ability to register new URI schemes.
type SchemeManager interface {
	RegisterScheme(string, SchemeFunc) error
}
