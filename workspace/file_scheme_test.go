package workspace

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/api/config"
	workspaceapi "unstable.build/go-tui/api/workspace"
)

func newTestFileScheme(uri workspaceapi.URI) (*fileScheme, error) {
	ret := new(fileScheme)
	ret.osStat = func(path string) (os.FileInfo, error) {
		if strings.Contains(path, ".txt") || strings.Contains(path, ".md") {
			return testFileInfo{name: path, isDir: false}, nil
		}
		return testFileInfo{name: path, isDir: true}, nil
	}
	ret.getUser = func() (*user.User, error) {
		return &user.User{Username: "git", HomeDir: "/home/git"}, nil
	}
	ret.lookupUser = func(username string) (*user.User, error) {
		return &user.User{Username: username, HomeDir: fmt.Sprintf("/home/%s", username)}, nil
	}
	err := ret.init(config.NopConfig(), uri)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func TestNewScheme(t *testing.T) {
	tsuite := []struct {
		desc         string
		workspaceURI string
		expectedErr  string
	}{
		{"no port no user workspace absolute returns error", "file://ernest.photography", "invalid file URI"},
		{"port no user workspace absolute returns error", "file://ernest.photography:4222", "invalid file URI"},
		{"port user workspace absolute returns error", "file://ernie@ernest.photography:4222", "invalid file URI"},
		{"port user workspace absolute slash returns error", "file://ernie@ernest.photography:4222/", "invalid file URI"},
		{"no port user workspace absolute slash returns error", "file://ernie@ernest.photography/", "invalid file URI"},
		{"different scheme returns error", "ssh:///tmp", "invalid file URI"},
		{"no host regular folder success", "file:///tmp", ""},
		{"root", "file:///", ""},
		{"file uri should return error", "file:///tmp/file.txt",
			"workspaceapi.URI does not refer to a directory: file:///tmp/file.txt"}, // newTestFileScheme sets osStat based on file name
	}

	for _, tcase := range tsuite {
		t.Run(tcase.desc, func(t *testing.T) {
			workspaceURI, err := workspaceapi.ParseURI(tcase.workspaceURI)
			require.NoError(t, err)

			// sut
			_, err = newTestFileScheme(workspaceURI)
			if tcase.expectedErr != "" {
				assert.EqualError(t, err, tcase.expectedErr)
			} else {
				assert.NoError(t, err)
			}

		})
	}
}

func TestFileSchemeURI(t *testing.T) {
	tsuite := []struct {
		desc         string
		workspaceURI string
		inPath       string
		expectedOut  string
		expectedErr  string
	}{
		{"absolute root", "file:///", "/",
			"file:///", ""},
		{"absolute root, non-root workspace", "file:///home/ernicles", "/",
			"file:///", ""},
		{"absolute root 2", "file:///var", "/tmp/file",
			"file:///tmp/file", ""},
		{"absolute folder is equal to workspace", "file:///home/ernicles", "/home/ernicles",
			"file:///home/ernicles", ""},
		{"absolute folder file", "file:///home/ernicles", "/home/ernicles/file.txt",
			"file:///home/ernicles/file.txt", ""},
		{"absolute other folder file", "file:///home/ernicles", "/home/git/file.txt",
			"file:///home/git/file.txt", ""},
		{"relative file", "file:///home/ernicles", "file.txt",
			"file:///home/ernicles/file.txt", ""},
		{"relative file to home, non workspace", "file:///home/src/blue", "~/file.txt",
			"file:///home/git/file.txt", ""},
		{"relative file to home, workspace", "file:///home/git/", "~/file.txt",
			"file:///home/git/file.txt", ""},
		{"relative upwards workspace tree", "file:///home/git/src/blue", "../../",
			"file:///home/git/src/blue/../../", ""},
		{"relative upwards workspace tree ending slash", "file:///home/git/src/blue/", "../../",
			"file:///home/git/src/blue/../../", ""},
		{"relative upwards workspace tree ./ path", "file:///home/git/src/blue", "./../../file.txt",
			"file:///home/git/src/blue/./../../file.txt", ""},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.desc, func(t *testing.T) {
			workspaceURI, err := workspaceapi.ParseURI(tcase.workspaceURI)
			require.NoError(t, err)

			s, err := newTestFileScheme(workspaceURI)
			require.NoError(t, err)
			defer s.Close()

			// sut
			actualOut, actualErr := s.URI(tcase.inPath)
			if tcase.expectedErr != "" {
				assert.Error(t, actualErr)
				assert.Nil(t, actualOut)
			} else {
				assert.NoError(t, actualErr)

				expectedURI, err := workspaceapi.ParseURI(tcase.expectedOut)
				require.NoError(t, err)
				assert.Equal(t, expectedURI.String(), actualOut.String())
			}

		})
	}
}

