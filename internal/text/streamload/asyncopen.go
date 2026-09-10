// Copyright (C) 2017-2026 The Rune Authors
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package streamload

import (
	"os"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/rune/internal/debug"
	"unstable.build/rune/internal/workspace/walkdir"
)

var (
	_ walkdir.Reader    = asyncOpenReader{}
	_ workspaceapi.File = (*asyncFile)(nil)
)

// asyncOpenReader intercepts calls to a walkdir.Reader.OpenFile
// to run them asynchronously, returning a workspaceapi.File that in turn
// waits for the open to finish before proceeding with its calls.
type asyncOpenReader struct {
	r walkdir.Reader
}

func (a asyncOpenReader) URI(p string) (workspaceapi.URI, error) { return a.r.URI(p) }

func (a asyncOpenReader) Stat(p string) (os.FileInfo, error) { return a.r.Stat(p) }

func (a asyncOpenReader) ReadDir(p string) ([]os.DirEntry, error) { return a.r.ReadDir(p) }

func (a asyncOpenReader) OpenFile(
	path string, flag int, perm os.FileMode,
) (workspaceapi.File, error) {
	f := &asyncFile{opened: make(chan struct{})}
	go debug.CapturePanicReport(func() {
		file, err := a.r.OpenFile(path, flag, perm)
		if err != nil {
			file = nil
		}
		f.mu.Lock()
		if f.closed && file != nil {
			// The caller closed the lazy file while the open was
			// still in flight; release the freshly opened file
			// instead of leaking it.
			_ = file.Close()
			file, err = nil, os.ErrClosed
		}
		f.file, f.err = file, err
		f.mu.Unlock()
		close(f.opened)
	})
	return f, nil
}

type asyncFile struct {
	opened chan struct{}

	mu     sync.Mutex
	file   workspaceapi.File
	err    error
	closed bool
}

func (f *asyncFile) await() (workspaceapi.File, error) {
	<-f.opened
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil, os.ErrClosed
	}
	return f.file, f.err
}

func (f *asyncFile) Ready() bool {
	select {
	case <-f.opened:
		return true
	default:
		return false
	}
}

func (f *asyncFile) Close() error {
	f.mu.Lock()
	f.closed = true
	file := f.file
	f.file = nil
	f.mu.Unlock()
	if file != nil {
		return file.Close()
	}
	return nil
}

func (f *asyncFile) Read(p []byte) (int, error) {
	file, err := f.await()
	if err != nil {
		return 0, err
	}
	return file.Read(p)
}

func (f *asyncFile) ReadAt(p []byte, off int64) (int, error) {
	file, err := f.await()
	if err != nil {
		return 0, err
	}
	return file.ReadAt(p, off)
}

func (f *asyncFile) Write(p []byte) (int, error) {
	file, err := f.await()
	if err != nil {
		return 0, err
	}
	return file.Write(p)
}

func (f *asyncFile) Seek(offset int64, whence int) (int64, error) {
	file, err := f.await()
	if err != nil {
		return 0, err
	}
	return file.Seek(offset, whence)
}

func (f *asyncFile) Name() string {
	file, err := f.await()
	if err != nil {
		return ""
	}
	return file.Name()
}

func (f *asyncFile) Stat() (os.FileInfo, error) {
	file, err := f.await()
	if err != nil {
		return nil, err
	}
	return file.Stat()
}

func (f *asyncFile) Sync() error {
	file, err := f.await()
	if err != nil {
		return err
	}
	return file.Sync()
}

func (f *asyncFile) Truncate(size int64) error {
	file, err := f.await()
	if err != nil {
		return err
	}
	return file.Truncate(size)
}

func (f *asyncFile) Fd() uintptr {
	file, err := f.await()
	if err != nil {
		return 0
	}
	return file.Fd()
}
