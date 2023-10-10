package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"

	"unstable.build/go-tui/api/config"
	schemeapi "unstable.build/go-tui/api/scheme"
	workspaceapi "unstable.build/go-tui/api/workspace"
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
func NewMemoryScheme(
	ctx context.Context, cfg config.Config, workspace workspaceapi.URI,
) (schemeapi.Scheme, error) {
	ret := new(memoryScheme)
	err := ret.init(workspace)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

type memoryScheme struct {
	workspace workspaceapi.URI
	mu        sync.Mutex
	files     map[string]*memFile
	fd        uintptr // next fd
}

func (m *memoryScheme) init(workspace workspaceapi.URI) error {
	if workspace.Host() != "" || workspace.User() != "" || workspace.Scheme() != MemoryScheme {
		return errors.New("invalid memory URI")
	}
	m.workspace = workspace
	m.files = make(map[string]*memFile)
	return nil
}

func (m *memoryScheme) NewFile(fd uintptr, filename string) workspaceapi.File {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, f := range m.files {
		if f.fd == fd {
			return f
		}
	}

	return InvalidFile(fd, filename, errors.New("invalid file descriptor"))
}

func (m *memoryScheme) Open(path string, flag int, mode os.FileMode) (
	workspaceapi.File, *workspaceapi.Error,
) {
	path = filepath.Clean(path)
	if path == "" {
		return nil, workspaceapi.NopError(fmt.Errorf("invalid file %q", path))
	}

	uri, err := m.URI(path)
	if err != nil {
		return nil, workspaceapi.NopError(err)
	}
	uriStr := uri.String()

	m.mu.Lock()
	f, ok := m.files[uriStr]
	m.mu.Unlock()
	if !ok && flag&os.O_CREATE == 0 {
		return nil, &workspaceapi.Error{IsNotExist: true}
	}
	if ok && flag&os.O_CREATE != 0 && flag&os.O_EXCL != 0 {
		return nil, &workspaceapi.Error{IsExist: true}
	}
	if ok && flag&os.O_TRUNC != 0 {
		ok = false // force re-create
	}
	if flag&os.O_APPEND != 0 || flag&os.O_SYNC != 0 {
		return nil, workspaceapi.NopError(errors.New("unsupported Open flag"))
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
		m.fd++
		f = NewMemoryFile(filename, m.fd, mode, data, &m.mu).(*memFile)
		m.mu.Lock()
		m.files[uriStr] = f
		m.mu.Unlock()
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
		return workspaceapi.Error{IsNotExist: true}.ToError()
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
		return workspaceapi.Error{IsNotExist: true}.ToError()
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

	// special cases, should always be a dir
	if uri.Path() == "/" {
		return memFileInfo{
			filename: "/",
			isDir:    true,
		}, nil
	}

	if uri.Equal(m.workspace) {
		return memFileInfo{
			filename: m.workspace.Path(),
			isDir:    true,
		}, nil
	}

	uriStr := uri.String()
	m.mu.Lock()
	f, ok := m.files[uriStr]
	m.mu.Unlock()
	if !ok {
		return nil, workspaceapi.Error{IsNotExist: true}.ToError()
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

func (m *memoryScheme) URI(path string) (workspaceapi.URI, error) {
	absPath, err := workspaceapi.ExpandPath(path, m.nopUser, func() (string, error) {
		return m.workspace.Path(), nil
	})
	if err != nil {
		return workspaceapi.URI{}, err
	}
	uriStr := "memory://" + absPath
	return workspaceapi.ParseURI(uriStr)
}

func (m *memoryScheme) StartCommand(ctx context.Context, cmd workspaceapi.Cmd) (
	workspaceapi.Pid, error,
) {
	return 0, errExecute
}

func (m *memoryScheme) Signal(pid workspaceapi.Pid, signal syscall.Signal) error {
	return errExecute
}

func (m *memoryScheme) NewPty(ctx context.Context) (workspaceapi.Pty, error) {
	return workspaceapi.Pty{}, errExecute
}

func (m *memoryScheme) SetPtySize(p workspaceapi.Pty, width, height int) error {
	return errExecute
}

func (m *memoryScheme) ReadDir(name string) (
	[]os.DirEntry, error,
) {
	info, err := m.Stat(name)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("not a directory")
	}

	name = info.Name()

	m.mu.Lock()
	defer m.mu.Unlock()

	var ret []os.DirEntry
	for uri := range m.files {
		uri, err := workspaceapi.ParseURI(uri)
		if err != nil {
			panic("could not parse internal uri")
		}
		path := uri.Path()
		if !strings.HasPrefix(path, name) {
			continue
		}
		// Rel(Join(Base ensures that path is always relative to base workspace path
		filename, _ := filepath.Rel(m.workspace.Path(), filepath.Join(m.workspace.Path(), filepath.Base(path)))
		ret = append(ret, memFileInfo{
			filename: filename,
		})
	}
	// deterministic output
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].(memFileInfo).filename > ret[j].(memFileInfo).filename
	})
	return ret, nil
}

func (m *memoryScheme) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.files = nil
	return nil
}
