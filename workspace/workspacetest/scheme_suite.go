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

//revive:disable:exported
package workspacetest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/api/schemeapi"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/workspace"
	"unstable.build/go-tui/workspace/walkdir"
)

func TestWorkspaceSchemeExecutor(
	t *testing.T,
	schemeFn func(t *testing.T) schemeapi.Scheme,
) {
	t.Run("command should run command and collect stdout", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()

		var out bytes.Buffer
		ch := make(chan error)
		cmd := workspaceapi.Cmd{
			Path:    "echo",
			Args:    []string{"blablabla\nblebleble"},
			Stdout:  &out,
			Watcher: workspaceapi.ChanWatcher(ch),
		}

		pid, err := scheme.StartCommand(context.Background(), cmd)
		require.NoError(t, err)
		require.NotZero(t, pid)

		require.NoError(t, <-ch)
		assert.Equal(t, "blablabla\nblebleble\n", out.String())
	})

	t.Run("command should run command and collect stderr", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()

		var out bytes.Buffer
		ch := make(chan error)
		cmd := workspaceapi.Cmd{
			Path:    "sh",
			Args:    []string{"-c", "echo blabla 1>&2"},
			Stderr:  &out,
			Watcher: workspaceapi.ChanWatcher(ch),
		}

		pid, err := scheme.StartCommand(context.Background(), cmd)
		require.NoError(t, err)
		require.NotZero(t, pid)

		require.NoError(t, <-ch)
		assert.Equal(t, "blabla\n", out.String())
	})

	t.Run("command should run command and feed stdin data", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()

		var out bytes.Buffer
		ch := make(chan error)
		cmd := workspaceapi.Cmd{
			Path:    "cat",
			Stdin:   strings.NewReader("blabla"),
			Stdout:  &out,
			Watcher: workspaceapi.ChanWatcher(ch),
		}

		pid, err := scheme.StartCommand(context.Background(), cmd)
		require.NoError(t, err)
		require.NotZero(t, pid)

		require.NoError(t, <-ch)
		assert.Equal(t, "blabla", out.String())
	})

	t.Run("should return error if command doesn't exist", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()

		ch := make(chan error)
		cmd := workspaceapi.Cmd{
			Path:    "aCommandThatShoulndtReallyExist55",
			Watcher: workspaceapi.ChanWatcher(ch),
		}

		pid, err := scheme.StartCommand(context.Background(), cmd)
		require.Zero(t, pid)
		require.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "not found"), err.Error())
	})

	t.Run("should return error if command errors ", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()

		var out bytes.Buffer
		ch := make(chan error)
		cmd := workspaceapi.Cmd{
			Path:    "cat",
			Args:    []string{"-X"},
			Stderr:  &out,
			Watcher: workspaceapi.ChanWatcher(ch),
		}

		pid, err := scheme.StartCommand(context.Background(), cmd)
		require.NoError(t, err)
		require.NotZero(t, pid)

		require.Error(t, <-ch)
		assert.True(t, strings.Contains(out.String(), "option"), out.String())
	})

	t.Run("pass env variables", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()

		var out bytes.Buffer
		ch := make(chan error)
		cmd := workspaceapi.Cmd{
			Path:    "sh",
			Args:    []string{"-c", "echo $XENV"},
			Stdout:  &out,
			Env:     []string{"XENV=myEnvVar"},
			Watcher: workspaceapi.ChanWatcher(ch),
		}

		pid, err := scheme.StartCommand(context.Background(), cmd)
		require.NoError(t, err)
		require.NotZero(t, pid)

		require.NoError(t, <-ch)
		assert.Equal(t, "myEnvVar\n", out.String())
	})

	t.Run("should cancel command if context cancels", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()

		ch := make(chan error)
		cmd := workspaceapi.Cmd{
			Path:    "sleep",
			Args:    []string{"60"},
			Watcher: workspaceapi.ChanWatcher(ch),
		}

		ctx, cancel := context.WithCancel(context.Background())

		pid, err := scheme.StartCommand(ctx, cmd)
		require.NoError(t, err)
		require.NotZero(t, pid)

		cancel()

		waitCtx, cancelWait := context.WithTimeout(
			context.Background(), 2*time.Second)
		defer cancelWait()
		select {
		case <-waitCtx.Done():
			t.Logf("failed to kill process in time")
			t.Fail()
		case err = <-ch:
			require.Error(t, err)
		}
	})
}

