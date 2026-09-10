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

package remotescheme

import (
	"errors"
	"os"
	"runtime"
	"sync/atomic"

	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

var (
	_ workspaceapi.File = (*remoteFile)(nil)

	errInvalidFd = errors.New("invalid file descriptor")
)

type remoteFile struct {
	fd         uintptr
	filename   string
	scheme     *remoteScheme
	generation uint64
	closed     atomic.Bool
}

func newRemoteFile(
	s *remoteScheme, fd uintptr, filename string, generation uint64,
) *remoteFile {
	ret := &remoteFile{
		scheme: s, fd: fd, filename: filename, generation: generation,
	}
	runtime.SetFinalizer(ret, func(f *remoteFile) {
		f.Close()
	})
	return ret
}

func (c *remoteFile) key() remoteFileKey {
	return remoteFileKey{generation: c.generation, fd: c.fd}
}

func (c *remoteFile) newFile() (workspaceapi.File, error) {
	scheme, generation, err := c.scheme.stateWithGeneration()
	if err != nil {
		return nil, err
	}
	return c.newFileForState(scheme, generation)
}

func (c *remoteFile) newFileForState(
	scheme schemeapi.Scheme, generation uint64,
) (workspaceapi.File, error) {
	if generation != c.generation {
		return nil, errInvalidFd
	}
	f := scheme.NewFile(c.fd, c.filename)
	if f == nil {
		return nil, errInvalidFd
	}
	// the returned file is transient so GC should
	// not close the remote file.
	runtime.SetFinalizer(f, nil)
	return f, nil
}

func (c *remoteFile) Read(p []byte) (n int, err error) {
	f, err := c.newFile()
	if err != nil {
		return 0, err
	}
	return f.Read(p)
}

func (c *remoteFile) ReadAt(p []byte, offset int64) (n int, err error) {
	f, err := c.newFile()
	if err != nil {
		return 0, err
	}
	return f.ReadAt(p, offset)
}

func (c *remoteFile) Write(p []byte) (n int, err error) {
	f, err := c.newFile()
	if err != nil {
		return 0, err
	}
	return f.Write(p)
}

func (c *remoteFile) Name() string {
	return c.filename
}

func (c *remoteFile) Stat() (os.FileInfo, error) {
	f, err := c.newFile()
	if err != nil {
		return nil, err
	}
	return f.Stat()
}

func (c *remoteFile) Sync() error {
	f, err := c.newFile()
	if err != nil {
		return err
	}
	return f.Sync()
}

func (c *remoteFile) Truncate(size int64) error {
	f, err := c.newFile()
	if err != nil {
		return err
	}
	return f.Truncate(size)
}

func (c *remoteFile) Fd() uintptr {
	return c.fd
}

func (c *remoteFile) Seek(offset int64, whence int) (int64, error) {
	f, err := c.newFile()
	if err != nil {
		return 0, err
	}
	return f.Seek(offset, whence)
}

func (c *remoteFile) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return nil
	}
	runtime.SetFinalizer(c, nil)
	c.scheme.files.Delete(c.key())

	f, err := c.newFile()
	if err != nil {
		if errors.Is(err, errInvalidFd) {
			return nil
		}
		return err
	}
	return f.Close()
}
