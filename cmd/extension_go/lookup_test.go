// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// fakeFileInfo satisfies os.FileInfo for paths scripted into fakeFS.
type fakeFileInfo struct {
	name string
	dir  bool
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return 0o755 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.dir }
func (f fakeFileInfo) Sys() any           { return nil }

// fakeFS implements workspaceapi.FileSystem for resolver tests.
// Paths that exist must be registered in `files` (regular file) or
// `dirs` (directory). Any other Stat returns os.ErrNotExist.
type fakeFS struct {
	files map[string]bool
	dirs  map[string]bool
}

func newFakeFS() *fakeFS {
	return &fakeFS{files: map[string]bool{}, dirs: map[string]bool{}}
}

func (f *fakeFS) addFile(path string) *fakeFS { f.files[path] = true; return f }
func (f *fakeFS) addDir(path string) *fakeFS  { f.dirs[path] = true; return f }

func (f *fakeFS) URI(_ string) (workspaceapi.URI, error) {
	return workspaceapi.URI{}, nil
}

func (f *fakeFS) OpenFile(_ string, _ int, _ os.FileMode) (workspaceapi.File, error) {
	return nil, errors.New("not supported")
}

func (f *fakeFS) Remove(_ string) error { return errors.New("not supported") }
func (f *fakeFS) MkdirAll(_ string, _ os.FileMode) error {
	return errors.New("not supported")
}

func (f *fakeFS) Stat(name string) (os.FileInfo, error) {
	if f.files[name] {
		return fakeFileInfo{name: name, dir: false}, nil
	}
	if f.dirs[name] {
		return fakeFileInfo{name: name, dir: true}, nil
	}
	return nil, &fs.PathError{Op: "stat", Path: name, Err: os.ErrNotExist}
}

func (f *fakeFS) ReadDir(_ string) ([]os.DirEntry, error) {
	return nil, errors.New("not supported")
}

// scriptedCmd records expected exit code, stdout payload, and a tag
// recorded into log for assertions.
type scriptedCmd struct {
	stdout string
	stderr string
	err    error
}

// fakeExecutor implements workspaceapi.Executor. It dispatches on
// (cmd.Path, cmd.Args[0..]) by joining them with spaces and looking
// up the resulting key in `responses`. Unknown commands return an
// error from Start.
type fakeExecutor struct {
	mu        sync.Mutex
	responses map[string]scriptedCmd
	calls     []string
	nextPid   workspaceapi.Pid
}

func newFakeExecutor() *fakeExecutor {
	return &fakeExecutor{
		responses: map[string]scriptedCmd{},
		nextPid:   1,
	}
}

func (e *fakeExecutor) respond(key string, r scriptedCmd) *fakeExecutor {
	e.responses[key] = r
	return e
}

func (e *fakeExecutor) callKey(cmd workspaceapi.Cmd) string {
	return strings.Join(append([]string{cmd.Path}, cmd.Args...), " ")
}

func (e *fakeExecutor) Start(_ context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
	e.mu.Lock()
	key := e.callKey(cmd)
	e.calls = append(e.calls, key)
	resp, ok := e.responses[key]
	pid := e.nextPid
	e.nextPid++
	e.mu.Unlock()

	if !ok {
		return 0, fmt.Errorf("fakeExecutor: no scripted response for %q", key)
	}
	if cmd.Stdout != nil && resp.stdout != "" {
		_, _ = io.Copy(cmd.Stdout, bytes.NewBufferString(resp.stdout))
	}
	if cmd.Stderr != nil && resp.stderr != "" {
		_, _ = io.Copy(cmd.Stderr, bytes.NewBufferString(resp.stderr))
	}
	if cmd.Watcher != nil {
		ch := cmd.Watcher.WatchProcess()
		go func(err error) {
			if ch != nil {
				ch <- err
			}
		}(resp.err)
	}
	return pid, nil
}

func (e *fakeExecutor) Signal(_ workspaceapi.Pid, _ syscall.Signal) error {
	return nil
}

func (e *fakeExecutor) Close() error { return nil }

const (
	envProbe    = `sh -c printf '%s\n%s\n' "$SHELL" "$HOME"`
	shellProbe  = "/bin/zsh -lc command -v gopls"
	shellProbeS = "sh -lc command -v gopls"
)