func TestWorkspaceSchemeFiles(
	t *testing.T,
	schemeFn func(t *testing.T) schemeapi.Scheme,
) {
	t.Run("Open", func(t *testing.T) {
		TestWorkspaceSchemeOpen(t, schemeFn, defaultCreateTestFile,
			io.ReadAll, (workspaceapi.File).Write, true)
	})
	t.Run("NewFile", func(t *testing.T) {
		TestWorkspaceSchemeNewFile(t, schemeFn, defaultCreateTestFile,
			io.ReadAll, (workspaceapi.File).Write)
	})
	t.Run("Remove", func(t *testing.T) {
		TestWorkspaceSchemeRemove(t, schemeFn, defaultCreateTestFile)
	})
	t.Run("Rename", func(t *testing.T) {
		TestWorkspaceSchemeRename(t, schemeFn, defaultCreateTestFile, io.ReadAll)
	})
	t.Run("Stat", func(t *testing.T) {
		TestWorkspaceSchemeStat(t, schemeFn, defaultCreateTestFile)
	})
	t.Run("Lstat", func(t *testing.T) {
		TestWorkspaceSchemeLstat(t, schemeFn, defaultCreateTestFile)
	})
	t.Run("Link", func(t *testing.T) {
		TestWorkspaceSchemeReadLink(t, schemeFn, defaultCreateTestFile)
	})
	t.Run("ReadDir", func(t *testing.T) {
		TestWorkspaceSchemeReadDir(t, schemeFn, defaultCreateTestFile)
	})
	t.Run("MkdirAll", func(t *testing.T) {
		TestWorkspaceSchemeMkdirAll(t, schemeFn, defaultCreateTestFile)
	})
	t.Run("workspace.ListFiles integration", func(t *testing.T) {
		TestWorkspaceSchemeListFilesIntegration(t, schemeFn, defaultCreateTestFile)
	})
	t.Run("workspace.Load integration", func(t *testing.T) {
		TestWorkspaceLoadIntegration(t, schemeFn, defaultCreateTestFile,
			readAllExceptLastEOL, (*cell.Buffer).Write)
	})
}

func readAllExceptLastEOL(r io.Reader) (data []byte, err error) {
	data, err = io.ReadAll(r)
	if err != nil {
		return
	}
	if bytes.HasSuffix(data, []byte{'\n'}) {
		data = data[:len(data)-1]
	}
	return
}

