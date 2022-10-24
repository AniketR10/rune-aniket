package workspace

import (
	"context"
	"fmt"
	"io"
	"os"
	"syscall"

	"github.com/ernestrc/blue/iterator"
	"unstable.build/go-tui/cell"
)

var _ Workspace = (multi)(multi{})

// Multi wraps a Workspace to provide oob Recover and Load requests to other workspaces/schemes
// whether initialized or not.
func Multi(m WorkspaceManager, workspace Workspace, uri URI) Workspace {
	return newMulti(m, uri, workspace)
}

type multi struct {
	defURI  URI
	def     Workspace
	manager WorkspaceManager
}

func newMulti(manager WorkspaceManager, defURI URI, def Workspace) *multi {
	return &multi{def: def, defURI: defURI, manager: manager}
}

func (m multi) Load(file URI, buf *cell.Buffer, swapDir URI, readOnly bool) (FlusherCloser, error) {
	is, err := IsWorkspaceURI(m.def, file)
	if err != nil {
		return nil, fmt.Errorf("multi.IsWorkspaceURI: %s", err)
	}
	if !is {
		return m.loadExtraneous(file, buf, swapDir, readOnly)
	}
	return m.def.Load(file, buf, swapDir, readOnly)
}

func (m multi) Recover(file, swapFilePath URI, buf *cell.Buffer, force bool) (FlusherCloser, error) {
	is, err := IsWorkspaceURI(m.def, file)
	if err != nil {
		return nil, fmt.Errorf("multi.IsWorkspaceURI: %s", err)
	}
	if !is {
		return m.recoverExtraneous(file, swapFilePath, buf, force)
	}
	return m.def.Recover(file, swapFilePath, buf, force)
}

/* the rest of methods default to using the default workspace */

func (m multi) ListFiles(ctx context.Context) (iterator.Iterator[string], error) {
	return m.def.ListFiles(ctx)
}

func (m multi) Open(path string, flag int, mode os.FileMode) (File, *Error) {
	return m.def.Open(path, flag, mode)
}

func (m multi) Remove(path string) error {
	return m.def.Remove(path)
}

func (m multi) URI(path string) (URI, error) {
	return m.def.URI(path)
}

func (m multi) Getwd() (URI, error) {
	return m.def.Getwd()
}

func (m multi) Command(name string, arg ...string) (Pid, error) {
	return m.def.Command(name, arg...)
}

func (m multi) Start(p Pid) error {
	return m.def.Start(p)
}

func (m multi) Signal(p Pid, s syscall.Signal) error {
	return m.def.Signal(p, s)
}

func (m multi) StderrPipe(p Pid) (io.ReadCloser, error) {
	return m.def.StderrPipe(p)
}

func (m multi) StdinPipe(p Pid) (io.WriteCloser, error) {
	return m.def.StdinPipe(p)
}

func (m multi) StdoutPipe(p Pid) (io.ReadCloser, error) {
	return m.def.StdoutPipe(p)
}

func (m multi) Wait(p Pid) error {
	return m.def.Wait(p)
}

func (m multi) NewPty() (Pty, error) {
	return m.def.NewPty()
}

func (m multi) SetPtySize(p Pty, width, height int) error {
	return m.def.SetPtySize(p, width, height)
}

func (m multi) Close() error {
	return m.def.Close()
}

func (m multi) loadExtraneous(file URI, buf *cell.Buffer, swapDir URI, readOnly bool) (FlusherCloser, error) {
	workspace, err := m.manager.AddWorkspace(file)
	if err != nil {
		return nil, err
	}
	return workspace.Load(file, buf, swapDir, readOnly)
}

func (m multi) recoverExtraneous(file, swapFilePath URI, buf *cell.Buffer, force bool) (FlusherCloser, error) {
	workspace, err := m.manager.AddWorkspace(file)
	if err != nil {
		return nil, err
	}
	return workspace.Recover(file, swapFilePath, buf, force)
}
