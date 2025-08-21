// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

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
	"unstable.build/go-tui/api/schemeapi"
	"unstable.build/go-tui/api/workspaceapi"
)

const (
	// MemoryScheme represents an in-memory URI scheme.
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
	workspace      workspaceapi.URI
	mu             sync.Mutex
	files          map[string]*memFile
	fd             uintptr // next fd
	watchpoints    map[workspaceapi.Event][]chan<- workspaceapi.EventInfo
	watchpointIDs  map[int]chan<- workspaceapi.EventInfo
	nextWatchpoint int
}

func (m *memoryScheme) init(workspace workspaceapi.URI) error {
	if workspace.Host() != "" || workspace.User() != "" || workspace.Scheme() != MemoryScheme {
		return errors.New("invalid memory URI")
	}
	m.workspace = workspace
	m.watchpoints = make(map[workspaceapi.Event][]chan<- workspaceapi.EventInfo)
	m.watchpointIDs = make(map[int]chan<- workspaceapi.EventInfo)
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
		f.m = m
		m.mu.Lock()
		m.files[uriStr] = f
		watchpoints := m.watchpoints[workspaceapi.Create]
		copied := make([]chan<- workspaceapi.EventInfo, len(watchpoints))
		copy(copied, watchpoints)
		m.mu.Unlock()
		for _, wp := range copied {
			fi := watchFileInfo{
				event:   workspaceapi.Create,
				uri:     uri,
			}
			wp <- fi
		}
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

	uriStr := uri.String()
	_, ok := m.files[uriStr]
	if !ok {
		m.mu.Unlock()
		return workspaceapi.Error{IsNotExist: true}.ToError()
	}

	delete(m.files, uriStr)
	watchpoints := m.watchpoints[workspaceapi.Remove]
	copied := make([]chan<- workspaceapi.EventInfo, len(watchpoints))
	copy(copied, watchpoints)
	m.mu.Unlock()

	for _, wp := range copied {
		fi := watchFileInfo{
			event:   workspaceapi.Remove,
			uri:     uri,
		}
		wp <- fi
	}
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

	f, ok := m.files[oldURIStr]
	if !ok {
		m.mu.Unlock()
		return workspaceapi.Error{IsNotExist: true}.ToError()
	}
	delete(m.files, oldURIStr)
	f.filename = filepath.Base(new)
	m.files[newURIStr] = f
	watchpoints := m.watchpoints[workspaceapi.Rename]
	copied := make([]chan<- workspaceapi.EventInfo, len(watchpoints))
	copy(copied, watchpoints)
	m.mu.Unlock()

	for _, wp := range copied {
		fis := []watchFileInfo{
			{
				event:   workspaceapi.Rename,
				uri:     oldURI,
			},
			{
				event:   workspaceapi.Rename,
				uri:     newURI,
			},
		}
		for _, fi := range fis {
			wp <- fi
		}
	}
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

func (m *memoryScheme) MkdirAll(path string, perm os.FileMode) error {
	// no-op, but provide error if file exists
	finfo, err := m.Stat(path)
	if err == nil && !finfo.IsDir() {
		return &os.PathError{Op: "mkdir", Path: path, Err: syscall.ENOTDIR}
	}
	return nil
}

func (m *memoryScheme) Watch(
	path string, c chan<- workspaceapi.EventInfo, events ...workspaceapi.Event,
) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextWatchpoint++
	id := m.nextWatchpoint
	m.watchpointIDs[id] = c
	for _, event := range events {
		m.watchpoints[event] = append(m.watchpoints[event], c)
	}
	return id, nil
}

func (m *memoryScheme) StopWatch(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.watchpointIDs[id]
	if !ok {
		return errors.New("watchpoint not found")
	}
	delete(m.watchpointIDs, id)

	for event, chs := range m.watchpoints {
		for i, ch := range chs {
			if ch == c {
				// remove channel from list of channels
				chs[i] = chs[len(chs)-1]
				chs = chs[:len(chs)-1]
				break
			}
		}
		m.watchpoints[event] = chs
	}
	return nil
}

func (m *memoryScheme) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, chs := range m.watchpoints {
		for _, ch := range chs {
			close(ch)
		}
	}

	m.files = nil
	m.watchpoints = nil
	m.watchpointIDs = nil
	return nil
}

type watchFileInfo struct {
	event   workspaceapi.Event
	uri     workspaceapi.URI
}

func (w watchFileInfo) Event() workspaceapi.Event {
	return w.event
}

func (w watchFileInfo) URI() workspaceapi.URI {
	return w.uri
}

func (w watchFileInfo) Sys() interface{} {
	return nil
}

func (w watchFileInfo) IsDir() (bool, error) { return false, nil }
