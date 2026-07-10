// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package workspacessh

import (
	"context"
	"fmt"

	"os"
	"os/user"
	"path/filepath"
	"sync"
	"syscall"
	"testing"

	multierr "github.com/ernestrc/go-multierror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/workspace"
	"unstable.build/go-tui/workspace/workspacetest"
)

func TestNewScheme(t *testing.T) {
	tsuite := []struct {
		desc         string
		workspaceURI string
		expectedURI  string
		expectedErr  string
	}{
		{"no port no user workspace absolute", "ssh://ernest.photography", "", ""},
		{"port no user workspace absolute", "ssh://ernest.photography:4222", "", ""},
		{"port user workspace absolute", "ssh://ernie@ernest.photography:4222", "", ""},
		{"port user workspace absolute slash", "ssh://ernie@ernest.photography:4222/", "", ""},
		{"no port user workspace absolute slash", "ssh://ernie@ernest.photography/", "", ""},
		{"no port user workspace relative slash", "ssh://ernie@ernest.photography/~/src",
			"ssh://ernie@ernest.photography/home/ernie/src", ""},
		{"no host returns error", "ssh:///tmp", "", "could not parse ssh workspaceapi.URI: ssh scheme with empty host is invalid"},
		{"different scheme returns error", "file:///tmp", "", "invalid non-ssh scheme"},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.desc, func(t *testing.T) {
			workspaceURI, err := workspaceapi.ParseURI(tcase.workspaceURI)
			require.NoError(t, err)

			// sut
			ch := make(chan string, 1) // when uri is a file URI it gets called twice
			s, err := newTestScheme(config.NopConfig(), workspaceURI,
				func(ctx context.Context, uri workspaceapi.URI, closeHook func(error)) (schemeapi.Scheme, error) {
					go func() { ch <- uri.String() }()
					return workspacetest.NewNopScheme("test")(
						ctx, config.NopConfig(), uri)
				})
			if tcase.expectedErr != "" {
				assert.EqualError(t, err, tcase.expectedErr)
			} else {
				assert.NoError(t, err)
				expectedURI := tcase.expectedURI
				if expectedURI == "" {
					expectedURI = workspaceURI.String()
				}
				actualURI := <-ch
				assert.Equal(t, expectedURI, actualURI)
				assert.NoError(t, s.Close())
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
	}

	for _, tcase := range tsuite {
		t.Run(tcase.desc, func(t *testing.T) {
			workspaceURI, err := workspaceapi.ParseURI(tcase.workspaceURI)
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

				expectedURI, err := workspaceapi.ParseURI(tcase.expectedOut)
				require.NoError(t, err)
				assert.Equal(t, expectedURI.String(), actualOut.String())
			}

		})
	}
}

// TestConnectSchemeUsesRune asserts that the workspace-scheme bootstrap
// looks for the `rune` binary on the remote — the same name that
// `cmd/rune` accepts via its `--workspace-server / -x` flag (see
// cmd/rune/main.go). A previous version of the code looked for an
// obsolete binary called `six`, which made every real connection fail
// with "six executable was not found on remote" even when authentication
// succeeded. The bug went undetected because the docker e2e matrix
// short-circuits via TestAuthDial and never reaches connectScheme.
func TestConnectSchemeUsesRune(t *testing.T) {
	rec := &recordingRemote{}

	s := new(scheme)
	s.ctx, s.cancelCtx = context.WithCancel(context.Background())
	defer s.cancelCtx()
	s.remoteFn = func(context.Context, sshConfig, workspaceapi.URI) (remote, error) {
		return rec, nil
	}
	s.getUser = func() (*user.User, error) {
		return &user.User{Username: "test", HomeDir: "/home/test"}, nil
	}
	s.ui = errorUI{}
	uri, err := workspaceapi.ParseURI("ssh://test@example.com/tmp")
	require.NoError(t, err)
	s.user, s.homedir, s.hostPort, s.basePath, err = parseWorkspaceURI(uri, s.getUser)
	require.NoError(t, err)

	closeHook := func(error) {}
	scheme, err := s.connectScheme(context.Background(), uri, closeHook)
	require.NoError(t, err)
	if scheme != nil {
		_ = scheme.Close()
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	require.NotEmpty(t, rec.commands, "expected connectScheme to issue commands")

	// First command is `which <bin>` — that's the canonical signal.
	first := rec.commands[0]
	assert.Equal(t, remotePathEnv+" which", first.Path,
		"first command should be `which <remote bin>` with the "+
			"~/.local/bin PATH injection; got %+v", first)
	require.NotEmpty(t, first.Args)
	assert.Equal(t, "rune", first.Args[0],
		"connectScheme must look for the `rune` binary on the remote "+
			"(matches cmd/rune --workspace-server). Got %q.",
		first.Args[0])

	// The actual workspace-server invocation should also use `rune`.
	var sawServer bool
	for _, c := range rec.commands {
		if c.Path == remotePathEnv+" rune" {
			sawServer = true
			assert.Contains(t, c.Args, "-x",
				"rune workspace server should be started with -x; got %+v", c)
			break
		}
	}
	assert.True(t, sawServer,
		"expected at least one command to invoke `rune` with the "+
			"~/.local/bin PATH injection; saw %+v", rec.commands)
}

// TestConnectSchemeSkipPreflight asserts that when
// sshConfig.skipPreflight is true, connectScheme does NOT issue the
// `which rune` and `ls <path>` pre-flight probes. Each probe opens a
// fresh SSH session channel, so skipping them is the user-visible
// escape hatch on servers with a tight MaxSessions budget (manual
// scenario workspace/workspacessh/manual_test/12_max_sessions_one.sh).
func TestConnectSchemeSkipPreflight(t *testing.T) {
	rec := &recordingRemote{}

	s := new(scheme)
	s.ctx, s.cancelCtx = context.WithCancel(context.Background())
	defer s.cancelCtx()
	s.cfg.skipPreflight = true
	s.remoteFn = func(context.Context, sshConfig, workspaceapi.URI) (remote, error) {
		return rec, nil
	}
	s.getUser = func() (*user.User, error) {
		return &user.User{Username: "test", HomeDir: "/home/test"}, nil
	}
	s.ui = errorUI{}
	uri, err := workspaceapi.ParseURI("ssh://test@example.com/tmp")
	require.NoError(t, err)
	s.user, s.homedir, s.hostPort, s.basePath, err = parseWorkspaceURI(uri, s.getUser)
	require.NoError(t, err)

	closeHook := func(error) {}
	scheme, err := s.connectScheme(context.Background(), uri, closeHook)
	require.NoError(t, err)
	if scheme != nil {
		_ = scheme.Close()
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	require.Len(t, rec.commands, 1,
		"skip_preflight must avoid the `which` and `ls` probes; only "+
			"the rune workspace-server invocation should be issued. "+
			"Got %+v", rec.commands)
	assert.Equal(t, remotePathEnv+" rune", rec.commands[0].Path,
		"the only command issued must be the rune workspace server, "+
			"launched with the ~/.local/bin PATH injection so the "+
			"supported install location works even without preflight")
	assert.Contains(t, rec.commands[0].Args, "-x",
		"rune workspace server should be started with -x; got %+v",
		rec.commands[0])
}

func TestParseWorkspaceURIHomeDir(t *testing.T) {
	tsuite := []struct {
		desc        string
		uri         string
		getUser     func() (*user.User, error)
		wantUser    string
		wantHomeDir string
	}{
		{
			desc:        "non-root user under /home",
			uri:         "ssh://test@example.com/~/src",
			wantUser:    "test",
			wantHomeDir: "/home/test",
		},
		{
			desc:        "root user maps to /root",
			uri:         "ssh://root@example.com/~/src",
			wantUser:    "root",
			wantHomeDir: "/root",
		},
		{
			desc: "empty user falls back to local user",
			uri:  "ssh://example.com/~/src",
			getUser: func() (*user.User, error) {
				return &user.User{Username: "root"}, nil
			},
			wantUser:    "",
			wantHomeDir: "/root",
		},
	}

	for _, tc := range tsuite {
		t.Run(tc.desc, func(t *testing.T) {
			uri, err := workspaceapi.ParseURI(tc.uri)
			require.NoError(t, err)

			getUser := tc.getUser
			if getUser == nil {
				getUser = func() (*user.User, error) {
					return &user.User{Username: "test", HomeDir: "/home/test"}, nil
				}
			}

			gotUser, gotHomeDir, _, gotBasePath, err := parseWorkspaceURI(uri, getUser)
			require.NoError(t, err)
			assert.Equal(t, tc.wantUser, gotUser)
			assert.Equal(t, tc.wantHomeDir, gotHomeDir,
				"~ should expand to the remote user's real home")
			assert.Equal(t, filepath.Join(tc.wantHomeDir, "src"), gotBasePath,
				"basePath must expand ~ using homeDir")
		})
	}
}

func TestIntegrationCanWorkspaceURI(t *testing.T) {
	tsuite := []struct {
		workspaceURI string
		uri          string
		expectedOut  bool
	}{
		{"ssh://ernest.photography/", "ssh://ernest.photography/tmp", true},
		{"ssh://ernest.photography/", "file://ernest.photography/tmp", false},
		{"ssh://ernest.photography/", "ssh://git@ernest.photography/tmp", false},
		{"ssh://unstablebuild@ernest.photography/", "ssh://git@ernest.photography/tmp", false},
		{"ssh://git@ernest.photography/", "ssh://git@ernest.photography/tmp", true},
		{"ssh://unstablebuild@ernest.photography/", "ssh://git@ernest.photography/tmp", false},
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
	}

	for i, tcase := range tsuite {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			inURI, err := workspaceapi.ParseURI(tcase.uri)
			require.NoError(t, err)

			inWorkspaceURI, err := workspaceapi.ParseURI(tcase.workspaceURI)
			require.NoError(t, err)

			fileScheme := newNopScheme(t, inWorkspaceURI)
			inWorkspace := workspace.NewSchemeWorkspace(inWorkspaceURI, fileScheme, inlineSchedule)

			// sut
			actual, err := workspace.IsWorkspaceURI(inWorkspace, inURI)
			require.NoError(t, err)
			assert.Equal(t, tcase.expectedOut, actual)
		})
	}
}

func TestIntegrationManagerIsWorkspaceFile(t *testing.T) {
	fileWorkspacePath, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	fileWorkspaceURI, err := workspaceapi.ParseURI(filepath.Join("file://", fileWorkspacePath))
	require.NoError(t, err)

	sshWorkspaceURI, err := workspaceapi.ParseURI("ssh://ernest.photography/~/")
	require.NoError(t, err)

	manager := workspace.NewManager(config.NopConfig(), inlineSchedule)
	require.NoError(t, manager.RegisterScheme(workspace.FileScheme, workspace.NewFileScheme))
	require.NoError(t, manager.RegisterScheme(Scheme,
		func(ctx context.Context, cfg config.Config, uri workspaceapi.URI) (schemeapi.Scheme, error) {
			return newTestScheme(cfg, uri, nil)
		}))

	ctx := context.Background()

	fileWorkspace, err := manager.AddWorkspace(ctx, fileWorkspaceURI)
	require.NoError(t, err)

	sshWorkspace, err := manager.AddWorkspace(ctx, sshWorkspaceURI)
	require.NoError(t, err)

	tsuite := []struct {
		uri               string
		expectedWorkspace workspace.Workspace
		expectedFound     bool
	}{
		{"ssh://ernest.photography/~/hello.txt", sshWorkspace, true},
		{"ssh://unstable.build/~/hello.txt", nil, false},
		{"file:///hello.txt", fileWorkspace, true},
		{"file:///home/git/hello.txt", fileWorkspace, true},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.uri, func(t *testing.T) {
			inURI, err := workspaceapi.ParseURI(tcase.uri)
			require.NoError(t, err)

			// sut
			actual, ok, err := manager.Workspace(inURI)
			require.NoError(t, err)
			assert.Equal(t, ok, tcase.expectedFound)
			assert.Equal(t, tcase.expectedWorkspace, actual)
		})
	}
}

