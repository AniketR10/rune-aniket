package api

import (
	"io"
	"os"
	"syscall"
)

// Pid is an Executor's command identifier. It doesn't necessarily translate
// to an os.Process.Pid.
type Pid int

// FileSystem abstracts the public facing API of a workspace file system.
type FileSystem interface {
	URI(path string) (URI, error)

	// Open opens a file at path with the given flag and mode.
	Open(path string, flag int, mode os.FileMode) (File, *Error)

	// Remove removes the file at path.
	Remove(path string) error

	// Stat returns a FileInfo describing the named file.
	Stat(path string) (os.FileInfo, error)

	// ReadDir reads the named directory, returning all its directory entries.
	ReadDir(name string) ([]os.DirEntry, error)
}

// Cmd represents an external command being prepared to run. See exec.Cmd for
// more details. Stdin, Stderr and Stdout, if set, will have their corresponding
type Cmd struct {
	Path    string
	Args    []string
	Env     []string
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	Watcher Watcher

	// SysProcAttr is ignored if passed from a extension.
	SysProcAttr *syscall.SysProcAttr
}

// Watcher adds the ability for wait a processes started by Cmd
// and collect any I/O errors or non-zero exit code.
type Watcher interface {
	// Watch returns a channel that can be used to
	// wait for the command to exit and waits for any copying to stdin or
	// copying from stdout or stderr to complete.

	// The returned error is nil if the command runs, has no problems copying
	// stdin, stdout, and stderr, and exits with a zero exit status.
	Watch() chan error
}

// Executor is the public facing API of a workspace's command execution.
type Executor interface {
	// Start starts the given cmd and returns the Pid of the underlying
	// process.
	Start(Cmd) (Pid, error)

	// Signal sends a signal to the running process.
	Signal(Pid, syscall.Signal) error

	io.Closer
}

// Terminal abstracts the ability to manage pseudoterminals.
type Terminal interface {
	// StartPty creates a new pseudoterminal.
	StartPty() (Pty, error)

	// SetPtySize sets the width and height in columns and rows of
	// a pseudoterminal.
	SetPtySize(p Pty, width, height int) error
}

// Pty is a pseudoterminal on a Workspace.
type Pty struct {
	// Master is the pty master device file.
	Master File
	// Slave is the pseudoterminal slave device path.
	Slave File
}

// File abstracts a subset of os.File
type File interface {
	Name() string
	Stat() (os.FileInfo, error)
	Sync() error
	Truncate(size int64) error
	Fd() uintptr

	io.Seeker
	io.Reader
	io.Closer
	io.Writer
}
