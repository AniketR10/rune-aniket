// Copyright (C) 2017-2026 Unstable Build, LLC
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
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// closeCountingFile wraps a workspaceapi.File and counts Close calls.
type closeCountingFile struct {
	workspaceapi.File
	closes *atomic.Int32
}

func (f closeCountingFile) Close() error {
	f.closes.Add(1)
	return f.File.Close()
}

// blockingCloseCountReader blocks OpenFile until release is closed
// and wraps returned files with a close counter.
type blockingCloseCountReader struct {
	fsReader
	release chan struct{}
	closes  atomic.Int32
}

func (r *blockingCloseCountReader) OpenFile(
	p string, flag int, perm os.FileMode,
) (workspaceapi.File, error) {
	<-r.release
	f, err := r.fsReader.OpenFile(p, flag, perm)
	if err != nil {
		return nil, err
	}
	return closeCountingFile{File: f, closes: &r.closes}, nil
}

func TestAsyncOpenFileReturnsImmediatelyAndReadBlocks(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hi\n"), 0o644))
	r := &blockingCloseCountReader{
		fsReader: fsReader{root: dir},
		release:  make(chan struct{}),
	}

	f, err := asyncOpenReader{r: r}.OpenFile("a.txt", os.O_RDONLY, 0)
	require.NoError(t, err, "OpenFile must return without waiting for the open")

	read := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 2)
		n, _ := f.Read(buf)
		read <- buf[:n]
	}()
	select {
	case b := <-read:
		t.Fatalf("Read returned %q before the open was released", b)
	default:
	}

	close(r.release)
	assert.Equal(t, []byte("hi"), <-read)
	require.NoError(t, f.Close())
	assert.Equal(t, int32(1), r.closes.Load())
}

func TestAsyncFileCloseDuringOpenReleasesUnderlying(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hi\n"), 0o644))
	r := &blockingCloseCountReader{
		fsReader: fsReader{root: dir},
		release:  make(chan struct{}),
	}

	f, err := asyncOpenReader{r: r}.OpenFile("a.txt", os.O_RDONLY, 0)
	require.NoError(t, err)

	require.NoError(t, f.Close(), "Close must not block on the in-flight open")
	close(r.release)

	_, err = f.Read(make([]byte, 1))
	require.ErrorIs(t, err, os.ErrClosed,
		"Read after Close must fail even once the open completes")
	assert.Equal(t, int32(1), r.closes.Load(),
		"the open goroutine must release the file opened after Close")
}

func TestAsyncFileOpenErrorSurfacesOnRead(t *testing.T) {
	dir := t.TempDir()
	r := newFSReader(dir)

	f, err := asyncOpenReader{r: r}.OpenFile("missing.txt", os.O_RDONLY, 0)
	require.NoError(t, err, "open errors are deferred to the first operation")

	_, err = f.Read(make([]byte, 1))
	require.Error(t, err)
	assert.True(t, errors.Is(err, os.ErrNotExist), "got: %v", err)
	require.NoError(t, f.Close())

	_, err = io.ReadAll(f)
	require.Error(t, err)
}