func TestSSHScheme(t *testing.T) {
	var cleanup []func() error
	t.Run("with memory scheme remote", func(t *testing.T) {
		workspacetest.TestWorkspaceSchemeFiles(t, func(t *testing.T) schemeapi.Scheme {
			workspaceURI, err := workspaceapi.ParseURI("ssh://test@host.com/")
			require.NoError(t, err)
			remoteURI, err := workspaceapi.ParseURI("memory:///")
			require.NoError(t, err)

			memScheme, err := workspace.NewMemoryScheme(
				context.Background(), config.NopConfig(), remoteURI)
			require.NoError(t, err)

			s, err := newTestScheme(config.NopConfig(), workspaceURI,
				func(ctx context.Context, uri workspaceapi.URI,
					closeHook func(error)) (schemeapi.Scheme, error) {
					return memScheme, nil
				})
			require.NoError(t, err)
			cleanup = append(cleanup, s.Close)
			return s
		})
	})

	t.Run("with file scheme remote", func(t *testing.T) {
		workspacetest.TestWorkspaceSchemeFiles(t, func(t *testing.T) schemeapi.Scheme {
			dir, err := os.MkdirTemp("", "ssh_scheme_suite")
			require.NoError(t, err)

			fileURI, err := workspaceapi.ParseURI("file://" + dir)
			require.NoError(t, err)

			workspaceURI, err := workspaceapi.ParseURI("ssh://host.com" + dir)
			require.NoError(t, err)

			fileScheme, err := workspace.NewFileScheme(
				context.Background(), config.NopConfig(), fileURI)
			require.NoError(t, err)

			s, err := newTestScheme(config.NopConfig(), workspaceURI,
				func(ctx context.Context, uri workspaceapi.URI,
					closeHook func(error)) (schemeapi.Scheme, error) {
					return fileScheme, nil
				})
			require.NoError(t, err)
			cleanup = append(cleanup, func() (ret error) {
				if err := s.Close(); err != nil {
					ret = multierr.Append(ret, err)
				}
				if err := os.RemoveAll(dir); err != nil {
					ret = multierr.Append(ret, err)
				}
				return ret
			})
			return s
		})
	})
	for _, clean := range cleanup {
		_ = clean()
	}
}

