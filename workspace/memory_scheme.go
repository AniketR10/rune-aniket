package workspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/ernestrc/blue/iterator"
	"unstable.build/go-tui/config"
)

const (
	// FileScheme represents the local file URL scheme.
	MemoryScheme = "memory"
)

var (
	errExecute = errors.New("cannot execute commands on in-memory scheme")
)

// NewMemoryScheme returns a Scheme that manages resources in a temporary
// in-memory file system. It ignores O_RDONLY, O_WRONLY, O_RDWR Open flags
// as well as O_SYNC and O_APPEND. Seek operations on the underlying files
// only support seeking to the beginning of the file.
func NewMemoryScheme(cfg config.Config, workspace URI) (Scheme, error) {
	ret := new(memoryScheme)
	err := ret.init(workspace)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

type memoryScheme struct {
	mu        sync.Mutex
	workspace URI
	files     map[string]*memFile
}

type memFile struct {
	bytes.Buffer
	filename string
	modTime  time.Time
}

type memFileInfo struct {
	bufLen   int64
	filename string
	modTime  time.Time
}

func (m *memoryScheme) init(workspace URI) error {
	if workspace.Host() != "" || workspace.User() != "" || workspace.Scheme() != MemoryScheme {
		return errors.New("invalid file URI")
	}
	m.workspace = workspace
	m.files = make(map[string]*memFile)
	return nil
}

func (m *memoryScheme) Open(path string, flag int, _ os.FileMode) (File, *Error) {
	path = filepath.Clean(path)

	filename := filepath.Base(path)
	if filename == "" {
		return nil, NopError(fmt.Errorf("invalid file %q", path))
	}

	uri, err := m.URI(path)
	if err != nil {
		return nil, NopError(err)
	}
	uriStr := uri.String()

	m.mu.Lock()
	defer m.mu.Unlock()

	f, ok := m.files[uriStr]
	if !ok && flag&os.O_CREATE == 0 {
		return nil, &Error{IsNotExist: true}
	}
	if ok && flag&os.O_EXCL != 0 {
		return nil, &Error{IsExist: true}
	}
	if ok && flag&os.O_TRUNC != 0 {
		ok = false // force re-create
	}
	if flag&os.O_APPEND != 0 || flag&os.O_SYNC != 0 {
		return nil, NopError(errors.New("unsupported Open flag"))
	}

	if !ok {
		f = &memFile{
			filename: filename,
		}
		m.files[uriStr] = f
	}

	return f, nil
}

func (m *memoryScheme) Remove(path string) error {
	uri, err := m.URI(path)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	uriStr := uri.String()
	_, ok := m.files[uriStr]
	if !ok {
		return Error{IsNotExist: true}.ToError()
	}

	delete(m.files, uriStr)
	return nil
}

func (m *memoryScheme) Rename(old, new string) error {
	oldURI, err := m.URI(old)
	if err != nil {
		return err
	}
	newURI, err := m.URI(new)
	if err != nil {
		return err
	}
	oldURIStr := oldURI.String()
	newURIStr := newURI.String()

	m.mu.Lock()
	defer m.mu.Unlock()

	f, ok := m.files[oldURIStr]
	if !ok {
		return Error{IsNotExist: true}.ToError()
	}
	delete(m.files, oldURIStr)
	f.filename = filepath.Base(new)
	m.files[newURIStr] = f
	return nil
}

func (m *memoryScheme) Stat(path string) (os.FileInfo, error) {
	uri, err := m.URI(path)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	uriStr := uri.String()
	f, ok := m.files[uriStr]
	if !ok {
		return nil, Error{IsNotExist: true}.ToError()
	}

	return f.Stat()
}

func (m *memoryScheme) Lstat(path string) (os.FileInfo, error) {
	return m.Stat(path)
}

func (m *memoryScheme) ReadLink(path string) (string, error) {
	return path, nil
}

func (m *memoryScheme) nopUser() (*user.User, error) {
	return &user.User{}, nil
}

func (m *memoryScheme) URI(path string) (URI, error) {
	absPath, err := ExpandPath(path, m.nopUser, func() (string, error) {
		return m.workspace.Path(), nil
	})
	if err != nil {
		return URI{}, err
	}
	uriStr := "memory://" + absPath
	return ParseURI(uriStr)
}

func (m *memoryScheme) Command(name string, arg ...string) (Pid, error) {
	return 0, errExecute
}

func (m *memoryScheme) Start(pid Pid) error {
	return errExecute
}

func (m *memoryScheme) Signal(pid Pid, signal syscall.Signal) error {
	return errExecute
}

func (m *memoryScheme) StderrPipe(pid Pid) (io.ReadCloser, error) {
	return nil, errExecute
}

func (m *memoryScheme) StdinPipe(pid Pid) (io.WriteCloser, error) {
	return nil, errExecute
}

func (m *memoryScheme) StdoutPipe(pid Pid) (io.ReadCloser, error) {
	return nil, errExecute
}

func (m *memoryScheme) Wait(pid Pid) error {
	return errExecute
}

func (m *memoryScheme) NewPty() (Pty, error) {
	return Pty{}, errExecute
}

func (m *memoryScheme) SetPtySize(p Pty, width, height int) error {
	return errExecute
}

func (m *memoryScheme) ListFiles(ctx context.Context) (
	it iterator.Iterator[string], err error,
) {
	var files []string
	for uri := range m.files {
		cwdlen := len(m.workspace.String())
		files = append(files, uri[cwdlen:])
	}
	return iterator.FromSlice(files), nil
}

func (m *memoryScheme) Close() error {
	m.files = nil
	return nil
}

func (c *memFile) Read(p []byte) (n int, err error) {
	return c.Buffer.Read(p)
}

func (c *memFile) Write(p []byte) (n int, err error) {
	n, err = c.Buffer.Write(p)
	return
}

func (c *memFile) Name() string {
	return c.filename
}

func (c *memFile) Stat() (os.FileInfo, error) {
	finfo := memFileInfo{
		bufLen:   int64(c.Len()),
		filename: c.filename,
		modTime:  c.modTime,
	}
	return finfo, nil
}

func (c *memFile) Sync() error {
	c.modTime = time.Now()
	return nil
}

func (c *memFile) Truncate(size int64) error {
	c.Buffer.Truncate(int(size))
	return nil
}

func (c *memFile) Seek(offset int64, whence int) (int64, error) {
	if offset != 0 || whence != 0 {
		return 0, errors.New("unsupported Seek: only seek to beggining of file supported")
	}
	content := c.String()
	c.Buffer = bytes.Buffer{}
	n, err := c.Buffer.Write([]byte(content))
	return int64(n), err
}

func (c *memFile) Close() error {
	return nil
}

func (t memFileInfo) Size() int64 {
	return t.bufLen
}

func (t memFileInfo) Mode() os.FileMode {
	return 0
}

func (t memFileInfo) ModTime() time.Time {
	return t.modTime
}

func (t memFileInfo) IsDir() bool {
	return false
}

func (t memFileInfo) Sys() interface{} {
	return nil
}

func (t memFileInfo) Name() string {
	return t.filename
}
