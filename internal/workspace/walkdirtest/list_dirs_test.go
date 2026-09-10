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

package walkdir

import (
	"context"
	"strconv"
	"sync"

	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"go.uber.org/goleak"
	"unstable.build/rune/internal/workspace"
	"unstable.build/rune/internal/workspace/walkdir"
)

func TestListDirs(t *testing.T) {

	t.Run("lists all dirs under workspace as relative", func(t *testing.T) {
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

				err = os.MkdirAll(filepath.Join(dir, "a"), 0666)
				require.NoError(t, err)

				err = os.MkdirAll(filepath.Join(dir, "b"), 0666)
				require.NoError(t, err)

				_, err = os.OpenFile(filepath.Join(dir, "c"), os.O_CREATE, 0666)
				require.NoError(t, err)

				scheme, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), uri)
				require.NoError(t, err)

				it, err := walkdir.ListDirs(context.Background(), scheme, path)
				assertIteratorEqual(t, []string{"a", "b"}, it)

				require.NoError(t, scheme.Close())
			})
		}
	})
	t.Run("lists nested dirs", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		dir, err := os.MkdirTemp(".", "")
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = os.RemoveAll(dir)
		})
		uri, err := workspaceapi.CurrentUserHostURI(dir)
		require.NoError(t, err)

		err = os.Mkdir(filepath.Join(dir, "a"), 0777)
		require.NoError(t, err)

		err = os.Mkdir(filepath.Join(dir, "a", "b"), 0777)
		require.NoError(t, err)

		err = os.Mkdir(filepath.Join(dir, "a", "b", "c"), 0777)
		require.NoError(t, err)

		scheme, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), uri)
		require.NoError(t, err)

		it, err := walkdir.ListDirs(context.Background(), scheme, dir)
		assertIteratorEqual(t, []string{"a/b/c", "a", "a/b"}, it)

		require.NoError(t, scheme.Close())
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
			err = os.MkdirAll(filepath.Join(subdir, "a"), 0666)
			require.NoError(t, err)
		}

		scheme, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), uri)
		require.NoError(t, err)

		it, err := walkdir.ListDirs(context.Background(), scheme, dir)
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

		dir1 := filepath.Join(dir, "a")
		err = os.MkdirAll(dir1, 0666)
		require.NoError(t, err)

		dir2 := filepath.Join(dir, "b")
		err = os.MkdirAll(dir2, 0666)
		require.NoError(t, err)

		scheme, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), uri)
		require.NoError(t, err)

		it, err := walkdir.ListDirs(context.Background(), scheme, dir)
		assertIteratorEqual(t, []string{dir1, dir2}, it)
		require.NoError(t, scheme.Close())
	})

	t.Run("returns before root Stat completes", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		dir := t.TempDir()
		uri, err := workspaceapi.CurrentUserHostURI(dir)
		require.NoError(t, err)

		require.NoError(t, os.MkdirAll(filepath.Join(dir, "a"), 0777))

		scheme, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), uri)
		require.NoError(t, err)

		gate := make(chan struct{})
		release := sync.OnceFunc(func() { close(gate) })
		defer release()

		res := listAsync(t, walkdir.ListDirs,
			gatedStatReader{Reader: scheme, gate: gate}, dir)
		require.NoError(t, res.err)

		release()
		assertIteratorEqual(t, []string{"a"}, res.it)

		require.NoError(t, scheme.Close())
	})
}
