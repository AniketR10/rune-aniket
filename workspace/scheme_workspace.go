package workspace

import (
	"fmt"
	"io"
	"syscall"

	"github.com/ernestrc/blue/logging"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/debug"
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

func (w *schemeWorkspace) log(msg string, args ...interface{}) {
	debug.StandardLogger().
		WithField(logging.KeyClass, "schemeWorkspace").
		Tracef(msg, args...)
}

func (w *schemeWorkspace) Recover(
	uri, swapURI URI, buf *cell.Buffer, force bool,
) (ret FlusherCloser, err error) {
	w.log("Recover(%s, %s, %p, force=%v)", uri, swapURI, buf, force)
	defer w.log("Recover(%s, %s, %p, force=%v): %p %v",
		uri, swapURI, buf, force, ret, err)

	// force cleanup and expansion of URI paths
	// but first check if it's from this workspace
	is, err := IsWorkspaceURI(w, uri)
	if err != nil {
		return nil, fmt.Errorf("schemeWorkspace.IsWorkspaceURI: %s", err)
	}
	if !is {
		return nil, fmt.Errorf("invalid file URI %q for workspace with URI %q", uri, w.w)
	}
	is, err = IsWorkspaceURI(w, swapURI)
	if err != nil {
		return nil, fmt.Errorf("schemeWorkspace.IsWorkspaceURI: %s", err)
	}
	if !is {
		return nil, fmt.Errorf("invalid file URI %q for workspace with URI %q", swapURI, w.w)
	}
	uri, err = w.p.URI(uri.Path())
	if err != nil {
		return nil, err
	}
	swapURI, err = w.p.URI(swapURI.Path())
	if err != nil {
		return nil, err
	}

	ret, err = newFileRecover(w.p, uri.Path(), swapURI.Path(), buf, force)
	if err != nil {
		err = mapErrors(err)
		return
	}
	return
}

func (w *schemeWorkspace) Load(
	uri URI, buf *cell.Buffer, swapDir URI, readOnly bool,
) (ret FlusherCloser, err error) {
	w.log("Load(%s, %s, %p, read=%v)", uri, swapDir, buf, readOnly)
	defer w.log("Load(%s, %s, %p, read=%v): %p %v",
		uri, swapDir, buf, readOnly, ret, err)

	// force cleanup and expansion of URI paths
	// but first check if it's from this workspace
	is, err := IsWorkspaceURI(w, uri)
	if err != nil {
		return nil, fmt.Errorf("schemeWorkspace.IsWorkspaceURI: %s", err)
	}
	if !is {
		return nil, fmt.Errorf("invalid file URI %q for workspace with URI %q", uri, w.w)
	}
	is, err = IsWorkspaceURI(w, swapDir)
	if err != nil {
		return nil, fmt.Errorf("schemeWorkspace.IsWorkspaceURI: %s", err)
	}
	if !is {
		return nil, fmt.Errorf("invalid file URI %q for workspace with URI %q", swapDir, w.w)
	}

	uri, err = w.p.URI(uri.Path())
	if err != nil {
		return nil, err
	}
	swapDir, err = w.p.URI(swapDir.Path())
	if err != nil {
		return nil, err
	}

	ret, err = newFile(w.p, uri.Path(), buf, swapDir.Path(), readOnly)
	if err != nil {
		err = mapErrors(err)
		return
	}
	return
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
