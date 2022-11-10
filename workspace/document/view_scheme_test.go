package document

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/workspace"
)

const dataFixture = "1234"

type testStruct struct {
	MyID string
	Data string
}

func (t testStruct) ID() string {
	return t.MyID
}

func schemeFn(t *testing.T) *readScheme[testStruct] {
	uri, err := workspace.ParseURI("fine:///")
	require.NoError(t, err)
	svc := document.NewInMemoryService()
	ret := new(readScheme[testStruct])
	ret.init(svc, uri, json.Marshaler())
	return ret
}

func createTestFile(t *testing.T, scheme *readScheme[testStruct], filename string) {
	var temp testStruct
	temp.MyID = filename
	temp.Data = dataFixture
	require.NoError(t, scheme.svc.Create(context.Background(), filename, temp))
}

func TestWorkspaceSchemeFiles(
	t *testing.T,
) {
	t.Run("Open", func(t *testing.T) {
		testWorkspaceSchemeViewOpen(t)
	})
	t.Run("Stat", func(t *testing.T) {
		testWorkspaceSchemeViewStat(t)
	})
	t.Run("ListFiles", func(t *testing.T) {
		testWorkspaceSchemeViewListFiles(t)
	})
}

func testWorkspaceSchemeViewStat(t *testing.T) {
	t.Run("returns a valid os.FileInfo of a regular file", func(t *testing.T) {
		scheme := schemeFn(t)
		createTestFile(t, scheme, "file")

		finfo, err := scheme.Stat("file")
		require.NoError(t, err)

		require.Equal(t, "file", finfo.Name())
		assert.WithinDuration(t, finfo.ModTime(), time.Now(), 1*time.Minute)
	})

	t.Run("returns error if file is not found", func(t *testing.T) {
		scheme := schemeFn(t)
		_, err := scheme.Stat("file")
		require.Error(t, err)
	})
}

func testWorkspaceSchemeViewListFiles(
	t *testing.T,
) {
	t.Run("returns an empty iterator if there are no files in the workspace", func(t *testing.T) {
		scheme := schemeFn(t)
		it, err := scheme.ListFiles(context.Background())
		require.NoError(t, err)
		_, ok := it.Next()
		require.False(t, ok)
	})

	t.Run("returns valid iterator", func(t *testing.T) {
		scheme := schemeFn(t)
		totalFiles := 10

		cwd, err := scheme.URI(".")
		require.NoError(t, err)

		for i := 0; i < totalFiles; i++ {
			createTestFile(t, scheme, "file"+strconv.Itoa(i))
		}

		it, err := scheme.ListFiles(context.Background())
		require.NoError(t, err)

		for i := 0; i < 10; i++ {
			path, ok := it.Next()
			require.True(t, ok)
			require.NoError(t, it.Err())
			assert.NotZero(t, path)
			assert.False(t, filepath.IsAbs(path))
			assert.Equal(t, cwd.Path(), filepath.Dir(filepath.Join(cwd.Path(), path)))
			_, werr := scheme.Open(path, 0, 0)
			assert.Nil(t, werr)
		}

		path, ok := it.Next()
		require.False(t, ok)
		assert.NoError(t, it.Err())
		assert.Zero(t, path)
	})
}

func testWorkspaceSchemeViewOpen(t *testing.T) {
	t.Run("returns error if O_CREATE flag is not passed and file doesn't exist", func(t *testing.T) {
		scheme := schemeFn(t)
		_, err := scheme.Open("file", 0, 0)
		require.NotNil(t, err)
		assert.True(t, err.IsNotExist)
	})
	t.Run("returns a read-only File if Open succeeds", func(t *testing.T) {
		scheme := schemeFn(t)
		createTestFile(t, scheme, "file")
		f, werr := scheme.Open("file", os.O_RDONLY, 0)
		require.Nil(t, werr)

		expectedData := fmt.Sprintf("{\"MyID\":\"file\",\"Data\":\"%s\"}", dataFixture)

		t.Run("Name returns the file name", func(t *testing.T) {
			// implementations may or may not return the full path name
			// in the case of a file scheme, absolute is returned because
			// a workspace.Scheme is localized to the current working directory
			// so if the path passed to Open is relative, then we need to
			// prepend the cwd.
			assert.Contains(t, f.Name(), "file")
		})

		t.Run("Read before seek succeeds", func(t *testing.T) {
			data, err := ioutil.ReadAll(f)
			require.NoError(t, err)
			assert.Equal(t, expectedData, string(data))
		})

		t.Run("Write errors", func(t *testing.T) {
			_, err := f.Write([]byte("1234567890"))
			require.Error(t, err)
		})

		t.Run("Sync errors", func(t *testing.T) {
			err := f.Sync()
			require.Error(t, err)
		})

		t.Run("Stat succeeds", func(t *testing.T) {
			finfo, err := f.Stat()
			require.NoError(t, err)
			assert.Equal(t, "file", finfo.Name())
			assert.WithinDuration(t, finfo.ModTime(), time.Now(), 1*time.Minute)
			assert.Equal(t, false, finfo.IsDir())
		})

		t.Run("Seek succeeds", func(t *testing.T) {
			_, err := f.Seek(0, 0)
			require.NoError(t, err)
		})

		t.Run("Read after Seek succeeds", func(t *testing.T) {
			data, err := ioutil.ReadAll(f)
			require.NoError(t, err)
			assert.Equal(t, expectedData, string(data))
		})

		t.Run("Truncate errors", func(t *testing.T) {
			err := f.Truncate(0)
			require.Error(t, err)
		})

		t.Run("Close succeeds", func(t *testing.T) {
			require.NoError(t, f.Close())
		})
	})
}
