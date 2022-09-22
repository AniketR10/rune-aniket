package workspace

import (
	"fmt"
	"os"
	"os/user"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	err := ret.init(uri)
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