func TestResolveGoplsBinary(t *testing.T) {
	t.Run("lsp_path wins without probing", func(t *testing.T) {
		fs := newFakeFS()
		ex := newFakeExecutor()
		got, err := resolveGoplsBinary(context.Background(), fs, ex, "/etc/gopls")
		require.NoError(t, err)
		assert.Equal(t, "/etc/gopls", got)
		assert.Empty(t, ex.calls, "no executor probes should run when lsp_path is set")
	})

	t.Run("shell probe success", func(t *testing.T) {
		fs := newFakeFS()
		ex := newFakeExecutor().
			respond(envProbe, scriptedCmd{stdout: "/bin/zsh\n/home/u\n"}).
			respond(shellProbe, scriptedCmd{stdout: "/home/u/go/bin/gopls\n"})
		got, err := resolveGoplsBinary(context.Background(), fs, ex, "")
		require.NoError(t, err)
		assert.Equal(t, "/home/u/go/bin/gopls", got)
	})

	t.Run("shell probe fails then well-known hits ~/go/bin", func(t *testing.T) {
		fs := newFakeFS().addFile("/home/u/go/bin/gopls")
		ex := newFakeExecutor().
			respond(envProbe, scriptedCmd{stdout: "/bin/zsh\n/home/u\n"}).
			respond(shellProbe, scriptedCmd{err: errors.New("not found")})
		got, err := resolveGoplsBinary(context.Background(), fs, ex, "")
		require.NoError(t, err)
		assert.Equal(t, "/home/u/go/bin/gopls", got)
	})

	t.Run("empty shell falls back to sh", func(t *testing.T) {
		fs := newFakeFS()
		ex := newFakeExecutor().
			respond(envProbe, scriptedCmd{stdout: "\n/home/u\n"}).
			respond(shellProbeS, scriptedCmd{stdout: "/opt/homebrew/bin/gopls\n"})
		got, err := resolveGoplsBinary(context.Background(), fs, ex, "")
		require.NoError(t, err)
		assert.Equal(t, "/opt/homebrew/bin/gopls", got)
	})

	t.Run("well-known hits absolute path when home is empty", func(t *testing.T) {
		fs := newFakeFS().addFile("/usr/local/go/bin/gopls")
		ex := newFakeExecutor().
			respond(envProbe, scriptedCmd{stdout: "\n\n"}).
			respond(shellProbeS, scriptedCmd{err: errors.New("not found")})
		got, err := resolveGoplsBinary(context.Background(), fs, ex, "")
		require.NoError(t, err)
		assert.Equal(t, "/usr/local/go/bin/gopls", got)
	})

	t.Run("directory at well-known location is ignored", func(t *testing.T) {
		fs := newFakeFS().addDir("/usr/local/go/bin/gopls")
		ex := newFakeExecutor().
			respond(envProbe, scriptedCmd{stdout: "\n\n"}).
			respond(shellProbeS, scriptedCmd{err: errors.New("not found")})
		_, err := resolveGoplsBinary(context.Background(), fs, ex, "")
		require.Error(t, err)
	})

	t.Run("everything fails returns error", func(t *testing.T) {
		fs := newFakeFS()
		ex := newFakeExecutor().
			respond(envProbe, scriptedCmd{stdout: "/bin/zsh\n/home/u\n"}).
			respond(shellProbe, scriptedCmd{err: errors.New("not found")})
		_, err := resolveGoplsBinary(context.Background(), fs, ex, "")
		require.Error(t, err)
	})

	t.Run("non-absolute shell output is rejected", func(t *testing.T) {
		fs := newFakeFS()
		ex := newFakeExecutor().
			respond(envProbe, scriptedCmd{stdout: "/bin/zsh\n/home/u\n"}).
			respond(shellProbe, scriptedCmd{stdout: "gopls\n"})
		_, err := resolveGoplsBinary(context.Background(), fs, ex, "")
		require.Error(t, err)
	})
}

func TestHasGoProjectFiles(t *testing.T) {
	cases := []struct {
		name  string
		files []string
		want  bool
	}{
		{"empty workspace", nil, false},
		{"go.mod present", []string{"go.mod"}, true},
		{"go.sum only", []string{"go.sum"}, true},
		{"go.work only", []string{"go.work"}, true},
		{"unrelated file", []string{"README.md"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := newFakeFS()
			for _, f := range tc.files {
				fs.addFile(f)
			}
			assert.Equal(t, tc.want, hasGoProjectFiles(context.Background(), fs))
		})
	}
}

func TestReadGoplsLspPath(t *testing.T) {
	t.Run("missing key returns empty without warning", func(t *testing.T) {
		cfg := config.JSONFromMap(map[string]any{})
		mn := &mockNotifications{}
		assert.Equal(t, "", readGoplsLspPath(cfg, mn))
		assert.Empty(t, mn.getMessages())
	})

	t.Run("valid path is returned verbatim", func(t *testing.T) {
		cfg := config.JSONFromMap(map[string]any{"lsp_path": "/opt/gopls"})
		mn := &mockNotifications{}
		assert.Equal(t, "/opt/gopls", readGoplsLspPath(cfg, mn))
		assert.Empty(t, mn.getMessages())
	})

	t.Run("path containing space is rejected with warn", func(t *testing.T) {
		cfg := config.JSONFromMap(map[string]any{"lsp_path": "/opt/g opls"})
		mn := &mockNotifications{}
		assert.Equal(t, "", readGoplsLspPath(cfg, mn))
		require.Len(t, mn.getMessages(), 1)
		assert.Equal(t, browserapi.LevelWarn, mn.getMessages()[0].Level)
		assert.Contains(t, mn.getMessages()[0].Message, "without spaces")
	})

	t.Run("wrong type is rejected with warn", func(t *testing.T) {
		cfg := config.JSONFromMap(map[string]any{"lsp_path": 42})
		mn := &mockNotifications{}
		assert.Equal(t, "", readGoplsLspPath(cfg, mn))
		require.Len(t, mn.getMessages(), 1)
		assert.Equal(t, browserapi.LevelWarn, mn.getMessages()[0].Level)
	})

	t.Run("nil config returns empty", func(t *testing.T) {
		assert.Equal(t, "", readGoplsLspPath(nil, &mockNotifications{}))
	})
}
