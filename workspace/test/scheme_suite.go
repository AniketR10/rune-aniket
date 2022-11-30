package test

import (
	"bytes"
	context "context"
	"errors"
	"fmt"
	io "io"
	"io/ioutil"
	os "os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cell "unstable.build/go-tui/cell"
	workspace "unstable.build/go-tui/workspace"
)

func TestWorkspaceSchemeFiles(
	t *testing.T,
	schemeFn func(t *testing.T) workspace.Scheme,
) {
	t.Run("Open", func(t *testing.T) {
		TestWorkspaceSchemeOpen(t, schemeFn, defaultCreateTestFile,
			ioutil.ReadAll, (workspace.File).Write, true)
	})
	t.Run("Remove", func(t *testing.T) {
		TestWorkspaceSchemeRemove(t, schemeFn, defaultCreateTestFile)
	})
	t.Run("Rename", func(t *testing.T) {
		TestWorkspaceSchemeRename(t, schemeFn, defaultCreateTestFile, ioutil.ReadAll)
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
	t.Run("workspace.ListFiles integration", func(t *testing.T) {
		TestWorkspaceSchemeListFilesIntegration(t, schemeFn, defaultCreateTestFile)
	})
	t.Run("workspace.Load integration", func(t *testing.T) {
		TestWorkspaceLoadIntegration(t, schemeFn, defaultCreateTestFile,
			readAllExceptLastEOL, (*cell.Buffer).Write)
	})
}

func readAllExceptLastEOL(r io.Reader) (data []byte, err error) {
	data, err = ioutil.ReadAll(r)
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
	schemeFn func(t *testing.T) workspace.Scheme,
	createTestFile func(*testing.T, workspace.Scheme, string, string) (workspace.File, func()),
	readAll func(io.Reader) ([]byte, error),
	write func(*cell.Buffer, []byte) (int, error),
) {
	t.Run("loads a NEW file into a buffer and flushes new data to it", func(t *testing.T) {
		scheme := schemeFn(t)
		uri, err := scheme.URI(".")
		require.NoError(t, err)
		wp := workspace.NewSchemeWorkspace(uri, scheme)
		fileuri := workspace.Join(uri, "myFile")
		swapDir := workspace.Join(uri, ".")
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
		_, cleanup := createTestFile(t, scheme, "myExistingFile", "VERY ")
		defer cleanup()

		uri, err := scheme.URI(".")
		require.NoError(t, err)
		wp := workspace.NewSchemeWorkspace(uri, scheme)
		fileuri := workspace.Join(uri, "myExistingFile")
		swapDir := workspace.Join(uri, ".")
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
		_, cleanup1 := createTestFile(t, scheme, "file", "")
		defer cleanup1()

		_, cleanup2 := createTestFile(t, scheme, ".file.swp", "mosca")
		defer cleanup2()

		uri, err := scheme.URI(".")
		require.NoError(t, err)
		wp := workspace.NewSchemeWorkspace(uri, scheme)
		fileuri := workspace.Join(uri, "file")
		swapuri := workspace.Join(uri, ".file.swp")
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
		_, cleanup2 := createTestFile(t, scheme, ".file.swp", "mosca")
		defer cleanup2()

		uri, err := scheme.URI(".")
		require.NoError(t, err)
		wp := workspace.NewSchemeWorkspace(uri, scheme)
		fileuri := workspace.Join(uri, "file")
		swapuri := workspace.Join(uri, ".file.swp")
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
		uri, err := scheme.URI(".")
		require.NoError(t, err)
		wp := workspace.NewSchemeWorkspace(uri, scheme)
		fileuri := workspace.Join(uri, "myCloseTest")
		swapDir := workspace.Join(uri, ".")

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

	t.Run("close should cleanup temp state such that next Load works", func(t *testing.T) {
		scheme := schemeFn(t)
		uri, err := scheme.URI(".")
		require.NoError(t, err)
		wp := workspace.NewSchemeWorkspace(uri, scheme)
		fileuri := workspace.Join(uri, "myCloseTest")
		swapDir := workspace.Join(uri, ".")

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

func defaultCreateTestFile(t *testing.T, s workspace.Scheme, filename, content string) (workspace.File, func()) {
	file, werr := s.Open(filename, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0644)
	require.Nil(t, werr, werr.String())
	_, err := file.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, file.Sync())
	_, err = file.Seek(0, 0)
	require.NoError(t, err)
	return file, func() {
		require.NoError(t, file.Close())
	}
}

func TestWorkspaceSchemeOpen(
	t *testing.T,
	schemeFn func(t *testing.T) workspace.Scheme,
	createTestFile func(*testing.T, workspace.Scheme, string, string) (workspace.File, func()),
	readAll func(io.Reader) ([]byte, error),
	writeFile func(workspace.File, []byte) (int, error),
	testRelativeAbsolutePaths bool,
) {
	t.Run("returns error if O_CREATE flag is not passed and file doesn't exist", func(t *testing.T) {
		scheme := schemeFn(t)
		_, err := scheme.Open("file", 0, 0644)
		require.NotNil(t, err)
		assert.True(t, err.IsNotExist)
	})

	t.Run("truncates file if O_TRUNC is passed if file stored has data", func(t *testing.T) {
		scheme := schemeFn(t)
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
			f, werr := scheme.Open("file", os.O_CREATE, 0644)
			require.Nil(t, werr, werr.String())

			cwdURI, err := scheme.URI(".")
			require.NoError(t, err)

			assert.Equal(t, filepath.Join(cwdURI.Path(), "file"), f.Name())
		})

		t.Run("if an absolute path is passed then it should access even outside of cwd", func(t *testing.T) {
			scheme := schemeFn(t)
			f, err := scheme.Open("/tmp/file", os.O_CREATE, 0644)
			require.Nil(t, err, err.String())
			assert.Equal(t, "/tmp/file", f.Name())
		})
	}

	t.Run("returns error if O_EXCL|O_CREATE flag is passed and file exist", func(t *testing.T) {
		scheme := schemeFn(t)
		_, cleanup := createTestFile(t, scheme, "file", "")
		defer cleanup()
		_, err := scheme.Open("file", os.O_EXCL|os.O_CREATE, 0644)
		require.NotNil(t, err, err.String())
		assert.True(t, err.IsExist)
	})

	t.Run("returns a working File if Open succeeds", func(t *testing.T) {
		scheme := schemeFn(t)
		f, err := scheme.Open("file", os.O_CREATE|os.O_RDWR, 0644)
		require.Nil(t, err, err.String())

		t.Run("Name returns the file name", func(t *testing.T) {
			// implementations may or may not return the full path name
			// in the case of a file scheme, absolute is returned because
			// a workspace.Scheme is localized to the current working directory
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
	})
}

func TestWorkspaceSchemeRemove(
	t *testing.T,
	schemeFn func(t *testing.T) workspace.Scheme,
	createTestFile func(*testing.T, workspace.Scheme, string, string) (workspace.File, func()),
) {
	t.Run("removes file", func(t *testing.T) {
		scheme := schemeFn(t)
		_, cleanup := createTestFile(t, scheme, "file", "")
		defer cleanup()
		require.NoError(t, scheme.Remove("file"))
	})
	t.Run("returns ErrNotExist if file has already been removed", func(t *testing.T) {
		scheme := schemeFn(t)
		_, cleanup := createTestFile(t, scheme, "file", "")
		defer cleanup()
		require.NoError(t, scheme.Remove("file"))
		err := scheme.Remove("file")
		require.Error(t, err)
		assert.True(t, errors.Is(err, os.ErrNotExist))
	})
	t.Run("returns ErrNotExist if file never existed", func(t *testing.T) {
		scheme := schemeFn(t)
		err := scheme.Remove("file")
		require.Error(t, err)
		assert.True(t, errors.Is(err, os.ErrNotExist))
	})
}

func TestWorkspaceSchemeRename(
	t *testing.T,
	schemeFn func(t *testing.T) workspace.Scheme,
	createTestFile func(*testing.T, workspace.Scheme, string, string) (workspace.File, func()),
	readAll func(io.Reader) ([]byte, error),
) {
	t.Run("renames a file if target name doesn't exist", func(t *testing.T) {
		scheme := schemeFn(t)
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
		err := scheme.Rename("file", "foile")
		require.Error(t, err)
		assert.True(t, errors.Is(err, os.ErrNotExist))
	})
}

func testWorkspaceSchemeStats(
	t *testing.T,
	schemeFn func(t *testing.T) workspace.Scheme,
	method func(workspace.Scheme, string) (os.FileInfo, error),
	createTestFile func(*testing.T, workspace.Scheme, string, string) (workspace.File, func()),
) {
	t.Run("returns a valid os.FileInfo of a regular file", func(t *testing.T) {
		scheme := schemeFn(t)
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
		_, err := method(scheme, "file")
		require.Error(t, err)
		assert.True(t, errors.Is(err, os.ErrNotExist))
	})
}

func TestWorkspaceSchemeStat(
	t *testing.T,
	schemeFn func(t *testing.T) workspace.Scheme,
	createTestFile func(*testing.T, workspace.Scheme, string, string) (workspace.File, func()),
) {
	testWorkspaceSchemeStats(t, schemeFn, (workspace.Scheme).Stat, createTestFile)
}

func TestWorkspaceSchemeLstat(
	t *testing.T,
	schemeFn func(t *testing.T) workspace.Scheme,
	createTestFile func(*testing.T, workspace.Scheme, string, string) (workspace.File, func()),
) {
	// clients do not use or need symlinks atm, so implementations
	// that do not support creating symlinks should not care about this.
	testWorkspaceSchemeStats(t, schemeFn, (workspace.Scheme).Lstat, createTestFile)
}

func TestWorkspaceSchemeReadLink(
	t *testing.T,
	schemeFn func(t *testing.T) workspace.Scheme,
	createTestFile func(*testing.T, workspace.Scheme, string, string) (workspace.File, func()),
) {
	t.Run("should return error if underlying file is not a symlink", func(t *testing.T) {
		scheme := schemeFn(t)

		_, cleanup := createTestFile(t, scheme, "file", "")
		defer cleanup()

		_, err := scheme.ReadLink("file")
		require.Error(t, err)
	})
}

func TestWorkspaceSchemeListFilesIntegration(
	t *testing.T,
	schemeFn func(t *testing.T) workspace.Scheme,
	createTestFile func(*testing.T, workspace.Scheme, string, string) (workspace.File, func()),
) {
	t.Run("os.FileInfo.IsDir on filesystem root should always return true", func(t *testing.T) {
		scheme := schemeFn(t)
		finfo, err := scheme.Stat("/")
		require.NoError(t, err)
		assert.True(t, finfo.IsDir())
	})

	t.Run("os.FileInfo.IsDir on workspace root should always return true", func(t *testing.T) {
		scheme := schemeFn(t)
		finfo, err := scheme.Stat(".")
		require.NoError(t, err)
		assert.True(t, finfo.IsDir())
	})

	t.Run("returns an empty iterator if there are no files anywhere", func(t *testing.T) {
		scheme := schemeFn(t)
		it, err := workspace.ListFiles(context.Background(), scheme, "")
		require.NoError(t, err)
		_, ok := it.Next()
		require.False(t, ok)
	})

	for _, path := range []string{".", ""} {
		t.Run(fmt.Sprintf("returns valid iterator with path %s", path), func(t *testing.T) {
			scheme := schemeFn(t)
			totalFiles := 1000

			cwd, err := scheme.URI(".")
			require.NoError(t, err)

			for i := 0; i < totalFiles; i++ {
				_, cleanup := createTestFile(t, scheme, "file"+strconv.Itoa(i), strconv.Itoa(i))
				cleanup()
			}

			it, err := workspace.ListFiles(context.Background(), scheme, path)
			require.NoError(t, err)

			for i := 0; i < totalFiles; i++ {
				path, ok := it.Next()
				require.True(t, ok, i)
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

	t.Run("root does not exist returns no error and scans parent folder", func(t *testing.T) {
		scheme := schemeFn(t)
		totalFiles := 10

		cwd, err := scheme.URI(".")
		require.NoError(t, err)

		for i := 0; i < totalFiles; i++ {
			_, cleanup := createTestFile(t, scheme, "file"+strconv.Itoa(i), strconv.Itoa(i))
			cleanup()
		}

		it, err := workspace.ListFiles(context.Background(), scheme, "./subfolder")
		require.NoError(t, err)

		for i := 0; i < totalFiles; i++ {
			path, ok := it.Next()
			require.True(t, ok, i)
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

	t.Run("root is used up until base directory; full file name is ignored", func(t *testing.T) {
		scheme := schemeFn(t)
		totalFiles := 10

		cwd, err := scheme.URI(".")
		require.NoError(t, err)

		for i := 0; i < totalFiles; i++ {
			_, cleanup := createTestFile(t, scheme, "file"+strconv.Itoa(i), strconv.Itoa(i))
			cleanup()
		}

		it, err := workspace.ListFiles(context.Background(), scheme, "./file1")
		require.NoError(t, err)

		for i := 0; i < totalFiles; i++ {
			path, ok := it.Next()
			require.True(t, ok, i)
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

	t.Run("does not return error if root's base dir does not exist", func(t *testing.T) {
		scheme := schemeFn(t)
		totalFiles := 10

		cwd, err := scheme.URI(".")
		require.NoError(t, err)

		for i := 0; i < totalFiles; i++ {
			_, cleanup := createTestFile(t, scheme, "file"+strconv.Itoa(i), strconv.Itoa(i))
			cleanup()
		}

		it, err := workspace.ListFiles(context.Background(), scheme, "fi/fi/fi/fi")
		require.NoError(t, err)

		for i := 0; i < totalFiles; i++ {
			path, ok := it.Next()
			require.True(t, ok, i)
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

func TestWorkspaceSchemeReadDir(
	t *testing.T,
	schemeFn func(t *testing.T) workspace.Scheme,
	createTestFile func(*testing.T, workspace.Scheme, string, string) (workspace.File, func()),
) {
	t.Run("a file should return an error", func(t *testing.T) {
		scheme := schemeFn(t)

		_, cleanup := createTestFile(t, scheme, "file", "")
		defer cleanup()

		_, err := scheme.ReadDir("file")
		require.Error(t, err)
	})

	t.Run("workspace dir should return all files at the workspace root directory", func(t *testing.T) {
		scheme := schemeFn(t)

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
