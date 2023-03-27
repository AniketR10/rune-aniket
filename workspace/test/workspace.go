package test

import (
	"context"
	"os"
	"syscall"

	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/workspace"
)

func WorkspaceFromAPIWorkspace(
	exec workspaceapi.Executor, fs workspaceapi.FileSystem,
	t workspaceapi.Terminal,
) workspace.Workspace {
	return toWorkspace{exec: exec, fs: fs, t: t}
}

type toWorkspace struct {
	exec workspaceapi.Executor
	fs   workspaceapi.FileSystem
	t    workspaceapi.Terminal
}

func (t toWorkspace) NewFile(fd uintptr, name string) workspaceapi.File {
	return nil
}

func (t toWorkspace) URI(path string) (workspaceapi.URI, error) {
	return t.fs.URI(path)
}

func (t toWorkspace) Load(file workspaceapi.URI, buf *cell.Buffer, swapDir workspaceapi.URI, readOnly bool) (workspace.FlusherCloser, error) {
	panic("unimplemented")
}

func (t toWorkspace) Recover(file, swapFilePath workspaceapi.URI, buf *cell.Buffer, force bool) (workspace.FlusherCloser, error) {
	panic("unimplemented")
}

func (t toWorkspace) Open(path string, flag int, mode os.FileMode) (workspaceapi.File, *workspaceapi.Error) {
	return t.fs.Open(path, flag, mode)
}

func (t toWorkspace) Remove(path string) error {
	return t.fs.Remove(path)
}

func (t toWorkspace) Rename(old, new string) error {
	panic("unimplemented")
}

func (t toWorkspace) Stat(path string) (os.FileInfo, error) {
	return t.fs.Stat(path)
}
func (t toWorkspace) Lstat(path string) (os.FileInfo, error) {
	panic("unimplemented")
}

func (t toWorkspace) ReadLink(name string) (string, error) {
	panic("unimplemented")
}
func (t toWorkspace) ReadDir(name string) ([]os.DirEntry, error) {
	return t.fs.ReadDir(name)
}

func (t toWorkspace) NewPty(context.Context) (workspaceapi.Pty, error) {
	return t.t.StartPty()
}

func (t toWorkspace) SetPtySize(p workspaceapi.Pty, width, height int) error {
	return t.t.SetPtySize(p, width, height)
}

func (t toWorkspace) StartCommand(
	ctx context.Context, cmd workspaceapi.Cmd,
) (workspaceapi.Pid, error) {
	return t.exec.Start(cmd)
}

func (t toWorkspace) Signal(p workspaceapi.Pid, s syscall.Signal) error {
	return t.exec.Signal(p, s)
}

func (t toWorkspace) Close() error {
	return t.exec.Close()
}
