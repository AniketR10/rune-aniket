package workspace

import (
	"io"
	"syscall"

	"unstable.build/go-tui/cell"
)

var _ Workspace = (multi)(multi{})

// multi wraps a Workspace to provide oob Recover and Load requests to other workspaces/schemes
// whether initialized or not.
type multi struct {
	defURI  URI
	def     Workspace
	manager *Manager
}

func newMulti(manager *Manager, defURI URI, def Workspace) *multi {
	return &multi{def: def, defURI: defURI, manager: manager}
}

func (m multi) Load(file URI, buf *cell.Buffer, swapDir URI, readOnly bool) (FlusherCloser, error) {
	if !IsWorkspaceURI(m.def, file) {
		return m.loadExtraneous(file, buf, swapDir, readOnly)
	}
	return m.def.Load(file, buf, swapDir, readOnly)
}

func (m multi) Recover(file, swapFilePath URI, buf *cell.Buffer, force bool) (FlusherCloser, error) {
	if !IsWorkspaceURI(m.def, file) {
		return m.recoverExtraneous(file, swapFilePath, buf, force)
	}
	return m.def.Recover(file, swapFilePath, buf, force)
}

/* the rest of methods default to using the default workspace */

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

func (m multi) Close() error {
	return m.def.Close()
}

func (m multi) loadExtraneous(file URI, buf *cell.Buffer, swapDir URI, readOnly bool) (FlusherCloser, error) {
	workspace, ok := m.manager.WorkspaceFile(file)
	if ok {
		return workspace.Load(file, buf, swapDir, readOnly)
	}

	workspace, err := m.manager.AddWorkspace(file)
	if err != nil {
		return nil, err
	}
	return workspace.Load(file, buf, swapDir, readOnly)
}

func (m multi) recoverExtraneous(file, swapFilePath URI, buf *cell.Buffer, force bool) (FlusherCloser, error) {
	workspace, ok := m.manager.WorkspaceFile(file)
	if ok {
		return workspace.Recover(file, swapFilePath, buf, force)
	}

	workspace, err := m.manager.AddWorkspace(file)
	if err != nil {
		return nil, err
	}
	return workspace.Recover(file, swapFilePath, buf, force)
}