func TestWorkspaceLoadIntegration(
	t *testing.T,
	schemeFn func(t *testing.T) schemeapi.Scheme,
	createTestFile func(*testing.T, schemeapi.Scheme, string, string) (workspaceapi.File, func()),
	readAll func(io.Reader) ([]byte, error),
	write func(*cell.Buffer, []byte) (int, error),
) {
	t.Run("loads a NEW file into a buffer and flushes new data to it", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()
		uri, err := scheme.URI(".")
		require.NoError(t, err)
		wp := workspace.NewSchemeWorkspace(uri, scheme)
		fileuri := workspaceapi.Join(uri, "myFile")
		swapDir := workspaceapi.Join(uri, ".")
		buf := cell.NewBuffer()

		fc, err := wp.Load(fileuri, buf, swapDir, false)
		require.NoError(t, err)
		_, err = write(buf, []byte("newData"))
		require.NoError(t, err)

		require.NoError(t, fc.Flush())

		f, werr := scheme.Open("myFile", os.O_RDONLY, 0)
		require.Nil(t, werr)
		data, err := readAll(f)
		require.NoError(t, err)
		require.Equal(t, "newData", string(data))
	})

	t.Run("loads an existing file into a buffer and flushes new data to it", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()
		_, cleanup := createTestFile(t, scheme, "myExistingFile", "VERY ")
		defer cleanup()

		uri, err := scheme.URI(".")
		require.NoError(t, err)
		wp := workspace.NewSchemeWorkspace(uri, scheme)
		fileuri := workspaceapi.Join(uri, "myExistingFile")
		swapDir := workspaceapi.Join(uri, ".")
		buf := cell.NewBuffer()

		fc, err := wp.Load(fileuri, buf, swapDir, false)
		require.NoError(t, err)
		_, err = write(buf, []byte("short"))
		require.NoError(t, err)

		require.NoError(t, fc.Flush())

		f, werr := scheme.Open("myExistingFile", os.O_RDONLY, 0)
		require.Nil(t, werr)
		data, err := readAll(f)
		require.NoError(t, err)
		require.Equal(t, "VERY short", string(data))
	})

	t.Run("recovers an existing file into a buffer", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()
		_, cleanup1 := createTestFile(t, scheme, "file", "")
		defer cleanup1()

		_, cleanup2 := createTestFile(t, scheme, ".file.swp", "mosca")
		defer cleanup2()

		uri, err := scheme.URI(".")
		require.NoError(t, err)
		wp := workspace.NewSchemeWorkspace(uri, scheme)
		fileuri := workspaceapi.Join(uri, "file")
		swapuri := workspaceapi.Join(uri, ".file.swp")
		buf := cell.NewBuffer()

		fc, err := wp.Recover(fileuri, swapuri, buf, false)
		require.NoError(t, err)

		require.NoError(t, fc.Close())

		f, werr := scheme.Open("file", os.O_RDONLY, 0)
		require.Nil(t, werr)
		data, err := readAll(f)
		require.NoError(t, err)
		require.Equal(t, "mosca", string(data))
	})

	t.Run("recovers a file that doesn't exist yet into a buffer", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()
		_, cleanup2 := createTestFile(t, scheme, ".file.swp", "mosca")
		defer cleanup2()

		uri, err := scheme.URI(".")
		require.NoError(t, err)
		wp := workspace.NewSchemeWorkspace(uri, scheme)
		fileuri := workspaceapi.Join(uri, "file")
		swapuri := workspaceapi.Join(uri, ".file.swp")
		buf := cell.NewBuffer()

		fc, err := wp.Recover(fileuri, swapuri, buf, false)
		require.NoError(t, err)

		require.NoError(t, fc.Close())

		f, werr := scheme.Open("file", os.O_RDONLY, 0)
		require.Nil(t, werr)
		data, err := readAll(f)
		require.NoError(t, err)
		require.Equal(t, "mosca", string(data))
	})

	t.Run("flush+close should cleanup temp state such that next Load is able to flush", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()
		uri, err := scheme.URI(".")
		require.NoError(t, err)
		wp := workspace.NewSchemeWorkspace(uri, scheme)
		fileuri := workspaceapi.Join(uri, "myCloseTest")
		swapDir := workspaceapi.Join(uri, ".")

		buf := cell.NewBuffer()
		fc, err := wp.Load(fileuri, buf, swapDir, false)
		require.NoError(t, err)
		_, err = write(buf, []byte("short"))
		require.NoError(t, err)

		require.NoError(t, fc.Flush())
		require.NoError(t, fc.Close())

		buf = cell.NewBuffer()
		fc, err = wp.Load(fileuri, buf, swapDir, false)
		require.NoError(t, err)
		_, err = write(buf, []byte("short"))
		require.NoError(t, err)

		require.NoError(t, fc.Flush())

		f, werr := scheme.Open("myCloseTest", os.O_RDONLY, 0)
		require.Nil(t, werr)
		data, err := readAll(f)
		require.NoError(t, err)
		require.Equal(t, "shortshort", string(data))
	})

	t.Run("force recover an open, flushed file should work", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()

		uri, err := scheme.URI(".")
		require.NoError(t, err)
		wp := workspace.NewSchemeWorkspace(uri, scheme)
		fileuri := workspaceapi.Join(uri, "dataAtRestTest")
		swapfileuri := workspaceapi.Join(uri, ".dataAtRestTest.swp")
		swapDir := workspaceapi.Join(uri, ".")

		buf := cell.NewBuffer()
		_, err = wp.Load(fileuri, buf, swapDir, false)
		require.NoError(t, err)
		_, err = write(buf, []byte("short"))
		require.NoError(t, err)

		time.Sleep(2 * time.Second)

		buf = cell.NewBuffer()
		fc2, err := wp.Recover(fileuri, swapfileuri, buf, true)
		require.NoError(t, err)
		require.NoError(t, fc2.Flush())

		f, werr := scheme.Open("dataAtRestTest", os.O_RDONLY, 0)
		require.Nil(t, werr)
		data, err := readAll(f)
		require.NoError(t, err)
		require.Equal(t, "short", string(data))

		require.NoError(t, fc2.Close())
		require.NoError(t, f.Close())
	})

	t.Run("close should cleanup temp state such that next Load works", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()
		uri, err := scheme.URI(".")
		require.NoError(t, err)
		wp := workspace.NewSchemeWorkspace(uri, scheme)
		fileuri := workspaceapi.Join(uri, "myCloseTest")
		swapDir := workspaceapi.Join(uri, ".")

		buf := cell.NewBuffer()
		fc, err := wp.Load(fileuri, buf, swapDir, false)
		require.NoError(t, err)

		require.NoError(t, fc.Close())

		buf = cell.NewBuffer()
		fc, err = wp.Load(fileuri, buf, swapDir, false)
		require.NoError(t, err)
		_, err = write(buf, []byte("short"))
		require.NoError(t, err)

		require.NoError(t, fc.Flush())

		f, werr := scheme.Open("myCloseTest", os.O_RDONLY, 0)
		require.Nil(t, werr)
		data, err := readAll(f)
		require.NoError(t, err)
		require.Equal(t, "short", string(data))
	})
}

