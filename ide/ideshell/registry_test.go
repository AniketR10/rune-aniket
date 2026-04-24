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


package ideshell

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/term/sh"
)

const testWidth = 200

type mockCmdHandler struct {
	handleFn func(context.Context, repl.Command, repl.ProgressWriter) (
		iterator.Iterator[component.Responsive], error,
	)
	completeFn func(context.Context, string, []string) (
		iterator.Iterator[string], error,
	)
	helpFn func(context.Context, []string) (
		iterator.Iterator[component.Responsive], error,
	)
}

func (m *mockCmdHandler) HandleCommand(
	ctx context.Context, cmd repl.Command, pw repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	if m.handleFn != nil {
		return m.handleFn(ctx, cmd, pw)
	}
	return iterator.FromSlice[component.Responsive](nil), nil
}

func (m *mockCmdHandler) Complete(
	ctx context.Context, cmd string, args []string,
) (iterator.Iterator[string], error) {
	if m.completeFn != nil {
		return m.completeFn(ctx, cmd, args)
	}
	return iterator.Empty[string](), nil
}

func (m *mockCmdHandler) Help(
	ctx context.Context, args []string,
) (iterator.Iterator[component.Responsive], error) {
	if m.helpFn != nil {
		return m.helpFn(ctx, args)
	}
	return toLines("mock help"), nil
}

func collectText(
	t *testing.T,
	iter iterator.Iterator[component.Responsive],
) []string {
	t.Helper()
	ctx := context.Background()
	var lines []string
	for {
		item, ok := iter.Next(ctx)
		if !ok {
			break
		}
		h := item.Height(testWidth)
		if h <= 0 {
			continue
		}
		w := term.NewStringWriter(testWidth, h)
		item.Resize(testWidth, h)
		item.Draw(w)
		_ = w.Flush()
		lines = append(lines, w.String())
	}
	require.NoError(t, iter.Err())
	return lines
}

func TestHandleCommandDispatches(t *testing.T) {
	r := NewRegistry()
	called := false
	r.Register("foo", "do foo", &mockCmdHandler{
		handleFn: func(_ context.Context, cmd repl.Command, _ repl.ProgressWriter) (
			iterator.Iterator[component.Responsive], error,
		) {
			called = true
			assert.Equal(t, "foo", cmd.Name)
			assert.Equal(t, []string{"bar"}, cmd.Args)
			return iterator.FromSlice([]component.Responsive{
				toResponsive("ok"),
			}), nil
		},
	})

	ctx := context.Background()
	iter, err := r.HandleCommand(ctx, repl.Command{
		Name: "foo", Args: []string{"bar"},
	}, repl.NopProgressWriter())
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	out := collectText(t, iter)
	assert.True(t, called)
	assert.Len(t, out, 1)
}

func TestHandleCommandUnknown(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()
	_, err := r.HandleCommand(ctx, repl.Command{Name: "nope"}, repl.NopProgressWriter())
	assert.True(t, errors.Is(err, repl.ErrNotFound))
}

func TestCompleteCommandNames(t *testing.T) {
	r := NewRegistry()
	r.Register("alpha", "a", &mockCmdHandler{})
	r.Register("beta", "b", &mockCmdHandler{})
	r.Register("apex", "a2", &mockCmdHandler{})

	ctx := context.Background()

	// Prefix "a" matches alpha and apex.
	iter, err := r.Complete(ctx, "a", nil)
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()
	got, err := iterator.ToSlice(ctx, iter)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "apex"}, got)

	// Prefix "b" matches beta.
	iter2, err := r.Complete(ctx, "b", nil)
	require.NoError(t, err)
	defer func() { _ = iter2.Close() }()
	got2, err := iterator.ToSlice(ctx, iter2)
	require.NoError(t, err)
	assert.Equal(t, []string{"beta"}, got2)

	// Empty prefix matches all, sorted.
	iter3, err := r.Complete(ctx, "", nil)
	require.NoError(t, err)
	defer func() { _ = iter3.Close() }()
	got3, err := iterator.ToSlice(ctx, iter3)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "apex", "beta"}, got3)
}