func setupTestDirectory(
	t testing.TB, totalFiles, nestEvery, emptyDirsPerFile int,
) (workspaceapi.URI, func()) {
	dir, err := os.MkdirTemp("", "list_files_test")
	require.NoError(t, err)

	workspaceURI, err := workspaceapi.ParseURI("file://" + dir)
	require.NoError(t, err)

	var closeFns []func()
	// write test files
	for i := 0; i < totalFiles; i++ {
		f, err := os.CreateTemp(dir, strconv.Itoa(i))
		require.NoError(t, err)

		_, err = f.WriteString(strconv.Itoa(i))
		require.NoError(t, err)

		err = f.Close()
		require.NoError(t, err)

		for i := 0; i < emptyDirsPerFile; i++ {
			// create more dirs than workers
			_, err = os.MkdirTemp(dir, "emptydir")
			require.NoError(t, err)
		}
		if i%nestEvery == 0 {
			// nest next temp file created
			dir, err = os.MkdirTemp(dir, "nested")
			require.NoError(t, err)
		}
		closeFns = append(closeFns, func() { os.Remove(f.Name()) })
	}
	return workspaceURI, func() {
		for _, closeFn := range closeFns {
			closeFn()
		}
	}
}

func TestFileSchemeListFilesLarge(t *testing.T) {
	const n = 100

	workspaceURI, closeFn := setupTestDirectory(t, n, 10, 10)
	defer closeFn()

	scheme, err := NewFileScheme(context.Background(),
		config.NopConfig(), workspaceURI)
	require.NoError(t, err)

	it, err := ListFiles(context.Background(), scheme, "")
	require.NoError(t, err)

	for i := 0; i < n; i++ {
		path, ok := it.Next()
		require.True(t, ok)
		require.NoError(t, it.Err())
		assert.NotZero(t, path)
		assert.False(t, filepath.IsAbs(path))
	}

	path, ok := it.Next()
	require.False(t, ok)
	assert.NoError(t, it.Err())
	assert.Zero(t, path)
}

func benchListFiles(b *testing.B, totalFiles, nestEvery, emptyDirsPerFile int) {
	workspaceURI, closeFn := setupTestDirectory(b, totalFiles, nestEvery, emptyDirsPerFile)

	scheme, err := NewFileScheme(context.Background(), config.NopConfig(), workspaceURI)
	if err != nil {
		b.Fatalf("error: %s", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		it, _ := ListFiles(context.Background(), scheme, "")

		// consume iterator
		ok := true
		for ok {
			_, ok = it.Next()
		}
	}
	b.StopTimer()
	closeFn()
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

func TestFileAssumptions(t *testing.T) {
	t.Run("Write overwrites data", func(t *testing.T) {
		f, err := os.CreateTemp("", "")
		name := f.Name()
		n, err := f.Write([]byte("12345"))
		require.NoError(t, err)
		require.NoError(t, f.Close())

		f, err = os.OpenFile(name, os.O_RDWR, 0)
		require.NoError(t, err)
		n, err = f.Write([]byte("ZZ"))
		require.NoError(t, err)
		assert.Equal(t, 2, n)

		nn, err := f.Seek(0, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(0), nn)

		data, err := io.ReadAll(f)
		require.NoError(t, err)
		assert.Equal(t, "ZZ345", string(data))
	})

	t.Run("Read uses write offset", func(t *testing.T) {
		f, err := os.CreateTemp("", "")
		name := f.Name()
		n, err := f.Write([]byte("12345"))
		require.NoError(t, err)
		require.NoError(t, f.Close())

		f, err = os.OpenFile(name, os.O_RDWR, 0)
		require.NoError(t, err)

		n, err = f.Write([]byte("ZZ"))
		require.NoError(t, err)
		assert.Equal(t, 2, n)

		data, err := io.ReadAll(f)
		require.NoError(t, err)
		assert.Equal(t, "345", string(data))
	})
}

func TestStartCommand(t *testing.T) {
	t.Run("Cmd.Dir makes command run on that directory", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "")
		require.NoError(t, err)

		uri, err := workspaceapi.ParseURI("file://" + tmpDir)
		require.NoError(t, err)

		s, err := newTestFileScheme(uri)
		require.NoError(t, err)

		var stdout bytes.Buffer
		var stderr bytes.Buffer

		ch := make(chan error)
		ctx := context.Background()

		// If Dir is passed runs there
		cmd := workspaceapi.Cmd{
			Path:    "/bin/sh",
			Args:    []string{"-c", "pwd"},
			Dir:     "/bin",
			Watcher: workspaceapi.ChanWatcher(ch),
			Stdout:  &stdout,
			Stderr:  &stderr,
		}

		_, err = s.StartCommand(ctx, cmd)
		require.NoError(t, err)

		err = <-ch
		assert.Equal(t, err, nil)

		assert.Equal(t, stderr.String(), "")
		assert.Equal(t, stdout.String(), "/bin\n")

		stderr.Reset()
		stdout.Reset()

	})

	t.Run("omitting Cmd.Dir makes command run on workspace dir", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "")
		require.NoError(t, err)

		uri, err := workspaceapi.ParseURI("file://" + tmpDir)
		require.NoError(t, err)

		s, err := newTestFileScheme(uri)
		require.NoError(t, err)

		var stdout bytes.Buffer
		var stderr bytes.Buffer

		ch := make(chan error)
		ctx := context.Background()

		cmd := workspaceapi.Cmd{
			Path:    "/bin/sh",
			Args:    []string{"-c", "pwd"},
			Watcher: workspaceapi.ChanWatcher(ch),
			Stdout:  &stdout,
			Stderr:  &stderr,
		}

		_, err = s.StartCommand(ctx, cmd)
		require.NoError(t, err)

		err = <-ch
		assert.Equal(t, err, nil)

		assert.Equal(t, stderr.String(), "")
		assert.Equal(t, stdout.String(), tmpDir+"\n")
	})
}
