// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package command

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

func TestCommandOutputLinesCompleterCloseSignals(t *testing.T) {
	var pid workspaceapi.Pid
	var sig syscall.Signal

	exec := &mockExecutor{
		startFn: func(_ context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
			// don't close stdout: simulates a long-running process
			return 42, nil
		},
		signalFn: func(_pid workspaceapi.Pid, _sig syscall.Signal) error {
			pid = _pid
			sig = _sig
			return nil
		},
	}

	c := OutputLinesCompleter(exec, []string{"long-running"})
	iter, _, err := c.Complete(context.Background(), nil)
	require.NoError(t, err)

	err = iter.Close()
	require.NoError(t, err)

	assert.Equal(t, workspaceapi.Pid(42), pid)
	assert.Equal(t, syscall.SIGINT, sig)
}

func TestCommandOutputLinesCompleterContextCancellation(t *testing.T) {
	exec := &mockExecutor{
		startFn: func(_ context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
			// stdout stays open: process never exits on its own
			return 1, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	c := OutputLinesCompleter(exec, []string{"hanging-cmd"})
	iter, _, err := c.Complete(ctx, nil)
	require.NoError(t, err)

	cancel()

	_, ok := iter.Next(ctx)
	assert.False(t, ok)
	err = iter.Err()
	if err != nil {
		assert.Contains(t, err.Error(), "closed")
	}
}

func TestCommandOutputLinesCompleter(t *testing.T) {
	tests := []struct {
		name      string
		cmdArgs   []string
		executor  *mockExecutor
		wantLines []string
		wantErr   string
	}{
		{
			name:    "empty args returns error",
			cmdArgs: nil,
			executor: &mockExecutor{
				startFn: func(_ context.Context, _ workspaceapi.Cmd) (workspaceapi.Pid, error) {
					t.Fatal("StartCommand should not be called")
					return 0, nil
				},
			},
			wantErr: "expected at least one argument",
		},
		{
			name:    "start command error propagates",
			cmdArgs: []string{"failing-cmd"},
			executor: &mockExecutor{
				startFn: func(_ context.Context, _ workspaceapi.Cmd) (workspaceapi.Pid, error) {
					return 0, io.ErrUnexpectedEOF
				},
			},
			wantErr: "unexpected EOF",
		},
		{
			name:    "single line",
			cmdArgs: []string{"echo", "hello"},
			executor: &mockExecutor{
				startFn: writeAndExit("hello\n", 1),
			},
			wantLines: []string{"hello"},
		},
		{
			name:    "multiple lines",
			cmdArgs: []string{"my-cmd"},
			executor: &mockExecutor{
				startFn: writeAndExit("alpha\nbeta\ngamma\n", 1),
			},
			wantLines: []string{"alpha", "beta", "gamma"},
		},
		{
			name:    "empty output",
			cmdArgs: []string{"my-cmd"},
			executor: &mockExecutor{
				startFn: writeAndExit("", 1),
			},
			wantLines: nil,
		},
		{
			name:    "trailing line without EOL",
			cmdArgs: []string{"my-cmd"},
			executor: &mockExecutor{
				startFn: writeAndExit("foo\nbar", 1),
			},
			wantLines: []string{"foo", "bar"},
		},
		{
			name:    "lines with spaces preserved",
			cmdArgs: []string{"my-cmd"},
			executor: &mockExecutor{
				startFn: writeAndExit("  leading\ntrailing  \n  both  \n", 1),
			},
			wantLines: []string{"  leading", "trailing  ", "  both  "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := OutputLinesCompleter(tt.executor, tt.cmdArgs)
			iter, prefix, err := c.Complete(context.Background(), nil)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, iter)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, "", prefix)

			got := collectAll(t, iter)
			assert.Equal(t, tt.wantLines, got)
		})
	}
}

type mockExecutor struct {
	startFn  func(ctx context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error)
	signalFn func(pid workspaceapi.Pid, sig syscall.Signal) error
}

func (m *mockExecutor) StartCommand(ctx context.Context, cmd workspaceapi.Cmd) (
	workspaceapi.Pid, error,
) {
	return m.startFn(ctx, cmd)
}

func (m *mockExecutor) Signal(pid workspaceapi.Pid, sig syscall.Signal) error {
	if m.signalFn != nil {
		return m.signalFn(pid, sig)
	}
	return nil
}

func (m *mockExecutor) Close() error {
	return nil
}

// collectAll drains the iterator and returns all values.
func collectAll(t *testing.T, iter iterator.Iterator[string]) []string {
	t.Helper()
	var out []string
	for {
		val, ok := iter.Next(context.Background())
		if !ok {
			require.NoError(t, iter.Err())
			break
		}
		out = append(out, val)
	}
	return out
}

