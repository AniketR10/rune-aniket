package workspace

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/ernestrc/blue/iterator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/api/config"
	workspaceapi "unstable.build/go-tui/api/workspace"
)

func assertIteratorEqual(
	t *testing.T, expected []string, it iterator.Iterator[string],
) {
	var actual []string
	for {
		next, ok := it.Next()
		if !ok {
			break
		}
		actual = append(actual, next)
	}
	require.NoError(t, it.Err())

	sort.Strings(actual)
	sort.Strings(expected)
	assert.Equal(t, actual, expected)
}

func TestListFiles(t *testing.T) {
	t.Run("lists all files under workspace as relative", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "")
		require.NoError(t, err)
		for _, path := range []string{".", dir} {
			t.Run(path, func(t *testing.T) {
				uri, err := workspaceapi.CurrentUserHostURI(dir)
				require.NoError(t, err)

				_, err = os.OpenFile(filepath.Join(dir, "a"), os.O_CREATE, 0666)
				require.NoError(t, err)

				_, err = os.OpenFile(filepath.Join(dir, "b"), os.O_CREATE, 0666)
				require.NoError(t, err)

				scheme, err := NewFileScheme(context.Background(), config.NopConfig(), uri)
				require.NoError(t, err)

				it, err := ListFiles(context.Background(), scheme, path)
				assertIteratorEqual(t, []string{"a", "b"}, it)
			})
		}
	})

	t.Run("lists all files under non-workspace dir as absolute", func(t *testing.T) {
		workspaceDir, err := ioutil.TempDir("", "")
		require.NoError(t, err)

		dir, err := ioutil.TempDir("", "")
		require.NoError(t, err)

		uri, err := workspaceapi.CurrentUserHostURI(workspaceDir)
		require.NoError(t, err)

		f1, err := os.OpenFile(filepath.Join(dir, "a"), os.O_CREATE, 0666)
		require.NoError(t, err)

		f2, err := os.OpenFile(filepath.Join(dir, "b"), os.O_CREATE, 0666)
		require.NoError(t, err)

		scheme, err := NewFileScheme(context.Background(), config.NopConfig(), uri)
		require.NoError(t, err)

		it, err := ListFiles(context.Background(), scheme, dir)
		assertIteratorEqual(t, []string{f1.Name(), f2.Name()}, it)
	})
}