func defaultCreateTestFile(t *testing.T, s schemeapi.Scheme, filename, content string) (workspaceapi.File, func()) {
	file, werr := s.Open(filename, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0644)
	require.Nil(t, werr, werr.String())
	_, err := file.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, file.Sync())
	_, err = file.Seek(0, 0)
	require.NoError(t, err)
	return file, func() {
		_ = file.Close()
		_ = s.Remove(file.Name()) // best effort
	}
}

func testFile(
	t *testing.T, f workspaceapi.File,
	readAll func(io.Reader) ([]byte, error),
	writeFile func(workspaceapi.File, []byte) (int, error),
) {
	t.Run("Name returns the file name", func(t *testing.T) {
		// implementations may or may not return the full path name
		// in the case of a file scheme, absolute is returned because
		// a schemeapi.Scheme is localized to the current working directory
		// so if the path passed to Open is relative, then we need to
		// prepend the cwd.
		assert.Contains(t, f.Name(), "file")
	})

	t.Run("Read before write", func(t *testing.T) {
		data, err := readAll(f)
		require.NoError(t, err)
		assert.Equal(t, "", string(data))
	})

	t.Run("Write", func(t *testing.T) {
		_, err := writeFile(f, []byte("1234567890"))
		require.NoError(t, err)
	})

	t.Run("Read after write before sync", func(t *testing.T) {
		/* this is undefined for now */
	})

	t.Run("Sync", func(t *testing.T) {
		err := f.Sync()
		require.NoError(t, err)
	})

	t.Run("Stat", func(t *testing.T) {
		finfo, err := f.Stat()
		require.NoError(t, err)
		assert.Equal(t, "file", finfo.Name())
		assert.WithinDuration(t, finfo.ModTime(), time.Now(), 1*time.Minute)
		assert.Equal(t, false, finfo.IsDir())
		// the following are unused atm, so we don't test for them.
		// assert.Equal(t, int64(10), finfo.Size())
		// assert.Equal(t, fs.FileMode(0644), finfo.Mode())
	})

	t.Run("Seek", func(t *testing.T) {
		_, err := f.Seek(0, 0)
		require.NoError(t, err)
	})

	t.Run("Read", func(t *testing.T) {
		data, err := readAll(f)
		require.NoError(t, err)
		assert.Equal(t, "1234567890", string(data))
	})

	t.Run("Truncate", func(t *testing.T) {
		err := f.Truncate(0)
		require.NoError(t, err)
	})

	t.Run("Read after truncate", func(t *testing.T) {
		var buf [10]byte
		n, err := f.Read(buf[:])
		require.Equal(t, io.EOF, err)
		assert.Equal(t, 0, n)
	})

	t.Run("Close", func(t *testing.T) {
		require.NoError(t, f.Close())
	})
}

func TestWorkspaceSchemeNewFile(
	t *testing.T,
	schemeFn func(t *testing.T) schemeapi.Scheme,
	createTestFile func(*testing.T, schemeapi.Scheme, string, string) (workspaceapi.File, func()),
	readAll func(io.Reader) ([]byte, error),
	writeFile func(workspaceapi.File, []byte) (int, error),
) {
	t.Run("returns a working File if Open succeeds", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()
		f, err := scheme.Open("file", os.O_CREATE|os.O_RDWR, 0644)
		require.Nil(t, err, err.String())
		f = scheme.NewFile(f.Fd(), f.Name())
		testFile(t, f, readAll, writeFile)
	})

	t.Run("file offsets are kept across instances of NewFile", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()
		f, werr := scheme.Open("file", os.O_CREATE|os.O_RDWR, 0644)
		require.Nil(t, werr, werr.String())
		f = scheme.NewFile(f.Fd(), f.Name())

		_, err := writeFile(f, []byte("1234567890"))
		require.NoError(t, err)

		require.NoError(t, f.Sync())
		_, err = f.Seek(0, 0)
		require.NoError(t, err)

		var buf [8]byte
		n, err := f.Read(buf[:])
		require.NoError(t, err)
		assert.Equal(t, 8, n)
		assert.Equal(t, "12345678", string(buf[:]))

		f = scheme.NewFile(f.Fd(), f.Name())
		n, err = f.Read(buf[:])
		require.NoError(t, err)
		assert.Equal(t, 2, n)
		assert.Equal(t, "90", string(buf[:n]))

		f = scheme.NewFile(f.Fd(), f.Name())
		_, err = f.Read(buf[:])
		require.Equal(t, io.EOF, err)
	})
}

