package workspace

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"

	"github.com/ernestrc/blue/iterator"
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
		err = fmt.Errorf("IsWorkspaceURI: %s", err)
		return
	}
	if !is {
		err = fmt.Errorf("invalid file URI %q for workspace with URI %q", uri, w.w)
		return
	}
	is, err = IsWorkspaceURI(w, swapURI)
	if err != nil {
		err = fmt.Errorf("IsWorkspaceURI: %s", err)
		return
	}
	if !is {
		err = fmt.Errorf("invalid file URI %q for workspace with URI %q", swapURI, w.w)
		return
	}
	uri, err = w.p.URI(uri.Path())
	if err != nil {
		return
	}
	swapURI, err = w.p.URI(swapURI.Path())
	if err != nil {
		return
	}

	ret, err = newFileRecover(w.p, uri.Path(), swapURI.Path(), buf, force)
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
		err = fmt.Errorf("IsWorkspaceURI: %s", err)
		return
	}
	if !is {
		err = fmt.Errorf("invalid file URI %q for workspace with URI %q", uri, w.w)
		return
	}
	is, err = IsWorkspaceURI(w, swapDir)
	if err != nil {
		err = fmt.Errorf("IsWorkspaceURI: %s", err)
		return
	}
	if !is {
		err = fmt.Errorf("invalid file URI %q for workspace with URI %q", swapDir, w.w)
		return
	}

	uri, err = w.p.URI(uri.Path())
	if err != nil {
		return
	}
	swapDir, err = w.p.URI(swapDir.Path())
	if err != nil {
		return
	}

	ret, err = newFile(w.p, uri.Path(), buf, swapDir.Path(), readOnly)
	if err == os.ErrNotExist {
		if readOnly {
			err = errors.New("cannot open file that doesn't exist in read-only")
		} else {
			err = errors.New("directory structure does not support creating file")
		}
	}
	return
}

func (w *schemeWorkspace) ListFiles(ctx context.Context) (iterator.Iterator[string], error) {
	return w.p.ListFiles(ctx)
}

func (w *schemeWorkspace) Open(path string, flag int, mode os.FileMode) (File, *Error) {
	return w.p.Open(path, flag, mode)
}

func (w *schemeWorkspace) Remove(path string) error {
	return w.p.Remove(path)
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

func (m *schemeWorkspace) NewPty() (Pty, error) {
	return m.p.NewPty()
}

func (m *schemeWorkspace) SetPtySize(p Pty, width, height int) error {
	return m.p.SetPtySize(p, width, height)
}

func (m *schemeWorkspace) Wait(pid Pid) (err error) {
	return m.p.Wait(pid)
}

func (m *schemeWorkspace) Close() error {
	return m.p.Close()
}
