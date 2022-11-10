package workspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
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
// in-memory file system. It does not enforce O_RDONLY, O_WRONLY, O_RDWR Open flags
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

// NewMemoryFile returns a in-memory File implementation.
func NewMemoryFile(filename string, mode fs.FileMode, data []byte) File {
	return &memFile{
		filename: filename,
		mode:     mode,
		modTime:  time.Now(),
		data:     data,
		reader:   bytes.NewReader(data),
	}
}

type memoryScheme struct {
	mu        sync.Mutex
	workspace URI
	files     map[string]*memFile
}

type memFile struct {
	reader   *bytes.Reader
	data     []byte
	filename string
	modTime  time.Time
	mode     os.FileMode
	offset   int64
}

type memFileInfo struct {
	bufLen   int64
	filename string
	mode     os.FileMode
	modTime  time.Time
}

func (m *memoryScheme) init(workspace URI) error {
	if workspace.Host() != "" || workspace.User() != "" || workspace.Scheme() != MemoryScheme {
		return errors.New("invalid memory URI")
	}
	m.workspace = workspace
	m.files = make(map[string]*memFile)
	return nil
}

func (m *memoryScheme) Open(path string, flag int, mode os.FileMode) (File, *Error) {
	path = filepath.Clean(path)
	if path == "" {
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
	if ok && flag&os.O_CREATE != 0 && flag&os.O_EXCL != 0 {
		return nil, &Error{IsExist: true}
	}
	if ok && flag&os.O_TRUNC != 0 {
		ok = false // force re-create
	}
	if flag&os.O_APPEND != 0 || flag&os.O_SYNC != 0 {
		return nil, NopError(errors.New("unsupported Open flag"))
	}

	if !ok {
		data := make([]byte, 0)
		var filename string
		rel, err := filepath.Rel(m.workspace.Path(), path)
		if err != nil {
			filename = filepath.Join(m.workspace.Path(), path)
		} else {
			filename = filepath.Join(m.workspace.Path(), rel)
		}
		f = NewMemoryFile(filename, mode, data).(*memFile)
		m.files[uriStr] = f
	} else {
		_, _ = f.Seek(0, 0)
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
	return "", errors.New("path is not a link")
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
		// strip URI
		cwdlen := len(m.workspace.String())
		// Rel(Join(Base ensures that path is always relative to base workspace path
		filename, _ := filepath.Rel(m.workspace.Path(), filepath.Join(m.workspace.Path(), filepath.Base(uri[cwdlen:])))
		files = append(files, filename)
	}
	return iterator.FromSlice(files), nil
}

func (m *memoryScheme) Close() error {
	m.files = nil
	return nil
}

func (c *memFile) Read(p []byte) (n int, err error) {
	return c.reader.Read(p)
}

func (c *memFile) Write(p []byte) (n int, err error) {
	if c.offset+int64(len(p)) > int64(len(c.data)) {
		diff := c.offset + int64(len(p)) - int64(len(c.data))
		c.data = append(c.data, make([]byte, diff)...)
		copy(c.data[diff:], c.data)
	}
	copy(c.data[c.offset:], p)
	n = len(p)
	c.offset += int64(n)
	c.reader.Reset(c.data)
	_, err = c.reader.Seek(c.offset, io.SeekStart)
	return
}

func (c *memFile) Name() string {
	return c.filename
}

func (c *memFile) Stat() (os.FileInfo, error) {
	finfo := memFileInfo{
		bufLen:   int64(len(c.data)),
		filename: filepath.Base(c.filename),
		modTime:  c.modTime,
		mode:     c.mode,
	}
	return finfo, nil
}

func (c *memFile) Sync() error {
	c.modTime = time.Now()
	return nil
}

func (c *memFile) Truncate(size int64) error {
	if size < 0 || size > int64(len(c.data)) {
		panic("invalid truncate size")
	}
	c.data = c.data[:size]
	c.offset = 0
	c.reader.Reset(c.data)
	return nil
}

func (c *memFile) Seek(offset int64, whence int) (int64, error) {
	var err error
	c.offset, err = c.reader.Seek(offset, whence)
	if err != nil {
		return 0, err
	}
	return c.offset, nil
}

func (c *memFile) Close() error {
	return nil
}

func (t memFileInfo) Size() int64 {
	return t.bufLen
}

func (t memFileInfo) Mode() os.FileMode {
	return t.mode
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
