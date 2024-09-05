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

//revive:disable:exported
package scheme

import (
	"context"
	"errors"
	"io"
	"os"
	"syscall"

	"unstable.build/go-tui/api/config"
	workspaceapi "unstable.build/go-tui/api/workspace"
)

var (
	// ErrSchemeAlreadyRegistered is retured by RegisterScheme when a scheme for the given
	// uri has already been registered.
	ErrSchemeAlreadyRegistered = errors.New("scheme already registered")
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
	MkdirAll(string, os.FileMode) error

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