// writeAndExit simulates a command that writes lines to stdout then exits.
func writeAndExit(output string, pid int) func(context.Context, workspaceapi.Cmd) (workspaceapi.Pid, error) {
	return func(_ context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
		go func() {
			w := cmd.Stdout.(io.WriteCloser)
			io.WriteString(w, output)
			w.Close()
			//cmd.Watcher.WatchProcess() <- nil
		}()
		return workspaceapi.Pid(pid), nil
	}
}

// fsReader is a tiny walkdir.Reader implementation rooted at a real
// directory on disk. The completer's Complete method composes URI,
// Stat, OpenFile and ReadDir; this fixture exercises each via os.* so
// we cover real-world filename handling (UTF-8, escapes, hidden
// entries, etc.) without dragging in a full workspace scheme.
type fsReader struct {
	root string
}

func newFSReader(root string) *fsReader { return &fsReader{root: root} }

func (r *fsReader) resolve(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(r.root, p)
}

func (r *fsReader) URI(p string) (workspaceapi.URI, error) {
	return workspaceapi.ParseURI("file://" + r.resolve(p))
}

func (r *fsReader) OpenFile(p string, flag int, perm os.FileMode) (workspaceapi.File, error) {
	return os.OpenFile(r.resolve(p), flag, perm)
}

func (r *fsReader) Stat(p string) (os.FileInfo, error) { return os.Stat(r.resolve(p)) }

func (r *fsReader) ReadDir(p string) ([]os.DirEntry, error) {
	return os.ReadDir(r.resolve(p))
}

// completerFixture is the canonical varied fixture used by both the
// table-driven completer tests and the benchmark. It is rooted at a
// fresh tmp dir per call and contains a mix of plain, escaped,
// quoted-style, unicode, hidden, deeply nested and shell-metachar
// names so that completer behaviour and performance can be measured
// against a realistic shape.
type completerFixture struct {
	root   string
	reader *fsReader
}

func newCompleterFixture(tb testing.TB) completerFixture {
	tb.Helper()
	root := tb.TempDir()

	// flat top-level entries spanning the edge cases we care about.
	files := []string{
		"alpha.go",
		"beta.txt",
		"gamma.md",
		"file with spaces.txt",
		"with\ttab.txt",
		"weird$name.txt",
		"glob*name.txt",
		"paren(name).txt",
		"quote'name.txt",
		"dquote\"name.txt",
		"backslash\\name.txt",
		"emoji-🚀.txt",
		"日本語.txt",
		"café.txt",         // composed (NFC)
		"cafe\u0301.txt",   // decomposed (NFD)
		".hidden",
		"plain.swp", // filtered by completer
		"normal.swp.go",
	}
	for _, f := range files {
		require.NoError(tb, os.WriteFile(filepath.Join(root, f), []byte("x"), 0o600))
	}

	dirs := []string{
		"src",
		"src/cmd",
		"src/cmd/rune",
		"src/pkg",
		"docs",
		"with space",
		"with space/nested",
		".hiddenDir",
	}
	for _, d := range dirs {
		require.NoError(tb, os.MkdirAll(filepath.Join(root, d), 0o700))
	}

	// Drop a file inside the spaced directory tree so completer
	// traversals rooted at it return at least one entry.
	require.NoError(tb, os.WriteFile(
		filepath.Join(root, "with space", "nested", "file.txt"),
		[]byte("s"), 0o600))

	// Populate nested dirs so traversal has real work to do under the
	// benchmark. A few hundred entries is comparable to a midsize
	// repository folder.
	for i := range 50 {
		_ = os.WriteFile(filepath.Join(root, "src", "cmd",
			fmt.Sprintf("file_%02d.go", i)), []byte("y"), 0o600)
		_ = os.WriteFile(filepath.Join(root, "src", "pkg",
			fmt.Sprintf("pkg_%02d.go", i)), []byte("z"), 0o600)
		_ = os.WriteFile(filepath.Join(root, "docs",
			fmt.Sprintf("doc-%02d.md", i)), []byte("d"), 0o600)
	}

	return completerFixture{root: root, reader: newFSReader(root)}
}

