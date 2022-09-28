package ssh

import (
	"bytes"
	"fmt"
	"io"
	"os/user"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/workspace"
)

type nopExecutor struct {
}

func (n nopExecutor) Command(name string, arg ...string) (workspace.Pid, error) {
	return 0, nil
}

func (n nopExecutor) Start(workspace.Pid) error {
	return nil
}

func (n nopExecutor) Signal(workspace.Pid, syscall.Signal) error {
	return nil
}

func (n nopExecutor) StderrPipe(workspace.Pid) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (n nopExecutor) StdinPipe(workspace.Pid) (io.WriteCloser, error) {
	var b bytes.Buffer
	return nopWriteCloser{Writer: &b}, nil
}

func (n nopExecutor) StdoutPipe(workspace.Pid) (io.ReadCloser, error) {
	return n.StderrPipe(0)
}

func (n nopExecutor) Wait(workspace.Pid) error {
	return nil
}

func (n nopExecutor) Close() error {
	return nil
}

type nopRemote struct {
}

func (n nopRemote) NewSession() (workspace.Executor, error) {
	return nopExecutor{}, nil
}

func (n nopRemote) Close() error {
	return nil
}

func newTestScheme(workspaceURI workspace.URI) (*scheme, error) {
	s := new(scheme)
	s.remoteFn = func(sshConfig, workspace.URI) (remote, error) {
		return nopRemote{}, nil
	}
	s.getUser = func() (*user.User, error) {
		return &user.User{Username: "git", HomeDir: "/home/git"}, nil
	}
	s.connectSchemeFn = func(uri workspace.URI, closeHook func(error)) (workspace.Scheme, error) {
		return workspace.NewNopScheme(config.NopConfig(), workspace.URI{})
	}

	err := s.init(sshConfig{}, workspaceURI)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func newNopScheme(t *testing.T, workspaceURI workspace.URI) *scheme {
	s, err := newTestScheme(workspaceURI)
	require.NoError(t, err)
	return s
}

func TestNewScheme(t *testing.T) {
	tsuite := []struct {
		desc         string
		workspaceURI string
		expectedErr  string
	}{
		{"no port no user workspace absolute", "ssh://ernest.photography", ""},
		{"port no user workspace absolute", "ssh://ernest.photography:4222", ""},
		{"port user workspace absolute", "ssh://ernie@ernest.photography:4222", ""},
		{"port user workspace absolute slash", "ssh://ernie@ernest.photography:4222/", ""},
		{"no port user workspace absolute slash", "ssh://ernie@ernest.photography/", ""},
		{"no host returns error", "ssh:///tmp", "could not parse ssh workspace URI: ssh scheme with empty host is invalid"},
		{"different scheme returns error", "file:///tmp", "invalid non-ssh scheme"},
		{"file URI is ok", "ssh://ernie@ernest.photography/tmp/file.txt", ""},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.desc, func(t *testing.T) {
			workspaceURI, err := workspace.ParseURI(tcase.workspaceURI)
			require.NoError(t, err)

			// sut
			_, err = newTestScheme(workspaceURI)
			if tcase.expectedErr != "" {
				assert.EqualError(t, err, tcase.expectedErr)
			} else {
				assert.NoError(t, err)
			}

		})
	}
}

