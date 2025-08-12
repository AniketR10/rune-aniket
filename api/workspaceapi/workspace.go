// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package workspaceapi

import (
	"context"
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

	// MkdirAll creates a directory named path, along with any necessary parents,
	// and returns nil, or else returns an error. The permission bits perm (before
	// umask) are used for all directories that MkdirAll creates. If path is
	// already a directory, MkdirAll does nothing and returns nil.
	MkdirAll(path string, perm os.FileMode) error
}

// Cmd represents an external command being prepared to run. See exec.Cmd for
// more details. Stdin, Stderr and Stdout, if set, will have their corresponding
type Cmd struct {
	Path    string
	Dir     string
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
	Start(context.Context, Cmd) (Pid, error)

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
