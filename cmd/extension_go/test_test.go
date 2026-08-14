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

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/ide/idelsp/lspcmd"
)

func TestIsTestFunc(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want bool
	}{
		{"TestAdd", true},
		{"TestAdd_subtraction", true},
		{"Test", true},
		{"BenchmarkAdd", true},
		{"Benchmark", true},
		{"FuzzParse", true},
		{"Fuzz", true},
		{"ExampleAdd", true},
		{"Example", true},
		{"main", false},
		{"init", false},
		{"helper", false},
		{"testHelper", false},
		{"benchmarkHelper", false},
		{"Testing", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, isTestFunc(tt.name))
		})
	}
}

func TestCompleteTestFuncs(t *testing.T) {
	t.Parallel()

	results := []syntaxapi.Result{
		{Text: "TestAdd"},
		{Text: "main"},
		{Text: "BenchmarkSort"},
		{Text: "init"},
		{Text: "helper"},
		{Text: "FuzzParse"},
		{Text: "ExampleNew"},
	}
	parser := &stubParser{results: results}

	iter, err := completeTestFuncs(context.Background(), parser)
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	var got []string
	for {
		s, ok := iter.Next(context.Background())
		if !ok {
			break
		}
		got = append(got, s)
	}
	require.NoError(t, iter.Err())
	assert.Equal(t, []string{"TestAdd", "BenchmarkSort", "FuzzParse", "ExampleNew"}, got)
}

type stubParser struct {
	syntaxapi.Parser
	results []syntaxapi.Result
}

func (p *stubParser) Search(
	_ string, _ []string, _ ...string,
) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.FromSlice(p.results), nil
}

// capturingExecutor records the workspaceapi.Cmd it is asked to start and
// drives a canned pass stream so the test command completes synchronously
// enough to assert on the spawned command.
type capturingExecutor struct {
	mu     sync.Mutex
	cmd    workspaceapi.Cmd
	stdout string
}

var _ workspaceapi.Executor = (*capturingExecutor)(nil)

func (e *capturingExecutor) Start(
	_ context.Context, cmd workspaceapi.Cmd,
) (workspaceapi.Pid, error) {
	e.mu.Lock()
	e.cmd = cmd
	e.mu.Unlock()

	go func() {
		if cmd.Stdout != nil && e.stdout != "" {
			_, _ = io.Copy(cmd.Stdout, bytes.NewBufferString(e.stdout))
		}
		if cmd.Watcher != nil {
			if ch := cmd.Watcher.WatchProcess(); ch != nil {
				ch <- nil
			}
		}
	}()
	return 1, nil
}

func (e *capturingExecutor) Signal(_ workspaceapi.Pid, _ syscall.Signal) error { return nil }
func (e *capturingExecutor) Close() error                                      { return nil }

func (e *capturingExecutor) captured() workspaceapi.Cmd {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cmd
}

func TestHandlersRequireOpenFile(t *testing.T) {
	t.Parallel()

	mn := &mockNotifications{}
	handlers := map[string]textapi.CommandHandler{
		"codelens":   &codeLensCmd{notify: mn},
		"codeaction": lspcmd.CodeActionHandler(nil, nil, mn, nil, nil, "", false, ""),
		"vulncheck":  &vulncheckCmd{notify: mn},
		"mod":        &modCmd{notify: mn},
		"add-import": &addImportCmd{notify: mn},
	}
	for name, h := range handlers {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := h.HandleCommand(t.Context(), textapi.Command{Name: name})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "no file open")
		})
	}
}

