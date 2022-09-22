package workspace

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/ernestrc/go-tui/config"
)

// NewNopScheme returns a scheme that does nothing and workspace.Executor API panics.
func NewNopScheme(cfg config.Config, uri URI) (Scheme, error) {
	scheme := &testScheme{}
	scheme.openFunc = func(name string, flag int, perm os.FileMode) (File, *Error) {
		return nil, nil
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
	openFunc   func(name string, flag int, perm os.FileMode) (File, *Error)
	removeFunc func(name string) error
	renameFunc func(oldName, newName string) error
	statFunc   func(name string) (os.FileInfo, error)
	lstatFunc  func(name string) (os.FileInfo, error)
}

func (t *testScheme) Command(name string, arg ...string) (Pid, error) {
	panic("unimplemented")
}
func (t *testScheme) Start(Pid) error {
	panic("unimplemented")
}
func (t *testScheme) Signal(Pid, syscall.Signal) error {
	panic("unimplemented")
}
func (t *testScheme) StderrPipe(Pid) (io.ReadCloser, error) {
	panic("unimplemented")
}
func (t *testScheme) StdinPipe(Pid) (io.WriteCloser, error) {
	panic("unimplemented")
}
func (t *testScheme) StdoutPipe(Pid) (io.ReadCloser, error) {
	panic("unimplemented")
}
func (t *testScheme) Wait(Pid) error {
	panic("unimplemented")
}

func (t *testScheme) URI(path string) (URI, error) {
	return ParseURI(fmt.Sprintf("test://%s", filepath.Join("/", path)))
}

func (t *testScheme) Open(path string, flag int, perm os.FileMode) (File, *Error) {
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

func (t *testScheme) Lstat(path string) (os.FileInfo, error) {
	return t.lstatFunc(path)
}

func (t *testScheme) ReadLink(path string) (string, error) {
	return path, nil
}

func (t *testScheme) Close() error {
	return nil
}
