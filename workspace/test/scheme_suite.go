package test

import (
	context "context"
	io "io"
	"io/fs"
	"io/ioutil"
	os "os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	workspace "unstable.build/go-tui/workspace"
)

func TestWorkspaceSchemeFiles(
	t *testing.T,
	schemeFn func(t *testing.T) workspace.Scheme,
) {
	t.Run("Open", func(t *testing.T) {
		testWorkspaceSchemeOpen(t, schemeFn)
	})
	t.Run("Remove", func(t *testing.T) {
		testWorkspaceSchemeRemove(t, schemeFn)
	})
	t.Run("Rename", func(t *testing.T) {
		testWorkspaceSchemeRename(t, schemeFn)
	})
	t.Run("Stat", func(t *testing.T) {
		testWorkspaceSchemeStat(t, schemeFn)
	})
	t.Run("Lstat", func(t *testing.T) {
		testWorkspaceSchemeLstat(t, schemeFn)
	})
	t.Run("Link", func(t *testing.T) {
		testWorkspaceSchemeReadLink(t, schemeFn)
	})
	t.Run("ListFiles", func(t *testing.T) {
		testWorkspaceSchemeListFiles(t, schemeFn)
	})
}

func createTestFile(t *testing.T, s workspace.Scheme, filename, content string) (workspace.File, func()) {
	file, werr := s.Open(filename, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0644)
	require.Nil(t, werr)
	_, err := file.Write([]byte(content))
	require.NoError(t, err)
	return file, func() {
		require.NoError(t, file.Close())
	}
}