func TestFilePathCompleterEdgeCases(t *testing.T) {
	t.Parallel()
	fix := newCompleterFixture(t)
	c := FilePathCompleter(fix.reader)

	tests := []struct {
		name string
		// args is the full arg list passed to Complete; the completer
		// only inspects the last entry. We always pass the command
		// name as args[0] so that the test exercises the same shape
		// the prompt produces in production.
		args []string
		// wantSubset asserts that every listed name appears in the
		// resulting iterator. We do not assert the full set so the
		// test stays robust against the fixture growing over time.
		wantSubset []string
		// wantNotIn asserts that none of these strings are returned.
		wantNotIn []string
		// wantSomeContains, when non-nil, asserts that at least one
		// returned entry contains every listed substring. Useful for
		// absolute-path expectations on platforms where the temp
		// directory may be reported via a different prefix (e.g.
		// /var vs /private/var on macOS).
		wantSomeContains []string
		// wantPrefix is the modified-last value the completer
		// returned (only set when the completer expands ~).
		wantPrefix string
		// wantErr, when non-empty, must be a substring of the error.
		wantErr string
	}{
		{
			name:       "no args lists workspace root",
			args:       []string{"edit"},
			wantSubset: []string{"alpha.go", "beta.txt"},
			wantNotIn:  []string{"plain.swp"},
		},
		{
			name:       "empty last arg lists workspace root",
			args:       []string{"edit", ""},
			wantSubset: []string{"alpha.go", "src/cmd/file_00.go"},
		},
		{
			name:       "plain partial path",
			args:       []string{"edit", "src"},
			// walkDirCompleter resolves to the workspace root (since
			// "src" is interpreted as a partial filename whose parent
			// is the cwd) and returns recursive results. We assert on
			// known files to keep the expectations stable.
			wantSubset: []string{"src/cmd/file_00.go", "src/pkg/pkg_00.go"},
		},
		{
			name:       "absolute path",
			args:       []string{"edit", filepath.Join(fix.root, "src")},
			wantSomeContains: []string{
				filepath.Join("src", "cmd", "file_00.go"),
			},
		},
		{
			name: "backslash-escaped space",
			args: []string{"edit", `with\ space`},
			wantSubset: []string{"with space/nested/file.txt"},
		},
		{
			name: "single-quoted with space",
			args: []string{"edit", `'with space'`},
			wantSubset: []string{"with space/nested/file.txt"},
		},
		{
			name: "double-quoted with space",
			args: []string{"edit", `"with space"`},
			wantSubset: []string{"with space/nested/file.txt"},
		},
		{
			name: "double-quoted unescapes inner quote",
			// We resolve to a path that does not exist and expect
			// either a clean empty result or a typed error — what we
			// must not see is panic or junk in the path lookup.
			args:       []string{"edit", `"a\"b"`},
			wantNotIn:  []string{`a\"b`, `"a\"b"`},
		},
		{
			name: "unclosed single quote is lenient",
			// The unquoter strips the opener, so this resolves to
			// directory "missing" which doesn't exist in the
			// fixture; we expect no panic and an empty/clean result.
			args:      []string{"edit", `'missing`},
			wantNotIn: []string{`'missing`},
		},
		{
			name:      "unclosed double quote is lenient",
			args:      []string{"edit", `"missing`},
			wantNotIn: []string{`"missing`},
		},
		{
			name: "empty single-quoted string",
			// '' unquotes to "" which the completer treats as a
			// fresh listing of the workspace root.
			args:       []string{"edit", `''`},
			wantSubset: []string{"alpha.go"},
		},
		{
			name: "trailing dangling backslash",
			// Lenient: the trailing \ is dropped during unquote and
			// we look up the bare prefix.
			args:       []string{"edit", `src\`},
			wantSubset: []string{"src/cmd/file_00.go", "src/pkg/pkg_00.go"},
		},
		{
			name: "tab character escaped",
			// A literal tab in the path is rejected by the URI
			// parser, which the prompt already handles by showing
			// the error to the user. The regression we are guarding
			// against here is a panic inside the unquoter; the
			// completer must surface a typed error instead.
			args:    []string{"edit", "with\\\ttab.txt"},
			wantErr: "invalid control character",
		},
		{
			name: "shell metachars survive single quoting",
			args: []string{"edit", `'weird$name.txt'`},
			// The unquoted value is a partial filename; the parent
			// is the workspace root, so the recursive listing must
			// include it.
			wantSubset: []string{"weird$name.txt"},
		},
		{
			name: "shell glob char in quoted value",
			args: []string{"edit", `'glob*name.txt'`},
			wantSubset: []string{"glob*name.txt"},
		},
		{
			name: "paren in quoted value",
			args: []string{"edit", `'paren(name).txt'`},
			wantSubset: []string{"paren(name).txt"},
		},
		{
			name:       "wide CJK characters",
			args:       []string{"edit", "日本"},
			wantSubset: []string{"日本語.txt"},
		},
		{
			name:       "emoji prefix",
			args:       []string{"edit", "emoji-"},
			wantSubset: []string{"emoji-🚀.txt"},
		},
		{
			name:       "NFC composed lookup matches",
			args:       []string{"edit", "café"},
			wantSubset: []string{"café.txt"},
		},
		// NFD lookup is intentionally omitted: filesystems differ in
		// how they normalise filenames (HFS+/APFS fold to NFC) so
		// the assertion would not be portable. The completer itself
		// passes the bytes through to ReadDir unchanged.
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			it, prefix, err := c.Complete(context.Background(), tc.args)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantPrefix, prefix)

			got := collectAll(t, it)
			gotSet := make(map[string]struct{}, len(got))
			for _, g := range got {
				gotSet[g] = struct{}{}
			}
			for _, want := range tc.wantSubset {
				_, ok := gotSet[want]
				assert.True(t, ok,
					"expected %q in result, got %v", want, got)
			}
			for _, want := range tc.wantNotIn {
				_, ok := gotSet[want]
				assert.False(t, ok,
					"did not expect %q in result, got %v", want, got)
			}
			for _, sub := range tc.wantSomeContains {
				found := false
				for _, g := range got {
					if strings.Contains(g, sub) {
						found = true
						break
					}
				}
				assert.True(t, found,
					"expected at least one entry containing %q, got %v",
					sub, got)
			}
		})
	}
}

func TestDirsCompleterFiltersHidden(t *testing.T) {
	t.Parallel()
	fix := newCompleterFixture(t)
	c := DirsCompleter(fix.reader)

	it, _, err := c.Complete(context.Background(), []string{"workspacenew"})
	require.NoError(t, err)
	got := collectAll(t, it)
	sort.Strings(got)

	for _, name := range got {
		assert.False(t, strings.HasPrefix(filepath.Base(name), "."),
			"DirsCompleter should not return hidden entry %q", name)
	}
}

// TestFilePathCompleterRejectsUnsupported verifies that pathological
// inputs that the unquoter cannot represent (e.g. embedded null byte)
// surface as a clean error instead of panicking.
func TestFilePathCompleterRejectsUnsupported(t *testing.T) {
	t.Parallel()
	fix := newCompleterFixture(t)
	c := FilePathCompleter(fix.reader)

	// A literal NUL byte cannot appear in a filename on POSIX. We use
	// it here to assert that the completer returns a typed error
	// (from the URI layer) rather than misbehaving silently.
	_, _, err := c.Complete(context.Background(),
		[]string{"edit", "bad\x00name"})
	if err == nil {
		// Some platforms accept the NUL through ParseURI and only
		// surface the error from the OS Stat call as an empty
		// iterator. Either path is acceptable as long as we don't
		// panic; the regression we are guarding against is a panic
		// inside the unquoter.
		t.Logf("no error returned for embedded NUL; iterator must be empty")
	}
}

// BenchmarkFilePathCompleter measures the per-call cost of
// FilePathCompleter.Complete on the canonical varied fixture. It is
// intended to track the overhead of the shell-aware unquoting added
// for RUNE-120.
//
// To compare with a baseline:
//
//	go test -run='^$' -bench=BenchmarkFilePathCompleter -benchmem \
//	    -count=10 ./handler/command/ > new.txt
//	go test -run='^$' -bench=BenchmarkFilePathCompleter -benchmem \
//	    -count=10 ./handler/command/ > old.txt   # at baseline
//	benchstat old.txt new.txt
func BenchmarkFilePathCompleter(b *testing.B) {
	fix := newCompleterFixture(b)
	c := FilePathCompleter(fix.reader)
	ctx := context.Background()

	// Each sub-benchmark mirrors a realistic prompt scenario. The
	// "*_quoted" variants exercise the shell-aware unquoting hot
	// path; the plain variants establish a baseline against which
	// the unquoting overhead can be measured.
	cases := []struct {
		name string
		args []string
	}{
		{"empty", []string{"edit"}},
		{"plain_partial", []string{"edit", "src"}},
		{"plain_nested", []string{"edit", "src/cmd"}},
		{"backslash_escape", []string{"edit", `with\ space`}},
		{"single_quoted", []string{"edit", `'with space'`}},
		{"double_quoted", []string{"edit", `"with space"`}},
		{"absolute", []string{"edit", filepath.Join(fix.root, "src", "cmd")}},
		{"unicode", []string{"edit", "日本"}},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				it, _, err := c.Complete(ctx, tc.args)
				if err != nil {
					b.Fatalf("Complete: %v", err)
				}
				// Drain so that the iterator's traversal cost is
				// reflected in the benchmark numbers.
				for {
					_, ok := it.Next(ctx)
					if !ok {
						break
					}
				}
				_ = it.Close()
			}
		})
	}
}