func TestWorkspaceSchemeOpen(
	t *testing.T,
	schemeFn func(t *testing.T) schemeapi.Scheme,
	createTestFile func(*testing.T, schemeapi.Scheme, string, string) (workspaceapi.File, func()),
	readAll func(io.Reader) ([]byte, error),
	writeFile func(workspaceapi.File, []byte) (int, error),
	testRelativeAbsolutePaths bool,
) {
	t.Run("returns error if O_CREATE flag is not passed and file doesn't exist", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()
		_, err := scheme.Open("file", 0, 0644)
		require.NotNil(t, err)
		assert.True(t, err.IsNotExist)
	})

	t.Run("truncates file if O_TRUNC is passed if file stored has data", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()
		_, cleanup := createTestFile(t, scheme, "file", "1234")
		defer cleanup()

		d, werr := scheme.Open("file", os.O_RDWR|os.O_TRUNC, 0)
		require.Nil(t, werr, werr.String())

		_, err := writeFile(d, []byte("zz"))
		require.NoError(t, err)

		_, err = d.Seek(0, 0)
		require.NoError(t, err)

		data, err := readAll(d)
		require.NoError(t, err)
		assert.Equal(t, "zz", string(data))

	})

	t.Run("truncates file if O_TRUNC is passed if it's a new file", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()
		d, werr := scheme.Open("file", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
		require.Nil(t, werr, werr.String())

		_, err := writeFile(d, []byte("zz"))
		require.NoError(t, err)

		_, err = d.Seek(0, 0)
		require.NoError(t, err)

		data, err := readAll(d)
		require.NoError(t, err)
		assert.Equal(t, "zz", string(data))
	})

	if testRelativeAbsolutePaths {
		t.Run("relative to cwd or absolute to cwd should be the same file", func(t *testing.T) {
			scheme := schemeFn(t)
			defer scheme.Close()
			_, cleanup := createTestFile(t, scheme, "file", "1234")
			defer cleanup()

			cwd, err := scheme.URI(".")
			require.NoError(t, err)

			d, werr := scheme.Open("file", os.O_RDONLY, 0)
			require.Nil(t, werr, werr.String())

			data, err := readAll(d)
			require.NoError(t, err)
			assert.Equal(t, "1234", string(data))

			d, werr = scheme.Open(filepath.Join(cwd.Path(), "file"), os.O_RDONLY, 0)
			require.Nil(t, werr, werr.String())

			data, err = readAll(d)
			require.NoError(t, err)
			assert.Equal(t, "1234", string(data))
		})

		t.Run("if a relative path is passed then that should be relative to the workspace cwd", func(t *testing.T) {
			scheme := schemeFn(t)
			defer scheme.Close()
			f, werr := scheme.Open("file", os.O_CREATE, 0644)
			require.Nil(t, werr, werr.String())

			cwdURI, err := scheme.URI(".")
			require.NoError(t, err)

			assert.Equal(t, filepath.Join(cwdURI.Path(), "file"), f.Name())
		})

		t.Run("if an absolute path is passed then it should access even outside of cwd", func(t *testing.T) {
			scheme := schemeFn(t)
			defer scheme.Close()
			f, err := scheme.Open("/tmp/file", os.O_CREATE, 0644)
			require.Nil(t, err, err.String())
			assert.Equal(t, "/tmp/file", f.Name())
		})
	}

	t.Run("returns error if O_EXCL|O_CREATE flag is passed and file exist", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()
		_, cleanup := createTestFile(t, scheme, "file", "")
		defer cleanup()
		_, err := scheme.Open("file", os.O_EXCL|os.O_CREATE, 0644)
		require.NotNil(t, err, err.String())
		assert.True(t, err.IsExist)
	})

	t.Run("returns a working File if Open succeeds", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()
		f, err := scheme.Open("file", os.O_CREATE|os.O_RDWR, 0644)
		require.Nil(t, err, err.String())

		testFile(t, f, readAll, writeFile)
	})
}

