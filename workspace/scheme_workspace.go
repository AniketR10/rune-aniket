package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/unstablebuild/blue/logging"
	log "github.com/sirupsen/logrus"
	schemeapi "unstable.build/go-tui/api/scheme"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cell"
)

// simple Scheme-backed Workspace implementation.
type schemeWorkspace struct {
	w workspaceapi.URI
	p schemeapi.Scheme
}

// NewSchemeWorkspace wraps a schemeapi.Scheme and implements a workspace.Loader,
// effectively converting a schemeapi.Scheme into a workspace.Workspace.
func NewSchemeWorkspace(w workspaceapi.URI, p schemeapi.Scheme) Workspace {
	ret := new(schemeWorkspace)
	ret.Init(w, p)
	return ret
}

func (w *schemeWorkspace) Init(uri workspaceapi.URI, p schemeapi.Scheme) {
	w.w = uri
	w.p = p
}

func (w *schemeWorkspace) log(msg string, args ...interface{}) {
	log.WithField(logging.KeyClass, "schemeWorkspace").
		Tracef(msg, args...)
}

func (w *schemeWorkspace) Recover(
	uri, swapURI workspaceapi.URI, buf *cell.Buffer, force bool,
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
		err = fmt.Errorf("invalid URI %q for workspace with URI %q", uri, w.w)
		return
	}
	is, err = IsWorkspaceURI(w, swapURI)
	if err != nil {
		err = fmt.Errorf("IsWorkspaceURI: %s", err)
		return
	}
	if !is {
		err = fmt.Errorf("invalid URI %q for workspace with URI %q", swapURI, w.w)
		return
	}

	// turn into relative if possible
	path := workspaceapi.RelPath(w.w, uri)
	swapPath := workspaceapi.RelPath(w.w, swapURI)

	ret, err = newFileRecover(w.p, path, swapPath, buf, force)
	return
}

func (w *schemeWorkspace) Load(
	uri workspaceapi.URI, buf *cell.Buffer, swapDir workspaceapi.URI, readOnly bool,
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

	// turn into relative if possible
	path := workspaceapi.RelPath(w.w, uri)
	swapDirPath := workspaceapi.RelPath(w.w, swapDir)

	ret, err = newFile(w.p, path, buf, swapDirPath, readOnly)
	if err == os.ErrNotExist {
		if readOnly {
			err = errors.New("cannot open file that doesn't exist in read-only")
		} else {
			err = errors.New("directory structure does not support creating file")
		}
	}
	return
}

func (w *schemeWorkspace) ReadDir(name string) (
	[]os.DirEntry, error,
) {
	return w.p.ReadDir(name)
}

func (w *schemeWorkspace) Open(path string, flag int, mode os.FileMode) (
	workspaceapi.File, *workspaceapi.Error,
) {
	return w.p.Open(path, flag, mode)
}

func (w *schemeWorkspace) NewFile(fd uintptr, name string) workspaceapi.File {
	return w.p.NewFile(fd, name)
}

func (w *schemeWorkspace) Stat(path string) (os.FileInfo, error) {
	return w.p.Stat(path)
}

func (w *schemeWorkspace) Lstat(path string) (os.FileInfo, error) {
	return w.p.Lstat(path)
}

func (w *schemeWorkspace) ReadLink(path string) (string, error) {
	return w.p.ReadLink(path)
}

func (w *schemeWorkspace) Remove(path string) error {
	return w.p.Remove(path)
}

func (w *schemeWorkspace) Rename(old, new string) error {
	return w.p.Rename(old, new)
}

func (w *schemeWorkspace) URI(path string) (workspaceapi.URI, error) {
	return w.p.URI(path)
}

func (w *schemeWorkspace) StartCommand(ctx context.Context, cmd workspaceapi.Cmd) (
	pid workspaceapi.Pid, err error,
) {
	return w.p.StartCommand(ctx, cmd)
}

func (m *schemeWorkspace) Signal(pid workspaceapi.Pid, sig syscall.Signal) (err error) {
	return m.p.Signal(pid, sig)
}

func (m *schemeWorkspace) NewPty(ctx context.Context) (workspaceapi.Pty, error) {
	return m.p.NewPty(ctx)
}

func (m *schemeWorkspace) SetPtySize(p workspaceapi.Pty, width, height int) error {
	return m.p.SetPtySize(p, width, height)
}

func (m *schemeWorkspace) Close() error {
	return m.p.Close()
}
