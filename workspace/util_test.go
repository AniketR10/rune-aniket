package workspace

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"unstable.build/go-tui/api/config"
	schemeapi "unstable.build/go-tui/api/scheme"
	workspaceapi "unstable.build/go-tui/api/workspace"
)

func newTestScheme(scheme string) schemeapi.SchemeFunc {
	return func(_ context.Context, cfg config.Config, uri workspaceapi.URI) (schemeapi.Scheme, error) {
		scheme := &testScheme{scheme: scheme}
		scheme.openFunc = func(name string, flag int, perm os.FileMode) (workspaceapi.File, *workspaceapi.Error) {
			return testFile{}, nil
		}
		scheme.removeFunc = func(name string) error {
			return nil
		}
		scheme.renameFunc = func(oldName, newName string) error {
			return nil
		}
		scheme.statFunc = func(name string) (os.FileInfo, error) {
			// best effort
			return testFileInfo{isDir: !strings.Contains(name, ".")}, nil
		}
		scheme.lstatFunc = func(name string) (os.FileInfo, error) {
			return testFileInfo{}, nil
		}
		return scheme, nil
	}
}

type testFile struct {
}

func (t testFile) Name() string {
	return ""
}

func (t testFile) Stat() (os.FileInfo, error) {
	return testFileInfo{}, nil
}

func (t testFile) Sync() error {
	return nil
}

func (t testFile) Fd() uintptr {
	return 0
}

func (t testFile) Truncate(size int64) error {
	return nil
}

func (t testFile) Seek(x int64, y int) (int64, error) {
	return 0, nil
}

func (t testFile) Read(b []byte) (int, error) {
	return 0, io.EOF
}

func (t testFile) Write(b []byte) (int, error) {
	return 0, nil
}

func (t testFile) Close() error {
	return nil
}

// implements os.FileInfo
type testFileInfo struct {
	name    string
	isDir   bool
	modTime time.Time
	size    int64
	mode    os.FileMode
}

func (t testFileInfo) Name() string {
	return t.name
}
func (t testFileInfo) Size() int64 {
	return t.size
}

func (t testFileInfo) Mode() os.FileMode {
	return t.mode
}

func (t testFileInfo) ModTime() time.Time {
	return t.modTime
}

func (t testFileInfo) IsDir() bool {
	return t.isDir
}

func (t testFileInfo) Sys() interface{} {
	return nil
}

type testScheme struct {
	scheme     string
	openFunc   func(name string, flag int, perm os.FileMode) (workspaceapi.File, *workspaceapi.Error)
	removeFunc func(name string) error
	renameFunc func(oldName, newName string) error
	statFunc   func(name string) (os.FileInfo, error)
	lstatFunc  func(name string) (os.FileInfo, error)
}

func (t *testScheme) StartCommand(ctx context.Context, cmd workspaceapi.Cmd) (
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

func (t *testScheme) NewPty(ctx context.Context) (ret workspaceapi.Pty, err error) {
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
