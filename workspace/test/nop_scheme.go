package test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/workspace"
)

// NewNopScheme returns a scheme that does nothing and workspace.Executor API panics.
func NewNopScheme(scheme string) workspace.SchemeFunc {
	return func(ctx context.Context, cfg config.Config, uri workspaceapi.URI) (workspace.Scheme, error) {
		scheme := &testScheme{scheme: scheme}
		scheme.openFunc = func(name string, flag int, perm os.FileMode) (
			workspaceapi.File, *workspaceapi.Error,
		) {
			return File{}, nil
		}
		scheme.removeFunc = func(name string) error {
			return nil
		}
		scheme.renameFunc = func(oldName, newName string) error {
			return nil
		}
		scheme.statFunc = func(name string) (os.FileInfo, error) {
			// best effort
			return FileInfo{FileIsDir: !strings.Contains(name, ".")}, nil
		}
		scheme.lstatFunc = func(name string) (os.FileInfo, error) {
			return FileInfo{}, nil
		}
		return scheme, nil
	}
}

type testScheme struct {
	scheme     string
	openFunc   func(name string, flag int, perm os.FileMode) (workspaceapi.File, *workspaceapi.Error)
	removeFunc func(name string) error
	renameFunc func(oldName, newName string) error
	statFunc   func(name string) (os.FileInfo, error)
	lstatFunc  func(name string) (os.FileInfo, error)
}

func (t *testScheme) Command(ctx context.Context, name string, arg ...string) (workspaceapi.Pid, error) {
	panic("unimplemented")
}
func (t *testScheme) StartCommand(context.Context, workspaceapi.Cmd) (
	workspaceapi.Pid, error,
) {
	panic("unimplemented")
}
func (t *testScheme) Signal(workspaceapi.Pid, syscall.Signal) error {
	panic("unimplemented")
}

func (t *testScheme) NewFile(fd uintptr, name string) workspaceapi.File {
	panic("unimplemented")
}

func (t *testScheme) URI(path string) (workspaceapi.URI, error) {
	return workspaceapi.ParseURI(fmt.Sprintf("%s://%s", t.scheme, filepath.Join("/", path)))
}

func (t *testScheme) Open(path string, flag int, perm os.FileMode) (workspaceapi.File, *workspaceapi.Error) {
	return t.openFunc(path, flag, perm)
}

func (t *testScheme) Remove(path string) error {
	return t.removeFunc(path)
}

func (t *testScheme) Rename(old, new string) error {
	return t.renameFunc(old, new)
}

func (t *testScheme) Stat(path string) (os.FileInfo, error) {
	return t.statFunc(path)
}

func (t *testScheme) NewPty(context.Context) (ret workspaceapi.Pty, err error) {
	panic("unimplemented")
}

func (t *testScheme) SetPtySize(p workspaceapi.Pty, width, height int) (err error) {
	panic("unimplemented")
}

func (t *testScheme) Lstat(path string) (os.FileInfo, error) {
	return t.lstatFunc(path)
}

func (t *testScheme) ReadLink(path string) (string, error) {
	return path, nil
}

func (t *testScheme) ReadDir(string) (
	[]os.DirEntry, error,
) {
	panic("unimplemented")
}

func (t *testScheme) Close() error {
	return nil
}