type nopExecutor struct {
}

func (n nopExecutor) StartCommand(
	ctx context.Context, cmd workspaceapi.Cmd,
) (workspaceapi.Pid, error) {
	return 0, nil
}

func (n nopExecutor) Signal(workspaceapi.Pid, syscall.Signal) error {
	return nil
}
func (n nopExecutor) Close() error {
	return nil
}

type nopRemote struct {
}

func (n nopRemote) NewSession() (schemeapi.Executor, error) {
	return nopExecutor{}, nil
}

func (n nopRemote) Close() error {
	return nil
}

// recordingRemote captures every command issued via NewSession/StartCommand
// so tests can assert on the binary that connectScheme runs on the remote.
// Each StartCommand is treated as a successful exit (status 0) by signaling
// the supplied ProcessWatcher with nil.
type recordingRemote struct {
	mu       sync.Mutex
	commands []workspaceapi.Cmd
}

func (r *recordingRemote) NewSession() (schemeapi.Executor, error) {
	return &recordingExecutor{remote: r}, nil
}

func (r *recordingRemote) Close() error { return nil }

type recordingExecutor struct {
	remote *recordingRemote
}

func (e *recordingExecutor) StartCommand(
	_ context.Context, cmd workspaceapi.Cmd,
) (workspaceapi.Pid, error) {
	e.remote.mu.Lock()
	e.remote.commands = append(e.remote.commands, cmd)
	e.remote.mu.Unlock()
	if cmd.Watcher != nil && cmd.Watcher.WatchProcess() != nil {
		go func() { cmd.Watcher.WatchProcess() <- nil }()
	}
	return 1, nil
}