func TestWorkspaceSchemeRemove(
	t *testing.T,
	schemeFn func(t *testing.T) schemeapi.Scheme,
	createTestFile func(*testing.T, schemeapi.Scheme, string, string) (workspaceapi.File, func()),
) {
	t.Run("removes file", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()
		_, cleanup := createTestFile(t, scheme, "file", "")
		defer cleanup()
		require.NoError(t, scheme.Remove("file"))
	})
	t.Run("returns ErrNotExist if file has already been removed", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()
		_, cleanup := createTestFile(t, scheme, "file", "")
		defer cleanup()
		require.NoError(t, scheme.Remove("file"))
		err := scheme.Remove("file")
		require.Error(t, err)
		assert.True(t, errors.Is(err, os.ErrNotExist))
	})
	t.Run("returns ErrNotExist if file never existed", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()
		err := scheme.Remove("file")
		require.Error(t, err)
		assert.True(t, errors.Is(err, os.ErrNotExist))
	})
}

func TestWorkspaceSchemeRename(
	t *testing.T,
	schemeFn func(t *testing.T) schemeapi.Scheme,
	createTestFile func(*testing.T, schemeapi.Scheme, string, string) (workspaceapi.File, func()),
	readAll func(io.Reader) ([]byte, error),
) {
	t.Run("renames a file if target name doesn't exist", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()
		_, cleanup := createTestFile(t, scheme, "file", "bla")
		defer cleanup()

		require.NoError(t, scheme.Rename("file", "foile"))
		f, werr := scheme.Open("foile", 0, 0)
		require.Nil(t, werr, werr.String())

		data, err := readAll(f)
		require.NoError(t, err)
		assert.Equal(t, "bla", string(data))

		_, werr = scheme.Open("file", 0, 0)
		require.NotNil(t, werr, werr.String())
		assert.True(t, werr.IsNotExist)
	})

	t.Run("renames a file, overriding the target when it exists", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()
		_, cleanup := createTestFile(t, scheme, "file", "bla")
		defer cleanup()
		_, cleanup2 := createTestFile(t, scheme, "foile", "blo")
		defer cleanup2()

		require.NoError(t, scheme.Rename("file", "foile"))
		f, werr := scheme.Open("foile", 0, 0)
		require.Nil(t, werr, werr.String())

		data, err := readAll(f)
		require.NoError(t, err)
		assert.Equal(t, "bla", string(data))

		_, werr = scheme.Open("file", 0, 0)
		require.NotNil(t, werr, werr.String())
		assert.True(t, werr.IsNotExist)
	})

	t.Run("returns error if original file does not exist", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()
		err := scheme.Rename("file", "foile")
		require.Error(t, err)
		assert.True(t, errors.Is(err, os.ErrNotExist))
	})
}

func testWorkspaceSchemeStats(
	t *testing.T,
	schemeFn func(t *testing.T) schemeapi.Scheme,
	method func(schemeapi.Scheme, string) (os.FileInfo, error),
	createTestFile func(*testing.T, schemeapi.Scheme, string, string) (workspaceapi.File, func()),
) {
	t.Run("returns a valid os.FileInfo of a regular file", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()
		_, cleanup := createTestFile(t, scheme, "file", "bla")
		defer cleanup()

		finfo, err := method(scheme, "file")
		require.NoError(t, err)

		// always needs to be base name of the file
		require.Equal(t, "file", finfo.Name())
		assert.WithinDuration(t, finfo.ModTime(), time.Now(), 1*time.Minute)
		assert.False(t, finfo.IsDir())
		// do not test unused methods of os.FileInfo
		// assert.Equal(t, int64(3), finfo.Size())
		// assert.Equal(t, fs.FileMode(0644), finfo.Mode())
	})

	t.Run("returns os.ErrNotExist error if file is not found", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()
		_, err := method(scheme, "file")
		require.Error(t, err)
		assert.True(t, errors.Is(err, os.ErrNotExist))
	})
}

func TestWorkspaceSchemeStat(
	t *testing.T,
	schemeFn func(t *testing.T) schemeapi.Scheme,
	createTestFile func(*testing.T, schemeapi.Scheme, string, string) (workspaceapi.File, func()),
) {
	testWorkspaceSchemeStats(t, schemeFn, (schemeapi.Scheme).Stat, createTestFile)
}

