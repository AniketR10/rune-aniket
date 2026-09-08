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

package walkdir

import (
	"context"
	"errors"
	"net"
	"strconv"
	"sync"
	"time"

	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"go.uber.org/goleak"
	"unstable.build/rune/internal/workspace"
	"unstable.build/rune/internal/workspace/walkdir"
)

func assertIteratorEqual(
	t *testing.T, expected []string, it iterator.Iterator[string],
) {
	var actual []string
	for {
		next, ok := it.Next(context.Background())
		if !ok {
			break
		}
		actual = append(actual, next)
	}
	require.NoError(t, it.Err())
	require.NoError(t, it.Close())

	sort.Strings(actual)
	sort.Strings(expected)
	assert.Equal(t, actual, expected)
}

// gatedStatReader blocks every Stat call until gate is closed,
// emulating a wedged remote scheme where Stat is an RPC that never
// returns.
type gatedStatReader struct {
	walkdir.Reader
	gate <-chan struct{}
}

func (r gatedStatReader) Stat(path string) (os.FileInfo, error) {
	<-r.gate
	return r.Reader.Stat(path)
}

type errStatReader struct {
	walkdir.Reader
	err error
}

func (r errStatReader) Stat(string) (os.FileInfo, error) { return nil, r.err }

type asyncListResult struct {
	it  iterator.Iterator[string]
	err error
}

// listAsync runs a walkdir constructor on its own goroutine and fails
// the test if it does not return promptly: constructors must never
// block on workspace I/O — the returned iterator is what blocks.
func listAsync(
	t *testing.T,
	fn func(context.Context, walkdir.Reader, string) (iterator.Iterator[string], error),
	r walkdir.Reader, root string,
) asyncListResult {
	t.Helper()
	resCh := make(chan asyncListResult, 1)
	go func() {
		it, err := fn(context.Background(), r, root)
		resCh <- asyncListResult{it, err}
	}()
	select {
	case res := <-resCh:
		return res
	case <-time.After(5 * time.Second):
		t.Fatal("constructor blocked on root Stat instead of returning the iterator")
		return asyncListResult{}
	}
}

