package workspace

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/config"
)

func newTestFileScheme(uri URI) (*fileScheme, error) {
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
		{"file uri should use file base dir as workspace", "file:///tmp/file.txt", ""}, // newTestFileScheme sets osStat based on file name
	}

	for _, tcase := range tsuite {
		t.Run(tcase.desc, func(t *testing.T) {
			workspaceURI, err := ParseURI(tcase.workspaceURI)
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
		{"workspace URI is a file URI should use file dir as workspace base URI", "file:///tmp/file.txt", "file.md",
			"file:///tmp/file.md", ""},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.desc, func(t *testing.T) {
			workspaceURI, err := ParseURI(tcase.workspaceURI)
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

				expectedURI, err := ParseURI(tcase.expectedOut)
				require.NoError(t, err)
				assert.Equal(t, expectedURI.String(), actualOut.String())
			}

		})
	}
}

func setupTestDirectory(totalFiles, nestEvery, emptyDirsPerFile int) (URI, func(), error) {
	dir, err := ioutil.TempDir("", "list_files_test")
	if err != nil {
		return URI{}, nil, err
	}
	workspaceURI, err := ParseURI("file://" + dir)
	if err != nil {
		return URI{}, nil, err
	}

	var closeFns []func()
	// write test files
	for i := 0; i < totalFiles; i++ {
		f, err := ioutil.TempFile(dir, strconv.Itoa(i))
		if err != nil {
			return URI{}, nil, err
		}
		_, err = f.WriteString(strconv.Itoa(i))
		if err != nil {
			return URI{}, nil, err
		}
		err = f.Close()
		if err != nil {
			return URI{}, nil, err
		}
		for i := 0; i < emptyDirsPerFile; i++ {
			// create more dirs than workers
			_, err = ioutil.TempDir(dir, "emptydir")
			if err != nil {
				return URI{}, nil, err
			}
		}
		if i%nestEvery == 0 {
			// nest next temp file created
			dir, err = ioutil.TempDir(dir, "nested")
			if err != nil {
				return URI{}, nil, err
			}
		}
		closeFns = append(closeFns, func() { os.Remove(f.Name()) })
	}
	return workspaceURI, func() {
		for _, closeFn := range closeFns {
			closeFn()
		}
	}, nil
}

func TestListFiles(t *testing.T) {
	workspaceURI, closeFn, err := setupTestDirectory(1000, 10, 10)
	defer closeFn()

	scheme, err := NewFileScheme(config.NopConfig(), workspaceURI)
	require.NoError(t, err)

	it, err := scheme.ListFiles(context.Background())
	require.NoError(t, err)

	for i := 0; i < 1000; i++ {
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
	workspaceURI, closeFn, err := setupTestDirectory(totalFiles, nestEvery, emptyDirsPerFile)
	if err != nil {
		b.Fatalf("error: %s:", err)
	}

	scheme, err := NewFileScheme(config.NopConfig(), workspaceURI)
	if err != nil {
		b.Fatalf("error: %s", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		it, _ := scheme.ListFiles(context.Background())

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