func TestWorkspaceSchemeLstat(
	t *testing.T,
	schemeFn func(t *testing.T) schemeapi.Scheme,
	createTestFile func(*testing.T, schemeapi.Scheme, string, string) (workspaceapi.File, func()),
) {
	// clients do not use or need symlinks atm, so implementations
	// that do not support creating symlinks should not care about this.
	testWorkspaceSchemeStats(t, schemeFn, (schemeapi.Scheme).Lstat, createTestFile)
}

func TestWorkspaceSchemeReadLink(
	t *testing.T,
	schemeFn func(t *testing.T) schemeapi.Scheme,
	createTestFile func(*testing.T, schemeapi.Scheme, string, string) (workspaceapi.File, func()),
) {
	t.Run("should return error if underlying file is not a symlink", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()

		_, cleanup := createTestFile(t, scheme, "file", "")
		defer cleanup()

		_, err := scheme.ReadLink("file")
		require.Error(t, err)
	})
}

func TestWorkspaceSchemeListFilesIntegration(
	t *testing.T,
	schemeFn func(t *testing.T) schemeapi.Scheme,
	createTestFile func(*testing.T, schemeapi.Scheme, string, string) (workspaceapi.File, func()),
) {
	t.Run("os.FileInfo.IsDir on filesystem root should always return true", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()
		finfo, err := scheme.Stat("/")
		require.NoError(t, err)
		assert.True(t, finfo.IsDir())
	})

	t.Run("os.FileInfo.IsDir on workspace root should always return true", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()
		finfo, err := scheme.Stat(".")
		require.NoError(t, err)
		assert.True(t, finfo.IsDir())
	})

	t.Run("returns an empty iterator if there are no files anywhere", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()
		it, err := walkdir.ListFiles(context.Background(), scheme, "")
		require.NoError(t, err)
		_, ok := it.Next(context.Background())
		require.False(t, ok)
	})

	for _, path := range []string{".", ""} {
		t.Run(fmt.Sprintf("returns valid iterator with path %s", path), func(t *testing.T) {
			scheme := schemeFn(t)
			defer scheme.Close()
			totalFiles := 25

			cwd, err := scheme.URI(".")
			require.NoError(t, err)

			for i := 0; i < totalFiles; i++ {
				_, cleanup := createTestFile(t, scheme, "file"+strconv.Itoa(i), strconv.Itoa(i))
				defer cleanup()
			}

			it, err := walkdir.ListFiles(context.Background(), scheme, path)
			require.NoError(t, err)

			for i := 0; i < totalFiles; i++ {
				path, ok := it.Next(context.Background())
				require.True(t, ok, i)
				require.NoError(t, it.Err())
				assert.NotZero(t, path)
				assert.False(t, filepath.IsAbs(path))
				assert.Equal(t, cwd.Path(), filepath.Dir(filepath.Join(cwd.Path(), path)))
			}

			path, ok := it.Next(context.Background())
			require.False(t, ok)
			assert.NoError(t, it.Err())
			assert.Zero(t, path)
		})
	}

	t.Run("root does not exist returns no error and scans parent folder", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()
		totalFiles := 10

		cwd, err := scheme.URI(".")
		require.NoError(t, err)

		for i := 0; i < totalFiles; i++ {
			_, cleanup := createTestFile(t, scheme, "file"+strconv.Itoa(i), strconv.Itoa(i))
			defer cleanup()
		}

		it, err := walkdir.ListFiles(context.Background(), scheme, "./subfolder")
		require.NoError(t, err)

		for i := 0; i < totalFiles; i++ {
			path, ok := it.Next(context.Background())
			require.True(t, ok, i)
			require.NoError(t, it.Err())
			assert.NotZero(t, path)
			assert.False(t, filepath.IsAbs(path))
			assert.Equal(t, cwd.Path(), filepath.Dir(filepath.Join(cwd.Path(), path)))
		}

		path, ok := it.Next(context.Background())
		require.False(t, ok)
		assert.NoError(t, it.Err())
		assert.Zero(t, path)
	})

	t.Run("root is used up until base directory; full file name is ignored", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()
		totalFiles := 10

		cwd, err := scheme.URI(".")
		require.NoError(t, err)

		for i := 0; i < totalFiles; i++ {
			_, cleanup := createTestFile(t, scheme, "file"+strconv.Itoa(i), strconv.Itoa(i))
			defer cleanup()
		}

		it, err := walkdir.ListFiles(context.Background(), scheme, "./file1")
		require.NoError(t, err)

		for i := 0; i < totalFiles; i++ {
			path, ok := it.Next(context.Background())
			require.True(t, ok, i)
			require.NoError(t, it.Err())
			assert.NotZero(t, path)
			assert.False(t, filepath.IsAbs(path))
			assert.Equal(t, cwd.Path(), filepath.Dir(filepath.Join(cwd.Path(), path)))
		}

		path, ok := it.Next(context.Background())
		require.False(t, ok)
		assert.NoError(t, it.Err())
		assert.Zero(t, path)
	})

	t.Run("passing canceled context to the returned iterator's Next returns an error", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()
		totalFiles := 10

		for i := 0; i < totalFiles; i++ {
			_, cleanup := createTestFile(t, scheme, "file"+strconv.Itoa(i), strconv.Itoa(i))
			defer cleanup()
		}

		ctx, cancel := context.WithCancel(context.Background())

		it, err := walkdir.ListFiles(ctx, scheme, "./file1")
		require.NoError(t, err)

		cancel()
		_, ok := it.Next(ctx)
		require.False(t, ok)
		assert.Error(t, it.Err())
	})

	t.Run("does not return error if root's base dir does not exist", func(t *testing.T) {
		// DEPRECATED: this behaviour should not be relied upon as some implementations do
		// directories and might error with a path like fi/fi/fi
		t.SkipNow()

		scheme := schemeFn(t)
		defer scheme.Close()
		totalFiles := 10

		cwd, err := scheme.URI(".")
		require.NoError(t, err)

		for i := 0; i < totalFiles; i++ {
			_, cleanup := createTestFile(t, scheme, "file"+strconv.Itoa(i), strconv.Itoa(i))
			defer cleanup()
		}

		it, err := walkdir.ListFiles(context.Background(), scheme, "fi/fi/fi/fi")
		require.NoError(t, err)

		for i := 0; i < totalFiles; i++ {
			path, ok := it.Next(context.Background())
			require.True(t, ok, i)
			require.NoError(t, it.Err())
			assert.NotZero(t, path)
			assert.False(t, filepath.IsAbs(path))
			assert.Equal(t, cwd.Path(), filepath.Dir(filepath.Join(cwd.Path(), path)))
		}

		path, ok := it.Next(context.Background())
		require.False(t, ok)
		assert.NoError(t, it.Err())
		assert.Zero(t, path)
	})
}

