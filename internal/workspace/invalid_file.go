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

package workspace

import (
	"os"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// InvalidFile returns a File that always returns the given err.
// This can be used by Scheme implementations to satisfy NewFile
// when callers passed an invalid file descriptor.
func InvalidFile(fd uintptr, name string, err error) workspaceapi.File {
	return &invalidFile{fd: fd, name: name, err: err}
}

type invalidFile struct {
	fd   uintptr
	name string
	err  error
}

func (c *invalidFile) Read(p []byte) (n int, err error) {
	return 0, c.err
}

func (c *invalidFile) ReadAt(p []byte, offset int64) (n int, err error) {
	return 0, c.err
}

func (c *invalidFile) Write(p []byte) (n int, err error) {
	return 0, c.err
}

func (c *invalidFile) Name() string {
	return c.name
}

func (c *invalidFile) Stat() (os.FileInfo, error) {
	return nil, c.err
}

func (c *invalidFile) Sync() error {
	return c.err
}

func (c *invalidFile) Truncate(size int64) error {
	return c.err
}

func (c *invalidFile) Fd() uintptr {
	return c.fd
}

func (c *invalidFile) Seek(offset int64, whence int) (int64, error) {
	return 0, c.err
}

func (c *invalidFile) Close() error {
	return c.err
}