func TestCompleteDelegatesToHandler(t *testing.T) {
	r := NewRegistry()
	r.Register("foo", "do foo", &mockCmdHandler{
		completeFn: func(
			_ context.Context, cmd string, args []string,
		) (iterator.Iterator[string], error) {
			assert.Equal(t, "foo", cmd)
			assert.Equal(t, []string{"ba"}, args)
			return iterator.FromSlice([]string{"bar", "baz"}), nil
		},
	})

	ctx := context.Background()
	iter, err := r.Complete(ctx, "foo", []string{"ba"})
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()
	got, err := iterator.ToSlice(ctx, iter)
	require.NoError(t, err)
	assert.Equal(t, []string{"bar", "baz"}, got)
}

func TestCompleteUnknownCommandReturnsEmpty(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()
	iter, err := r.Complete(ctx, "nope", []string{"x"})
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()
	got, err := iterator.ToSlice(ctx, iter)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestHelpListsCommands(t *testing.T) {
	r := NewRegistry()
	r.Register("alpha", "does alpha", &mockCmdHandler{})
	require.NoError(t, r.RegisterREPLCommand(textapi.CommandManual{
		Name:     "beta",
		Synopsis: "[path]",
		Summary:  "does beta",
	}, &mockCmdHandler{}))

	ctx := context.Background()
	iter, err := r.Help(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	out := collectText(t, iter)
	require.Len(t, out, 1)
	assert.Contains(t, out[0], "alpha")
	assert.Contains(t, out[0], "does alpha")
	assert.Contains(t, out[0], "beta [path]")
	assert.Contains(t, out[0], "does beta")
}

func TestHelpDelegatesToHandler(t *testing.T) {
	r := NewRegistry()
	r.Register("foo", "do foo", &mockCmdHandler{
		helpFn: func(_ context.Context, args []string) (
			iterator.Iterator[component.Responsive], error,
		) {
			assert.Equal(t, []string{"sub"}, args)
			return toLines("foo sub help"), nil
		},
	})

	ctx := context.Background()
	iter, err := r.Help(ctx, []string{"foo", "sub"})
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	out := collectText(t, iter)
	require.Len(t, out, 1)
	assert.Contains(t, out[0], "foo sub help")
}

func TestHelpUnknownCommand(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()
	_, err := r.Help(ctx, []string{"nope"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command: nope")
}

func TestNewRegistersHelp(t *testing.T) {
	_, r := New(
		func(func()) bool { return false },
		term.NopInterrupter(),
	)

	ctx := context.Background()

	// "help" should be registered.
	iter, err := r.Complete(ctx, "help", nil)
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()
	got, err := iterator.ToSlice(ctx, iter)
	require.NoError(t, err)
	assert.Equal(t, []string{"help"}, got)
}

func TestHelpCommandOutput(t *testing.T) {
	r := NewRegistry()
	registerBaseCommands(r)
	r.Register("foo", "does foo things", &mockCmdHandler{})

	ctx := context.Background()

	// "help" with no args lists all commands.
	iter, err := r.HandleCommand(ctx, repl.Command{
		Name: "help",
	}, repl.NopProgressWriter())
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()
	out := collectText(t, iter)
	require.Len(t, out, 1)
	assert.Contains(t, out[0], "foo")
	assert.Contains(t, out[0], "help")
}

func TestHelpCommandOutputUsesMarkdownList(t *testing.T) {
	r := NewRegistry()
	registerBaseCommands(r)
	require.NoError(t, r.RegisterREPLCommand(textapi.CommandManual{
		Name:    "status",
		Summary: "show status",
	}, &mockCmdHandler{}))

	ctx := context.Background()
	iter, err := r.HandleCommand(ctx, repl.Command{Name: "help"}, repl.NopProgressWriter())
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	out := collectText(t, iter)
	require.Len(t, out, 1)
	assert.Contains(t, out[0], "• help — Show available commands")
	assert.Contains(t, out[0], "• status — show status")
}

func TestHelpCommandDelegates(t *testing.T) {
	r := NewRegistry()
	registerBaseCommands(r)
	r.Register("foo", "does foo", &mockCmdHandler{
		helpFn: func(_ context.Context, args []string) (
			iterator.Iterator[component.Responsive], error,
		) {
			assert.Empty(t, args)
			return toLines("foo detailed help"), nil
		},
	})

	ctx := context.Background()
	iter, err := r.HandleCommand(ctx, repl.Command{
		Name: "help", Args: []string{"foo"},
	}, repl.NopProgressWriter())
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	out := collectText(t, iter)
	require.Len(t, out, 1)
	assert.Contains(t, out[0], "foo detailed help")
}

func TestHelpCommandComplete(t *testing.T) {
	r := NewRegistry()
	registerBaseCommands(r)
	r.Register("foo", "f", &mockCmdHandler{})
	r.Register("far", "f", &mockCmdHandler{})

	ctx := context.Background()

	// Complete "help f<TAB>" should suggest foo and far.
	iter, err := r.Complete(ctx, "help", []string{"f"})
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()
	got, err := iterator.ToSlice(ctx, iter)
	require.NoError(t, err)
	assert.Equal(t, []string{"far", "foo"}, got)
}

type recordingProgressWriter struct {
	mu      sync.Mutex
	samples []progressSample
}

type progressSample struct {
	progress, total int64
	units           string
}

func (w *recordingProgressWriter) Progress(progress, total int64, units string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.samples = append(w.samples, progressSample{progress, total, units})
}

func (w *recordingProgressWriter) get() []progressSample {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]progressSample(nil), w.samples...)
}

// TestShellHandlerForwardsProgressToRegisteredCommand verifies that a
// repl.ProgressWriter passed to the shell handler reaches a command
// registered via the registry. This mirrors the production flow the
// editor uses when it dispatches `agent download ...` from the companion
// shell REPL: repl.Handler -> sh.commandHandler -> CommandRegistry ->
// registered CommandHandler.
func TestShellHandlerForwardsProgressToRegisteredCommand(t *testing.T) {
	shellHandler, r := New(
		func(func()) bool { return false },
		term.NopInterrupter(),
	)
	t.Cleanup(func() { _ = shellHandler.Close() })

	r.Register("dl", "download", &mockCmdHandler{
		handleFn: func(
			_ context.Context, _ repl.Command, pw repl.ProgressWriter,
		) (iterator.Iterator[component.Responsive], error) {
			require.NotNil(t, pw)
			pw.Progress(0, 0, "B")
			pw.Progress(50, 100, "B")
			pw.Progress(100, 100, "B")
			return iterator.FromSlice[component.Responsive](nil), nil
		},
	})

	// sh wraps the registry; the repl.Handler uses sh as its underlying
	// CommandHandler. Drive a command end-to-end through that path with
	// our recording ProgressWriter.
	shCmd := sh.New(r)
	pw := &recordingProgressWriter{}
	ctx := context.Background()
	iter, err := shCmd.HandleCommand(ctx, repl.Command{Name: "dl"}, pw)
	require.NoError(t, err)
	t.Cleanup(func() { _ = iter.Close() })

	_, err = iterator.ToSlice(ctx, iter)
	require.NoError(t, err)

	samples := pw.get()
	require.NotEmpty(t, samples,
		"expected ProgressWriter to receive updates through sh -> registry")
	last := samples[len(samples)-1]
	assert.Equal(t, int64(100), last.progress)
	assert.Equal(t, int64(100), last.total)
	assert.Equal(t, "B", last.units)
}