func TestListFiles(t *testing.T) {

	t.Run("lists all files under workspace as relative", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		dir, err := os.MkdirTemp("", "")
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = os.RemoveAll(dir)
		})
		for _, path := range []string{".", dir} {
			t.Run(path, func(t *testing.T) {
				uri, err := workspaceapi.CurrentUserHostURI(dir)
				require.NoError(t, err)

				_, err = os.OpenFile(filepath.Join(dir, "a"), os.O_CREATE, 0666)
				require.NoError(t, err)

				_, err = os.OpenFile(filepath.Join(dir, "b"), os.O_CREATE, 0666)
				require.NoError(t, err)

				scheme, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), uri)
				require.NoError(t, err)

				it, err := walkdir.ListFiles(context.Background(), scheme, path)
				assertIteratorEqual(t, []string{"a", "b"}, it)

				require.NoError(t, scheme.Close())
			})
		}
	})

	t.Run("lists only regular files", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		dir, err := os.MkdirTemp("", "")
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = os.RemoveAll(dir)
		})
		for _, path := range []string{".", dir} {
			t.Run(path, func(t *testing.T) {
				uri, err := workspaceapi.CurrentUserHostURI(dir)
				require.NoError(t, err)

				_, err = os.OpenFile(filepath.Join(dir, "a"), os.O_CREATE, 0666)
				require.NoError(t, err)

				l, err := net.Listen("unix", filepath.Join(dir, "b"))
				require.NoError(t, err)
				defer l.Close()

				scheme, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), uri)
				require.NoError(t, err)

				it, err := walkdir.ListFiles(context.Background(), scheme, path)
				assertIteratorEqual(t, []string{"a"}, it)

				require.NoError(t, scheme.Close())
			})
		}
	})

	t.Run("Close before scanning all should abort", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		dir, err := os.MkdirTemp("", "")
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = os.RemoveAll(dir)
		})
		uri, err := workspaceapi.CurrentUserHostURI(dir)
		require.NoError(t, err)

		const n = 1000
		for i := range n {
			subdir := filepath.Join(dir, strconv.Itoa(i))
			require.NoError(t, os.MkdirAll(subdir, 0777))
			_, err = os.OpenFile(filepath.Join(subdir, "a"), os.O_CREATE, 0666)
			require.NoError(t, err)
		}

		scheme, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), uri)
		require.NoError(t, err)

		it, err := walkdir.ListFiles(context.Background(), scheme, dir)
		require.NoError(t, it.Close())

		require.NoError(t, scheme.Close())
	})

	t.Run("lists all files under non-workspace dir as absolute", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		workspaceDir, err := os.MkdirTemp("", "")
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = os.RemoveAll(workspaceDir)
		})

		dir, err := os.MkdirTemp("", "")
		require.NoError(t, err)

		uri, err := workspaceapi.CurrentUserHostURI(workspaceDir)
		require.NoError(t, err)

		f1, err := os.OpenFile(filepath.Join(dir, "a"), os.O_CREATE, 0666)
		require.NoError(t, err)

		f2, err := os.OpenFile(filepath.Join(dir, "b"), os.O_CREATE, 0666)
		require.NoError(t, err)

		scheme, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), uri)
		require.NoError(t, err)

		it, err := walkdir.ListFiles(context.Background(), scheme, dir)
		assertIteratorEqual(t, []string{f1.Name(), f2.Name()}, it)
		require.NoError(t, scheme.Close())
	})

	t.Run("returns before root Stat completes", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		dir := t.TempDir()
		uri, err := workspaceapi.CurrentUserHostURI(dir)
		require.NoError(t, err)

		_, err = os.OpenFile(filepath.Join(dir, "a"), os.O_CREATE, 0666)
		require.NoError(t, err)

		scheme, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), uri)
		require.NoError(t, err)

		gate := make(chan struct{})
		release := sync.OnceFunc(func() { close(gate) })
		defer release()

		res := listAsync(t, walkdir.ListFiles,
			gatedStatReader{Reader: scheme, gate: gate}, dir)
		require.NoError(t, res.err)

		release()
		assertIteratorEqual(t, []string{"a"}, res.it)

		require.NoError(t, scheme.Close())
	})

	t.Run("Close returns while root Stat is blocked", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		dir := t.TempDir()
		uri, err := workspaceapi.CurrentUserHostURI(dir)
		require.NoError(t, err)

		scheme, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), uri)
		require.NoError(t, err)

		gate := make(chan struct{})
		release := sync.OnceFunc(func() { close(gate) })
		defer release()

		res := listAsync(t, walkdir.ListFiles,
			gatedStatReader{Reader: scheme, gate: gate}, dir)
		require.NoError(t, res.err)

		closed := make(chan struct{})
		go func() {
			_ = res.it.Close()
			close(closed)
		}()
		select {
		case <-closed:
		case <-time.After(5 * time.Second):
			t.Fatal("Close blocked on root Stat")
		}
		require.ErrorIs(t, res.it.Err(), context.Canceled)

		release()
		require.NoError(t, scheme.Close())
	})

	t.Run("root Stat errors are reported via Err", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		uri, err := workspaceapi.CurrentUserHostURI(t.TempDir())
		require.NoError(t, err)

		scheme, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), uri)
		require.NoError(t, err)

		statErr := errors.New("stat boom")
		it, err := walkdir.ListFiles(context.Background(),
			errStatReader{Reader: scheme, err: statErr}, ".")
		require.NoError(t, err)

		_, ok := it.Next(context.Background())
		require.False(t, ok)
		require.ErrorContains(t, it.Err(), "stat boom")
		require.NoError(t, it.Close())

		require.NoError(t, scheme.Close())
	})
}