func testWorkspaceSchemeOpen(
	t *testing.T,
	schemeFn func(t *testing.T) workspace.Scheme,
) {
	t.Run("returns error if O_CREATE flag is not passed and file doesn't exist", func(t *testing.T) {
		scheme := schemeFn(t)
		_, err := scheme.Open("file", 0, 0644)
		require.NotNil(t, err)
		assert.True(t, err.IsNotExist)
	})

	t.Run("relative to cwd or absolute to cwd should be the same file", func(t *testing.T) {
		scheme := schemeFn(t)
		_, cleanup := createTestFile(t, scheme, "file", "1234")
		defer cleanup()

		cwd, err := scheme.URI(".")
		require.NoError(t, err)

		d, werr := scheme.Open("file", os.O_RDONLY, 0)
		require.Nil(t, werr)

		data, err := ioutil.ReadAll(d)
		require.NoError(t, err)
		assert.Equal(t, "1234", string(data))

		d, werr = scheme.Open(filepath.Join(cwd.Path(), "file"), os.O_RDONLY, 0)
		require.Nil(t, werr)

		data, err = ioutil.ReadAll(d)
		require.NoError(t, err)
		assert.Equal(t, "1234", string(data))
	})

	t.Run("if a relative path is passed then that should be relative to the workspace cwd", func(t *testing.T) {
		scheme := schemeFn(t)
		f, werr := scheme.Open("file", os.O_CREATE, 0644)
		require.Nil(t, werr)

		cwdURI, err := scheme.URI(".")
		require.NoError(t, err)

		assert.Equal(t, filepath.Join(cwdURI.Path(), "file"), f.Name())
	})

	t.Run("if an absolute path is passed then it should access even outside of cwd", func(t *testing.T) {
		scheme := schemeFn(t)
		f, err := scheme.Open("/tmp/file", os.O_CREATE, 0644)
		require.Nil(t, err)
		assert.Equal(t, "/tmp/file", f.Name())
	})

	t.Run("returns error if O_EXCL|O_CREATE flag is passed and file exist", func(t *testing.T) {
		scheme := schemeFn(t)
		_, cleanup := createTestFile(t, scheme, "file", "")
		defer cleanup()
		_, err := scheme.Open("file", os.O_EXCL|os.O_CREATE, 0644)
		require.NotNil(t, err)
		assert.True(t, err.IsExist)
	})

	t.Run("returns a working File if Open succeeds", func(t *testing.T) {
		scheme := schemeFn(t)
		f, err := scheme.Open("file", os.O_CREATE|os.O_RDWR, 0644)
		require.Nil(t, err)

		t.Run("Name returns the file name", func(t *testing.T) {
			// implementations may or may not return the full path name
			// in the case of a file scheme, absolute is returned because
			// a workspace.Scheme is localized to the current working directory
			// so if the path passed to Open is relative, then we need to
			// prepend the cwd.
			assert.Contains(t, f.Name(), "file")
		})

		t.Run("Read before write", func(t *testing.T) {
			var buf [10]byte
			n, err := f.Read(buf[:])
			require.Equal(t, io.EOF, err)
			assert.Equal(t, 0, n)
		})

		t.Run("Write", func(t *testing.T) {
			n, err := f.Write([]byte("1234567890"))
			require.NoError(t, err)
			assert.Equal(t, 10, n)
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
			assert.Equal(t, int64(10), finfo.Size())
			assert.Equal(t, fs.FileMode(0644), finfo.Mode())
			assert.WithinDuration(t, finfo.ModTime(), time.Now(), 1*time.Minute)
			assert.Equal(t, false, finfo.IsDir())
		})

		t.Run("Seek", func(t *testing.T) {
			_, err := f.Seek(0, 0)
			require.NoError(t, err)
		})

		t.Run("Read", func(t *testing.T) {
			var buf [10]byte
			n, err := f.Read(buf[:])
			require.NoError(t, err)
			assert.Equal(t, 10, n)

			n, err = f.Read(buf[:])
			require.Equal(t, io.EOF, err)
			assert.Equal(t, 0, n)
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
	})
}

func testWorkspaceSchemeRemove(
	t *testing.T,
	schemeFn func(t *testing.T) workspace.Scheme,
) {
	t.Run("removes file", func(t *testing.T) {
		scheme := schemeFn(t)
		_, cleanup := createTestFile(t, scheme, "file", "")
		defer cleanup()
		require.NoError(t, scheme.Remove("file"))
	})
	t.Run("returns error if file has already been removed", func(t *testing.T) {
		scheme := schemeFn(t)
		_, cleanup := createTestFile(t, scheme, "file", "")
		defer cleanup()
		require.NoError(t, scheme.Remove("file"))
		require.Error(t, scheme.Remove("file"))
	})
	t.Run("returns error if file never existed", func(t *testing.T) {
		scheme := schemeFn(t)
		require.Error(t, scheme.Remove("file"))
	})
}

func testWorkspaceSchemeRename(
	t *testing.T,
	schemeFn func(t *testing.T) workspace.Scheme,
) {
	t.Run("renames a file if target name doesn't exist", func(t *testing.T) {
		scheme := schemeFn(t)
		_, cleanup := createTestFile(t, scheme, "file", "bla")
		defer cleanup()

		require.NoError(t, scheme.Rename("file", "foile"))
		f, werr := scheme.Open("foile", 0, 0)
		require.Nil(t, werr)

		data, err := ioutil.ReadAll(f)
		require.NoError(t, err)
		assert.Equal(t, "bla", string(data))

		_, werr = scheme.Open("file", 0, 0)
		require.NotNil(t, werr)
		assert.True(t, werr.IsNotExist)
	})

	t.Run("renames a file, overriding the target when it exists", func(t *testing.T) {
		scheme := schemeFn(t)
		_, cleanup := createTestFile(t, scheme, "file", "bla")
		defer cleanup()
		_, cleanup2 := createTestFile(t, scheme, "foile", "blo")
		defer cleanup2()

		require.NoError(t, scheme.Rename("file", "foile"))
		f, werr := scheme.Open("foile", 0, 0)
		require.Nil(t, werr)

		data, err := ioutil.ReadAll(f)
		require.NoError(t, err)
		assert.Equal(t, "bla", string(data))

		_, werr = scheme.Open("file", 0, 0)
		require.NotNil(t, werr)
		assert.True(t, werr.IsNotExist)
	})

	t.Run("returns error if original file does not exist", func(t *testing.T) {
		scheme := schemeFn(t)
		require.Error(t, scheme.Rename("file", "foile"))
	})
}

func testWorkspaceSchemeStats(
	t *testing.T,
	schemeFn func(t *testing.T) workspace.Scheme,
	method func(workspace.Scheme, string) (os.FileInfo, error),
) {
	t.Run("returns a valid os.FileInfo of a regular file", func(t *testing.T) {
		scheme := schemeFn(t)
		_, cleanup := createTestFile(t, scheme, "file", "bla")
		defer cleanup()

		finfo, err := method(scheme, "file")
		require.NoError(t, err)

		// always needs to be base name of the file
		require.Equal(t, "file", finfo.Name())
		assert.Equal(t, int64(3), finfo.Size())
		assert.Equal(t, fs.FileMode(0644), finfo.Mode())
		assert.WithinDuration(t, finfo.ModTime(), time.Now(), 1*time.Minute)
		assert.False(t, finfo.IsDir())
	})

	t.Run("returns error if file is not found", func(t *testing.T) {
		scheme := schemeFn(t)
		_, err := method(scheme, "file")
		require.Error(t, err)
	})
}

func testWorkspaceSchemeStat(
	t *testing.T,
	schemeFn func(t *testing.T) workspace.Scheme,
) {
	testWorkspaceSchemeStats(t, schemeFn, (workspace.Scheme).Stat)
}

func testWorkspaceSchemeLstat(
	t *testing.T,
	schemeFn func(t *testing.T) workspace.Scheme,
) {
	// clients do not use or need symlinks atm, so implementations
	// that do not support creating symlinks should not care about this.
	testWorkspaceSchemeStats(t, schemeFn, (workspace.Scheme).Lstat)
}

func testWorkspaceSchemeReadLink(
	t *testing.T,
	schemeFn func(t *testing.T) workspace.Scheme,
) {
	t.Run("should return error if underlying file is not a symlink", func(t *testing.T) {
		scheme := schemeFn(t)

		_, cleanup := createTestFile(t, scheme, "file", "")
		defer cleanup()

		_, err := scheme.ReadLink("file")
		require.Error(t, err)
	})
}

func testWorkspaceSchemeListFiles(
	t *testing.T,
	schemeFn func(t *testing.T) workspace.Scheme,
) {
	t.Run("returns an empty iterator if there are no files in the workspace", func(t *testing.T) {
		scheme := schemeFn(t)
		it, err := scheme.ListFiles(context.Background())
		require.NoError(t, err)
		_, ok := it.Next()
		require.False(t, ok)
	})

	t.Run("returns empty iterator with the files in the", func(t *testing.T) {
		scheme := schemeFn(t)
		totalFiles := 1000

		cwd, err := scheme.URI(".")
		require.NoError(t, err)

		for i := 0; i < totalFiles; i++ {
			_, cleanup := createTestFile(t, scheme, "file"+strconv.Itoa(i), strconv.Itoa(i))
			cleanup()
		}

		it, err := scheme.ListFiles(context.Background())
		require.NoError(t, err)

		for i := 0; i < 1000; i++ {
			path, ok := it.Next()
			require.True(t, ok)
			require.NoError(t, it.Err())
			assert.NotZero(t, path)
			assert.False(t, filepath.IsAbs(path))
			assert.Equal(t, cwd.Path(), filepath.Dir(filepath.Join(cwd.Path(), path)))
		}

		path, ok := it.Next()
		require.False(t, ok)
		assert.NoError(t, it.Err())
		assert.Zero(t, path)
	})
}