func TestURI(t *testing.T) {
	tsuite := []struct {
		desc         string
		workspaceURI string
		inPath       string
		expectedOut  string
		expectedErr  string
	}{
		{"no port no user workspace absolute", "ssh://ernest.photography", "/",
			"ssh://ernest.photography/", ""},
		{"no port no user workspace home relative", "ssh://ernest.photography", "/~/",
			"ssh://ernest.photography/home/git", ""}, // newTestScheme sets git as default user
		{"port no user workspace home relative", "ssh://ernest.photography:455", "/~/",
			"ssh://ernest.photography:455/home/git", ""},
		{"port no user workspace absolute", "ssh://ernest.photography:455", "/tmp/var",
			"ssh://ernest.photography:455/tmp/var", ""},
		{"port user workspace absolute", "ssh://ernie@ernest.photography:455", "/tmp/var",
			"ssh://ernie@ernest.photography:455/tmp/var", ""},
		{"port user workspace home relative", "ssh://ernie@ernest.photography:455", "~/src/blue",
			"ssh://ernie@ernest.photography:455/home/ernie/src/blue", ""},
		{"relative common path implicit home folder", "ssh://ernest.photography/home/git/src/blue", "~/src/blue/fireplace.txt",
			"ssh://ernest.photography/home/git/src/blue/fireplace.txt", ""},
		{"relative common path explicit home folder", "ssh://ernest.photography/home/git/src/blue", "/home/git/src/blue/fireplace.txt",
			"ssh://ernest.photography/home/git/src/blue/fireplace.txt", ""},
		{"relative upwards workspace tree", "ssh://ernest.photography/home/git/src/blue", "../../",
			"ssh://ernest.photography/home/git", ""},
		{"relative upwards workspace tree ending slash", "ssh://ernest.photography/home/git/src/blue/", "../../",
			"ssh://ernest.photography/home/git", ""},
		{"relative upwards workspace tree ./ path", "ssh://ernest.photography/home/git/src/blue", "./../../file.txt",
			"ssh://ernest.photography/home/git/file.txt", ""},
		{"workspace URI with file uri should exclude the file", "ssh://ernest.photography/home/git/src/blue/file.txt", "hello.txt",
			"ssh://ernest.photography/home/git/src/blue/hello.txt", ""},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.desc, func(t *testing.T) {
			workspaceURI, err := workspace.ParseURI(tcase.workspaceURI)
			require.NoError(t, err)

			s := newNopScheme(t, workspaceURI)
			defer s.Close()

			// sut
			actualOut, actualErr := s.URI(tcase.inPath)
			if tcase.expectedErr != "" {
				assert.Error(t, actualErr)
				assert.Nil(t, actualOut)
			} else {
				assert.NoError(t, actualErr)

				expectedURI, err := workspace.ParseURI(tcase.expectedOut)
				require.NoError(t, err)
				assert.Equal(t, expectedURI.String(), actualOut.String())
			}

		})
	}
}

func TestIntegrationIsWorkspaceURI(t *testing.T) {
	tsuite := []struct {
		workspaceURI string
		uri          string
		expectedOut  bool
	}{
		{"ssh://ernest.photography/", "ssh://ernest.photography/tmp", true},
		{"ssh://ernest.photography/", "file://ernest.photography/tmp", false},
		{"ssh://ernest.photography/", "ssh://git@ernest.photography/tmp", false},
		{"ssh://ernestrc@ernest.photography/", "ssh://git@ernest.photography/tmp", false},
		{"ssh://git@ernest.photography/", "ssh://git@ernest.photography/tmp", true},
		{"ssh://ernestrc@ernest.photography/", "ssh://git@ernest.photography/tmp", false},
		{"ssh://git@ernest.photography/", "ssh://git@ernest.photography:12222/tmp", false},
		{"ssh://git@ernest.photography:12222/", "ssh://git@ernest.photography/tmp", false},
		{"ssh://ernest.photography/tmp", "ssh://ernest.photography/tmp", true},
		{"ssh://ernest.photography:122/tmp", "ssh://ernest.photography:122/tmp", true},
		{"ssh://git@ernest.photography:122/tmp", "ssh://ernest.photography:122/tmp", false},
		{"ssh://ernest.photography:122/tmp", "ssh://git@ernest.photography:122/tmp", false},
		{"ssh://ernest.photography/var", "ssh://ernest.photography/tmp", true}, // diff tree, but scheme should be able to handle it
		{"ssh://ernest.photography/var", "ssh://ernest.photography/var/file.txt", true},
		{"ssh://ernest.photography/var", "ssh://ernest.photography/var/dir/dir/dir/file.txt", true},
		{"ssh://ernest.photography:22/var", "ssh://ernest.photography:22/var/dir/dir/dir/file.txt", true},
		{"ssh://ernest.photography/var/", "ssh://ernest.photography/var/file.txt", true},
		{"ssh://root@ernest.photography/var/", "ssh://root@ernest.photography/var/file.txt", true},
		{"ssh://root@ernest.photography:1999/var/", "ssh://root@ernest.photography:1999/var/file.txt", true},
		// test workspace uri with file
		{"ssh://root@ernest.photography:1999/var/file.txt", "ssh://root@ernest.photography:1999/var/hello.txt", true},
	}

	for i, tcase := range tsuite {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			inURI, err := workspace.ParseURI(tcase.uri)
			require.NoError(t, err)

			inWorkspaceURI, err := workspace.ParseURI(tcase.workspaceURI)
			require.NoError(t, err)

			fileScheme := newNopScheme(t, inWorkspaceURI)
			inWorkspace := workspace.NewSchemeWorkspace(inWorkspaceURI, fileScheme)

			// sut
			actual := workspace.IsWorkspaceURI(inWorkspace, inURI)
			assert.Equal(t, tcase.expectedOut, actual)
		})
	}
}
