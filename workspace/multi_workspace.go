package workspace

import (
	"context"
	"fmt"
	"os"
	"syscall"

	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cell"
)

var _ Workspace = (multi)(multi{})

// Multi wraps a Workspace to provide oob Recover and Load requests to other workspaces/schemes
// whether initialized or not.
func Multi(
	ctx context.Context, m WorkspaceManager, def Workspace, uri workspaceapi.URI,
) Workspace {
	return newMulti(ctx, m, uri, def)
}

type multi struct {
	parentCtx context.Context
	defURI    workspaceapi.URI
	def       Workspace
	manager   WorkspaceManager
}

func newMulti(
	ctx context.Context, manager WorkspaceManager,
	defURI workspaceapi.URI, def Workspace,
) *multi {
	return &multi{parentCtx: ctx, def: def, defURI: defURI, manager: manager}
}

func (m multi) Load(
	file workspaceapi.URI, buf *cell.Buffer, swapDir workspaceapi.URI, readOnly bool,
) (FlusherCloser, error) {
	is, err := IsWorkspaceURI(m.def, file)
	if err != nil {
		return nil, fmt.Errorf("workspaceapi.URI: %s", err)
	}
	if !is {
		return m.loadExtraneous(file, buf, swapDir, readOnly)
	}
	return m.def.Load(file, buf, swapDir, readOnly)
}

func (m multi) Recover(
	file, swapFilePath workspaceapi.URI, buf *cell.Buffer, force bool,
) (FlusherCloser, error) {
	is, err := IsWorkspaceURI(m.def, file)
	if err != nil {
		return nil, fmt.Errorf("workspaceapi.URI: %s", err)
	}
	if !is {
		return m.recoverExtraneous(file, swapFilePath, buf, force)
	}
	return m.def.Recover(file, swapFilePath, buf, force)
}

/* the rest of methods default to using the default workspace */

func (m multi) ReadDir(name string) ([]os.DirEntry, error) {
	return m.def.ReadDir(name)
}

func (m multi) Open(path string, flag int, mode os.FileMode) (
	workspaceapi.File, *workspaceapi.Error,
) {
	return m.def.Open(path, flag, mode)
}

func (m multi) NewFile(fd uintptr, name string) workspaceapi.File {
	return m.def.NewFile(fd, name)
}

func (m multi) Stat(path string) (os.FileInfo, error) {
	return m.def.Stat(path)
}

func (m multi) Lstat(path string) (os.FileInfo, error) {
	return m.def.Lstat(path)
}

func (m multi) ReadLink(path string) (string, error) {
	return m.def.ReadLink(path)
}

func (m multi) Remove(path string) error {
	return m.def.Remove(path)
}

func (m multi) Rename(old, new string) error {
	return m.def.Rename(old, new)
}

func (m multi) URI(path string) (workspaceapi.URI, error) {
	return m.def.URI(path)
}

func (m multi) StartCommand(ctx context.Context, cmd workspaceapi.Cmd) (
	workspaceapi.Pid, error,
) {
	return m.def.StartCommand(ctx, cmd)
}

func (m multi) Signal(p workspaceapi.Pid, s syscall.Signal) error {
	return m.def.Signal(p, s)
}

func (m multi) NewPty(ctx context.Context) (workspaceapi.Pty, error) {
	return m.def.NewPty(ctx)
}

func (m multi) SetPtySize(p workspaceapi.Pty, width, height int) error {
	return m.def.SetPtySize(p, width, height)
}

func (m multi) Close() error {
	return m.def.Close()
}

func (m multi) loadExtraneous(
	file workspaceapi.URI, buf *cell.Buffer, swapDir workspaceapi.URI, readOnly bool,
) (FlusherCloser, error) {
	workspace, err := m.manager.AddWorkspace(m.parentCtx, workspaceapi.Dir(file))
	if err != nil {
		return nil, err
	}
	return workspace.Load(file, buf, swapDir, readOnly)
}

func (m multi) recoverExtraneous(
	file, swapFilePath workspaceapi.URI, buf *cell.Buffer, force bool,
) (FlusherCloser, error) {
	workspace, err := m.manager.AddWorkspace(m.parentCtx, workspaceapi.Dir(file))
	if err != nil {
		return nil, err
	}
	return workspace.Recover(file, swapFilePath, buf, force)
}