func TestFileSchemeListFilesLarge(t *testing.T) {
	defer goleak.VerifyNone(t)

	const n = 100

	workspaceURI, closeFn := setupTestDirectory(t, n, 10, 10)
	defer closeFn()

	scheme, err := workspace.NewFileScheme(context.Background(),
		config.NopConfig(), workspaceURI)
	require.NoError(t, err)

	it, err := walkdir.ListFiles(context.Background(), scheme, "")
	require.NoError(t, err)

	for range n {
		path, ok := it.Next(context.Background())
		require.True(t, ok)
		require.NoError(t, it.Err())
		assert.NotZero(t, path)
		assert.False(t, filepath.IsAbs(path))
	}

	path, ok := it.Next(context.Background())
	require.False(t, ok)
	assert.NoError(t, it.Err())
	assert.Zero(t, path)
	require.NoError(t, scheme.Close())
}

func BenchmarkListFilesTinyDir(b *testing.B) {
	benchListFiles(b, 5, 5, 5)
}

func BenchmarkListFilesSmallDirShallow(b *testing.B) {
	benchListFiles(b, 50, 5, 1)
}

func BenchmarkListFilesSmallDirDeep(b *testing.B) {
	benchListFiles(b, 50, 1, 1)
}

func BenchmarkListFilesLargeDirDeep(b *testing.B) {
	benchListFiles(b, 500, 10, 1)
}

func BenchmarkListFilesLargeDirShallow(b *testing.B) {
	benchListFiles(b, 500, 100, 1)
}

func BenchmarkListFilesHugeDirDeep(b *testing.B) {
	benchListFiles(b, 5000, 100, 1)
}

func BenchmarkListFilesHugeDirShallow(b *testing.B) {
	benchListFiles(b, 5000, 1000, 1)
}

func BenchmarkListFilesUberDir(b *testing.B) {
	benchListFiles(b, 500000, 10000, 1)
}

func BenchmarkListFilesLotsEmptyDir(b *testing.B) {
	benchListFiles(b, 500, 100, 100)
}

func benchListFiles(b *testing.B, totalFiles, nestEvery, emptyDirsPerFile int) {
	workspaceURI, closeFn := setupTestDirectory(b, totalFiles, nestEvery, emptyDirsPerFile)

	scheme, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), workspaceURI)
	if err != nil {
		b.Fatalf("error: %s", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		it, _ := walkdir.ListFiles(context.Background(), scheme, "")

		// consume iterator
		ok := true
		for ok {
			_, ok = it.Next(context.Background())
		}
	}
	b.StopTimer()
	closeFn()
	_ = scheme.Close()
}

func setupTestDirectory(
	t testing.TB, totalFiles, nestEvery, emptyDirsPerFile int,
) (workspaceapi.URI, func()) {
	dir, err := os.MkdirTemp("", "list_files_test")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})

	workspaceURI, err := workspaceapi.ParseURI("file://" + dir)
	require.NoError(t, err)

	var closeFns []func()
	// write test files
	for i := range totalFiles {
		f, err := os.CreateTemp(dir, strconv.Itoa(i))
		require.NoError(t, err)

		_, err = f.WriteString(strconv.Itoa(i))
		require.NoError(t, err)

		err = f.Close()
		require.NoError(t, err)

		for range emptyDirsPerFile {
			// create more dirs than workers
			d, err := os.MkdirTemp(dir, "emptydir")
			require.NoError(t, err)
			t.Cleanup(func() {
				_ = os.RemoveAll(d)
			})
		}
		if i%nestEvery == 0 {
			// nest next temp file created
			dir, err = os.MkdirTemp(dir, "nested")
			require.NoError(t, err)
			t.Cleanup(func() {
				_ = os.RemoveAll(dir)
			})
		}
		closeFns = append(closeFns, func() { os.Remove(f.Name()) })
	}
	return workspaceURI, func() {
		for _, closeFn := range closeFns {
			closeFn()
		}
	}
}