func TestTestCmdHandleCommand(t *testing.T) {
	t.Parallel()

	t.Run("no args and nil resource returns guidance error", func(t *testing.T) {
		t.Parallel()

		mn := &mockNotifications{}
		h := &testCmd{notify: mn}
		err := h.HandleCommand(t.Context(), textapi.Command{Name: "test"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "pass the name of the test to run")
	})

	t.Run("arg with nil resource runs from workspace root", func(t *testing.T) {
		t.Parallel()

		ex := &capturingExecutor{
			stdout: `{"Action":"pass","Package":"example.com/test","Test":"TestAdd","Elapsed":0.01}`,
		}
		mn := &mockNotifications{}
		h := &testCmd{notify: mn, executor: ex}

		err := h.HandleCommand(t.Context(), textapi.Command{
			Name: "test",
			Args: []string{"TestAdd"},
		})
		require.NoError(t, err)

		got := ex.captured()
		assert.Equal(t, "go", got.Path)
		assert.Empty(t, got.Dir)
		assert.Equal(t,
			[]string{"test", "-json", "-count=1", "-run", "^TestAdd$", "./..."},
			got.Args)
	})

	t.Run("arg with open file still searches whole module", func(t *testing.T) {
		t.Parallel()

		ex := &capturingExecutor{
			stdout: `{"Action":"pass","Package":"example.com/test","Test":"TestAdd","Elapsed":0.01}`,
		}
		mn := &mockNotifications{}
		h := &testCmd{notify: mn, executor: ex}

		uri, err := workspaceapi.ParseURI("file:///work/pkg/main_test.go")
		require.NoError(t, err)

		err = h.HandleCommand(t.Context(), textapi.Command{
			Name:     "test",
			Args:     []string{"TestAdd"},
			URI:      uri,
			Resource: &stubResource{uri: uri},
		})
		require.NoError(t, err)

		got := ex.captured()
		// A named test selected from workspace-wide completion may live
		// in any package, so it must run module-wide from the workspace
		// root rather than only the open file's package.
		assert.Empty(t, got.Dir)
		assert.Equal(t,
			[]string{"test", "-json", "-count=1", "-run", "^TestAdd$", "./..."},
			got.Args)
	})
}

func TestProcessTestOutput(t *testing.T) {
	t.Parallel()

	t.Run("all pass", func(t *testing.T) {
		t.Parallel()

		mn := &mockNotifications{}
		h := &testCmd{notify: mn}

		jsonLines := strings.Join([]string{
			`{"Action":"run","Package":"example.com/test","Test":"TestAdd"}`,
			`{"Action":"output","Package":"example.com/test","Test":"TestAdd","Output":"=== RUN   TestAdd\n"}`,
			`{"Action":"run","Package":"example.com/test","Test":"TestAdd/case1"}`,
			`{"Action":"pass","Package":"example.com/test","Test":"TestAdd/case1","Elapsed":0.01}`,
			`{"Action":"pass","Package":"example.com/test","Test":"TestAdd","Elapsed":0.02}`,
			`{"Action":"pass","Package":"example.com/test","Elapsed":0.5}`,
		}, "\n")

		pr, pw := io.Pipe()
		go func() {
			_, _ = pw.Write([]byte(jsonLines))
			_ = pw.Close()
		}()

		exitErrCh := make(chan error, 1)
		exitErrCh <- nil

		var stderr bytes.Buffer
		h.processTestOutput("TestAdd", pr, &stderr, exitErrCh)

		msgs := mn.getMessages()
		require.GreaterOrEqual(t, len(msgs), 2)
		// First is the progress notification.
		assert.Equal(t, browserapi.LevelInfo, msgs[0].Level)
		assert.Contains(t, msgs[0].Message, "Running TestAdd")
		// Last is the success result.
		last := msgs[len(msgs)-1]
		assert.Equal(t, browserapi.LevelSuccess, last.Level)
		assert.Contains(t, last.Message, "passed")
	})

	t.Run("some fail", func(t *testing.T) {
		t.Parallel()

		mn := &mockNotifications{}
		h := &testCmd{notify: mn}

		jsonLines := strings.Join([]string{
			`{"Action":"run","Package":"example.com/test","Test":"TestAdd"}`,
			`{"Action":"fail","Package":"example.com/test","Test":"TestAdd","Elapsed":0.01}`,
			`{"Action":"fail","Package":"example.com/test","Elapsed":0.5}`,
		}, "\n")

		pr, pw := io.Pipe()
		go func() {
			_, _ = pw.Write([]byte(jsonLines))
			_ = pw.Close()
		}()

		exitErrCh := make(chan error, 1)
		exitErrCh <- fmt.Errorf("exit status 1")

		var stderr bytes.Buffer
		h.processTestOutput("TestAdd", pr, &stderr, exitErrCh)

		msgs := mn.getMessages()
		require.GreaterOrEqual(t, len(msgs), 2)
		last := msgs[len(msgs)-1]
		assert.Equal(t, browserapi.LevelError, last.Level)
		assert.Contains(t, last.Message, "failed")
	})

	t.Run("build error", func(t *testing.T) {
		t.Parallel()

		mn := &mockNotifications{}
		h := &testCmd{notify: mn}

		pr, pw := io.Pipe()
		go func() {
			_ = pw.Close()
		}()

		exitErrCh := make(chan error, 1)
		exitErrCh <- fmt.Errorf("exit status 2")

		var stderr bytes.Buffer
		stderr.WriteString("main_test.go:5:2: undefined: Add\n")
		h.processTestOutput("TestAdd", pr, &stderr, exitErrCh)

		msgs := mn.getMessages()
		require.GreaterOrEqual(t, len(msgs), 2)
		last := msgs[len(msgs)-1]
		assert.Equal(t, browserapi.LevelError, last.Level)
		assert.Contains(t, last.Message, "undefined: Add")
	})

	t.Run("no tests found", func(t *testing.T) {
		t.Parallel()

		mn := &mockNotifications{}
		h := &testCmd{notify: mn}

		// go test -json outputs a pass with no Test field when no tests match.
		jsonLines := `{"Action":"pass","Package":"example.com/test","Elapsed":0.01}`

		pr, pw := io.Pipe()
		go func() {
			_, _ = pw.Write([]byte(jsonLines))
			_ = pw.Close()
		}()

		exitErrCh := make(chan error, 1)
		exitErrCh <- nil

		var stderr bytes.Buffer
		h.processTestOutput("TestNonexistent", pr, &stderr, exitErrCh)

		msgs := mn.getMessages()
		require.GreaterOrEqual(t, len(msgs), 2)
		last := msgs[len(msgs)-1]
		// A clean exit with zero matched tests must not be reported as a
		// pass; the named test never ran (e.g. wrong package targeted).
		assert.Equal(t, browserapi.LevelWarn, last.Level)
		assert.Contains(t, last.Message, "no matching test ran")
	})
}