func (e *recordingExecutor) Signal(workspaceapi.Pid, syscall.Signal) error { return nil }
func (e *recordingExecutor) Close() error                                  { return nil }

// errorUI fails any prompt; tests use it because newTestScheme stubs the
// remote, so prompts should never fire.
type errorUI struct{}

func (errorUI) PromptSecret(context.Context, string) (string, error) {
	return "", fmt.Errorf("unexpected prompt: secret")
}

func (errorUI) PromptText(context.Context, string, string) (string, error) {
	return "", fmt.Errorf("unexpected prompt: text")
}

func (errorUI) PromptChoice(context.Context, string, []string) (int, error) {
	return -1, fmt.Errorf("unexpected prompt: choice")
}

func (errorUI) Notify(NotificationLevel, string) {}

func newTestScheme(
	cfg config.Config, workspaceURI workspaceapi.URI,
	connectSchemeFn func(ctx context.Context,
		uri workspaceapi.URI, closeHook func(error)) (schemeapi.Scheme, error),
) (schemeapi.Scheme, error) {
	s := new(scheme)
	s.ctx, s.cancelCtx = context.WithCancel(context.Background())
	s.remoteFn = func(context.Context, sshConfig, workspaceapi.URI) (remote, error) {
		return nopRemote{}, nil
	}
	s.getUser = func() (*user.User, error) {
		return &user.User{Username: "git", HomeDir: "/home/git"}, nil
	}
	s.ui = errorUI{}
	if connectSchemeFn == nil {
		connectSchemeFn = func(ctx context.Context, uri workspaceapi.URI, closeHook func(error)) (
			schemeapi.Scheme, error,
		) {
			return workspacetest.NewNopScheme("test")(ctx, config.NopConfig(), uri)
		}
	}
	s.connectSchemeFn = connectSchemeFn

	err := s.init(context.Background(), sshConfig{}, workspaceURI)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func newNopScheme(t *testing.T, workspaceURI workspaceapi.URI) *scheme {
	s, err := newTestScheme(config.NopConfig(), workspaceURI, nil)
	require.NoError(t, err)
	return s.(*scheme)
}

// inlineSchedule is a synchronous workspace.ScheduleNextTick stub
// that runs fn on the calling goroutine. Test-only: production code
// must use the host event-loop scheduler so reload's buffer
// mutations do not run on a worker goroutine.
func inlineSchedule(fn func()) bool {
	fn()
	return true
}
