package workspace

import (
	"errors"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ernestrc/go-tui/cell"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

var discardLogger = log.New()

func init() {
	discardLogger.Out = ioutil.Discard
	discardLogger.Level = log.PanicLevel
}

func TestManagerInitLocal(t *testing.T) {
	tsuite := []struct {
		desc         string
		inWorkspace  string
		expectosStat string
		wantErr      bool
	}{
		{"returns an error if workspace is not a directory",
			"file:///tmp", "/tmp", true},
		{"returns no error if workspace is a directory",
			"file:///tmp", "/tmp/hello", false},
	}
	for _, tcase := range tsuite {
		t.Run(tcase.desc, func(t *testing.T) {
			m := new(Manager)
			m.osStat = func(name string) (os.FileInfo, error) {
				if tcase.wantErr {
					return nil, errors.New("oops")
				}
				return testFileInfo{isDir: true}, nil
			}
			uri, err := ParseURI(tcase.inWorkspace)
			require.NoError(t, err)
			err = m.init(discardLogger, uri)
			if tcase.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestManagerURI(t *testing.T) {
	tsuite := []struct {
		desc        string
		inWorkspace string
		getcwd      string
		getUser     *user.User
		inPath      string
		wantURI     string
	}{
		{"handles relative path with local workspace",
			"file:///tmp", "/tmp", &user.User{}, "relative_path/file.go", "file:///tmp/relative_path/file.go"},
		{"handles abs path with local workspace",
			"file:///tmp", "/tmp", &user.User{}, "/var/log/abs_path/file.go", "file:///var/log/abs_path/file.go"},
		{"handles abs path with remote workspace",
			"ssh://ernicles@my_host:8080/home/ipad", "/home/ipad", &user.User{},
			"/var/log/abs_path/file.go", "ssh://ernicles@my_host:8080/var/log/abs_path/file.go"},
		{"handles relative path with remote workspace",
			"ssh://ernicles@my_host:8080/home/ipad", "/home/ipad", &user.User{},
			"relative_path/file.go", "ssh://ernicles@my_host:8080/home/ipad/relative_path/file.go"},
		{"handles ~/ path", // has to be remote otherwise we use user.Current and we cannot mock the homedir
			"ssh://kombutcha@my_host/home/ipad", "/tmp", &user.User{HomeDir: "/home/kombutcha"}, "~/file.go", "ssh://kombutcha@my_host/home/kombutcha/file.go"},
	}
	for _, tcase := range tsuite {
		m := new(Manager)
		m.osStat = func(string) (os.FileInfo, error) {
			return testFileInfo{isDir: true}, nil
		}
		m.userLookup = func(name string) (*user.User, error) {
			return tcase.getUser, nil
		}
		m.initRemote = func() error {
			m.sshConn = goSshClient{new(ssh.Client)}
			return nil
		}
		workspaceURI, err := ParseURI(tcase.inWorkspace)
		require.NoError(t, err)
		require.NoError(t, m.init(discardLogger, workspaceURI))

		wantURI, err := ParseURI(tcase.wantURI)
		require.NoError(t, err)

		// sut
		actualURI, err := m.URI(tcase.inPath)
		require.NoError(t, err)
		assert.Equal(t, wantURI.String(), actualURI.String())
	}
}

func newManagerIntegration(t *testing.T) *Manager {
	m := new(Manager)
	m.osStat = func(string) (os.FileInfo, error) {
		return testFileInfo{isDir: true}, nil
	}
	m.userLookup = func(name string) (*user.User, error) {
		return new(user.User), nil
	}
	cwd, err := CurrentUserHostURI(".")
	require.NoError(t, err)
	require.NoError(t, m.init(discardLogger, cwd))
	return m
}

func TestManagerOpenIntegration(t *testing.T) {
	t.Run("returns os.ErrNotExist if file does not exist in read-only mode", func(t *testing.T) {
		m := newManagerIntegration(t)

		// only way to guarantee that the file won't exist
		// is creating it and then removing it
		f, err := ioutil.TempFile("", "workspace_test")
		require.NoError(t, err)
		err = os.Remove(f.Name())
		require.NoError(t, err)
		nonexistent, err := CurrentUserHostURI(f.Name())

		// sut
		_, err = m.Open(nonexistent, cell.NewBuffer(), URI{}, true)
		require.Equal(t, os.ErrNotExist, err)
		require.True(t, os.IsNotExist(err))
	})
}

func TestManagerIntegration(t *testing.T) {
	t.Run("sets the command dir to the workspace directory", func(t *testing.T) {
		m := newManagerIntegration(t)

		// create a file in a known directory
		tempDir, err := ioutil.TempDir("", "workspace_test")
		require.NoError(t, err)
		f, err := ioutil.TempFile(tempDir, "workspace_test")
		require.NoError(t, err)

		// re-initialize with temp dir as cwd
		tempDirURI, err := CurrentUserHostURI(tempDir)
		require.NoError(t, err)
		require.NoError(t, m.init(discardLogger, tempDirURI))

		// sut
		pid, err := m.Command("ls", "-altrh", ".")

		stdout, err := m.StdoutPipe(pid)
		require.NoError(t, err)

		err = m.Start(pid)
		require.NoError(t, err)

		data, err := ioutil.ReadAll(stdout)
		require.NoError(t, err)

		err = m.Wait(pid)
		require.NoError(t, err)

		assert.True(t, strings.Contains(string(data), filepath.Base(f.Name())))
	})
}
