package workspace

import (
	"io"
	"syscall"

	"unstable.build/go-tui/cell"
)

// simple Scheme-backed Workspace implementation.
type schemeWorkspace struct {
	w URI
	p Scheme
}

// NewSchemeWorkspace wraps a workspace.Scheme to return the canonical workspace.Workspace.
// It adds Getwd, which returns the given URI and adds Recover and Load, which
// return a FlusherCloser that directly uses the given Scheme to manipulate the underlying
// resources.
func NewSchemeWorkspace(w URI, p Scheme) Workspace {
	ret := new(schemeWorkspace)
	ret.Init(w, p)
	return ret
}

func (w *schemeWorkspace) Init(uri URI, p Scheme) {
	w.w = uri
	w.p = p
}

func (w *schemeWorkspace) Recover(
	uri, swapURI URI, buf *cell.Buffer, force bool,
) (FlusherCloser, error) {
	fc, err := newFileRecover(w.p, uri, swapURI, buf, force)
	if err != nil {
		return nil, mapErrors(err)
	}
	return fc, nil
}

func (w *schemeWorkspace) Load(
	uri URI, buf *cell.Buffer, swapDir URI, readOnly bool,
) (FlusherCloser, error) {
	fc, err := newFile(w.p, uri, buf, swapDir, readOnly)
	if err != nil {
		return nil, mapErrors(err)
	}
	return fc, nil
}

func (w *schemeWorkspace) Getwd() (URI, error) {
	return w.w, nil
}

func (w *schemeWorkspace) URI(path string) (URI, error) {
	return w.p.URI(path)
}

func (w *schemeWorkspace) Command(name string, arg ...string) (pid Pid, err error) {
	return w.p.Command(name, arg...)
}

func (m *schemeWorkspace) Start(pid Pid) (err error) {
	return m.p.Start(pid)
}

func (m *schemeWorkspace) Signal(pid Pid, sig syscall.Signal) (err error) {
	return m.p.Signal(pid, sig)
}

func (m *schemeWorkspace) StderrPipe(pid Pid) (ret io.ReadCloser, err error) {
	return m.p.StderrPipe(pid)
}

func (m *schemeWorkspace) StdinPipe(pid Pid) (ret io.WriteCloser, err error) {
	return m.p.StdinPipe(pid)
}

func (m *schemeWorkspace) StdoutPipe(pid Pid) (ret io.ReadCloser, err error) {
	return m.p.StdoutPipe(pid)
}

func (m *schemeWorkspace) Wait(pid Pid) (err error) {
	return m.p.Wait(pid)
}

func (m *schemeWorkspace) Close() error {
	return m.p.Close()
}