func TestWorkspaceSchemeReadDir(
	t *testing.T,
	schemeFn func(t *testing.T) schemeapi.Scheme,
	createTestFile func(*testing.T, schemeapi.Scheme, string, string) (workspaceapi.File, func()),
) {
	t.Run("a file should return an error", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()

		_, cleanup := createTestFile(t, scheme, "file", "")
		defer cleanup()

		_, err := scheme.ReadDir("file")
		require.Error(t, err)
	})

	t.Run("workspace dir should return all files at the workspace root directory", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()

		_, cleanup := createTestFile(t, scheme, "file1", "")
		defer cleanup()

		_, cleanup = createTestFile(t, scheme, "file2", "")
		defer cleanup()

		entries, err := scheme.ReadDir(".")
		require.NoError(t, err)
		require.Len(t, entries, 2)
		if entries[0].Name() == "file1" {
			assert.Equal(t, "file2", entries[1].Name())
		} else {
			assert.Equal(t, "file2", entries[0].Name())
			assert.Equal(t, "file1", entries[1].Name())
		}
	})
}

func TestWorkspaceSchemeMkdirAll(
	t *testing.T,
	schemeFn func(t *testing.T) schemeapi.Scheme,
	createTestFile func(*testing.T, schemeapi.Scheme, string, string) (workspaceapi.File, func()),
) {
	t.Run("a file should return an error", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()

		_, cleanup := createTestFile(t, scheme, "./file", "")
		defer cleanup()

		f, werr := scheme.Open("./file", os.O_RDONLY, 0)
		require.Nil(t, werr)
		require.NoError(t, f.Close())

		err := scheme.MkdirAll("./file", 0700)
		require.Error(t, err)
	})

	t.Run("workspace dir should be a no-op", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()

		err := scheme.MkdirAll(".", 0700)
		require.NoError(t, err)
	})

	t.Run("nested dir and then create a nested file should always work", func(t *testing.T) {
		scheme := schemeFn(t)
		defer scheme.Close()

		err := scheme.MkdirAll("./nested/directory/very/nested", 0700)
		require.NoError(t, err)

		file, werr := scheme.Open("./nested/directory/very/nested/file", os.O_CREATE, 0666)
		require.Nil(t, werr, werr.String())
		require.NoError(t, file.Close())
	})
}
