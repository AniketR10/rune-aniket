package workspace

import (
	"io/ioutil"
	"os/user"
	"testing"

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
		desc        string
		inWorkspace string
		expectGetwd string
		wantChdir   string
	}{
		{"does not change working directory if already current dir",
			"file:///tmp", "/tmp", ""},
		{"changes working directory if not current dir",
			"file:///tmp", "/tmp/hello", "/tmp"},
	}
	for _, tcase := range tsuite {
		t.Run(tcase.desc, func(t *testing.T) {
			m := new(Manager)
			m.osGetwd = func() (string, error) {
				return tcase.expectGetwd, nil
			}
			var actualChdir string
			m.osChdir = func(chdir string) error {
				actualChdir = chdir
				return nil
			}
			uri, err := ParseURI(tcase.inWorkspace)
			require.NoError(t, err)
			require.NoError(t, m.init(discardLogger, uri))
			assert.Equal(t, tcase.wantChdir, actualChdir)
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
		m.osGetwd = func() (string, error) {
			return tcase.getcwd, nil
		}
		m.osChdir = func(chdir string) error {
			panic("should not change cwd")
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
