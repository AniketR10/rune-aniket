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

//revive:disable:exported
package workspacetest

import (
	"io"
	"os"
	"time"
)

// File satisfies workspaceapi.File
type File struct {
	Reads  [][]byte
	Writes [][]byte
}

func (t *File) Name() string {
	return ""
}

func (t *File) Fd() uintptr {
	return 0
}

func (t *File) Stat() (os.FileInfo, error) {
	return FileInfo{}, nil
}

func (t *File) Sync() error {
	return nil
}
func (t *File) Truncate(size int64) error {
	return nil
}

func (t *File) Seek(x int64, y int) (int64, error) {
	return 0, nil
}

func (t *File) Read(b []byte) (int, error) {
	if len(t.Reads) == 0 {
		return 0, io.EOF
	}
	copy(b, t.Reads[len(t.Reads)-1])
	t.Reads = t.Reads[:len(t.Reads)-1]
	return len(b), nil
}

func (t *File) ReadAt(b []byte, offset int64) (int, error) {
	return 0, io.EOF
}

func (t *File) Write(b []byte) (int, error) {
	n := make([]byte, len(b))
	copy(n, b)
	t.Writes = append(t.Writes, n)
	return 0, nil
}

func (t *File) Close() error {
	return nil
}

// FileInfo satisfies os.FileInfo.
type FileInfo struct {
	Filename    string
	FileIsDir   bool
	FileModTime time.Time
	FileSize    int64
	FileMode    os.FileMode
}

func (t FileInfo) Name() string {
	return t.Filename
}
func (t FileInfo) Size() int64 {
	return t.FileSize
}

func (t FileInfo) Mode() os.FileMode {
	return t.FileMode
}

func (t FileInfo) ModTime() time.Time {
	return t.FileModTime
}

func (t FileInfo) IsDir() bool {
	return t.FileIsDir
}

func (t FileInfo) Sys() any {
	return nil
}
