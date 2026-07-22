// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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
