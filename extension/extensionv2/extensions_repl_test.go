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

package extensionv2

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/auth"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	sdkiterator "github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/component/markdown"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/text/texttest"
)

type extensionsCapturingEditor struct {
	*texttest.TestEditor
	manual  textapi.CommandManual
	handler textapi.REPLHandler
	calls   int
}

func newExtensionsCapturingEditor() *extensionsCapturingEditor {
	return &extensionsCapturingEditor{TestEditor: texttest.NopEditor()}
}

func (e *extensionsCapturingEditor) RegisterREPLCommand(
	manual textapi.CommandManual, handler textapi.REPLHandler,
) error {
	e.calls++
	e.manual = manual
	e.handler = handler
	return nil
}

var _ text.Editor = (*extensionsCapturingEditor)(nil)

func TestRegisterExtensionsREPLCommand(t *testing.T) {
	t.Parallel()

	runner := newTestWorkspaceRunner(t)
	editor := newExtensionsCapturingEditor()

	require.NoError(t, registerExtensionsREPLCommand(runner, editor))
	require.Equal(t, 1, editor.calls)
	assert.Equal(t, extensionsREPLCommand, editor.manual.Name)
	require.Len(t, editor.manual.Commands, 5)
	assert.Equal(t, extensionsREPLCommandStatus, editor.manual.Commands[0].Name)
	assert.Equal(t, extensionsREPLCommandInfo, editor.manual.Commands[1].Name)
	assert.Equal(t, extensionsREPLCommandStart, editor.manual.Commands[2].Name)
	assert.Equal(t, extensionsREPLCommandStop, editor.manual.Commands[3].Name)
	assert.Equal(t, extensionsREPLCommandRestart, editor.manual.Commands[4].Name)
	assert.NotNil(t, editor.handler)
}

func TestExtensionsREPLStatusOutput(t *testing.T) {
	t.Parallel()

	runner := newTestWorkspaceRunner(t)
	require.NoError(t, runner.Run("alpha", "/bin/ext --serve", config.NopConfig()))
	runner.stopExtension("alpha", assert.AnError)
	require.NoError(t, runner.Run("beta", "/bin/beta --watch", config.NopConfig()))
	handler := extensionsREPLHandler{runner: runner}

	it, err := handler.HandleCommand(context.Background(), repl.Command{
		Name: extensionsREPLCommand,
		Args: []string{extensionsREPLCommandStatus},
	}, repl.NopProgressWriter())
	require.NoError(t, err)
	items, err := sdkiterator.ToSlice(context.Background(), it)
	require.NoError(t, err)

	require.Len(t, items, 1)
	md, ok := items[0].(*markdown.Component)
	require.True(t, ok, "expected *markdown.Component, got %T", items[0])
	require.NotNil(t, md)
	// Render wide enough that no cells wrap, so we can assert full values.
	out := extensionsResponsiveStringsWithWidth(t, items, 320)
	rendered := strings.Join(out, "\n")
	// Drop cell wrapping whitespace inserted by the markdown table renderer so
	// assertions target the logical values rather than visual layout.
	flattened := strings.Join(strings.Fields(rendered), " ")
	for _, want := range []string{
		"ID", "Status", "PID", "Starts",
		"alpha", "Errored",
		"beta", "Running",
	} {
		assert.Contains(t, flattened, want)
	}
	assert.NotContains(t, flattened, "Last error")
	assert.NotContains(t, flattened, "Command")
	assert.NotContains(t, flattened, "/bin/ext --serve")
	assert.NotContains(t, flattened, "/bin/beta --watch")
}

func TestExtensionsREPLInfoOutput(t *testing.T) {
	t.Parallel()

	runner := newTestWorkspaceRunner(t)
	require.NoError(t, runner.Run("alpha", "/bin/ext --serve", config.MapConfig(map[string]any{"foo": "bar"})))
	handler := extensionsREPLHandler{runner: runner}

	it, err := handler.HandleCommand(context.Background(), repl.Command{
		Name: extensionsREPLCommand,
		Args: []string{extensionsREPLCommandInfo, "alpha"},
	}, repl.NopProgressWriter())
	require.NoError(t, err)
	items, err := sdkiterator.ToSlice(context.Background(), it)
	require.NoError(t, err)
	require.Len(t, items, 1)
	_, ok := items[0].(*markdown.Component)
	require.True(t, ok, "expected *markdown.Component, got %T", items[0])

	out := extensionsResponsiveStringsWithWidth(t, items, 320)
	flattened := strings.Join(strings.Fields(strings.Join(out, "\n")), " ")
	for _, want := range []string{
		"alpha",
		"Running",
		"/bin/ext --serve",
		"foo",
		"bar",
		"Starts",
		"PID",
		"Uptime",
	} {
		assert.Contains(t, flattened, want)
	}
}

func TestExtensionsREPLStartCommand(t *testing.T) {
	t.Parallel()

	exec := &protocolDrivingExecutor{extensionID: "alpha"}
	runner := newTestWorkspaceRunnerWithExecutor(t, exec)
	handler := extensionsREPLHandler{runner: runner}

	it, err := handler.HandleCommand(context.Background(), repl.Command{
		Name: extensionsREPLCommand,
		Args: []string{extensionsREPLCommandStart, "alpha", "/bin/ext", "--serve"},
	}, repl.NopProgressWriter())
	require.NoError(t, err)
	items, err := sdkiterator.ToSlice(context.Background(), it)
	require.NoError(t, err)

	out := extensionsResponsiveStrings(t, items)
	assert.Equal(t, []string{"Started extension alpha"}, out)
	cmd := exec.snapshotCmd()
	assert.Equal(t, "/bin/ext", cmd.Path)
	assert.Equal(t, []string{"--serve"}, cmd.Args)
}

func TestExtensionsREPLStartCommandWithConfig(t *testing.T) {
	t.Parallel()

	exec := &protocolDrivingExecutor{extensionID: "alpha"}
	runner := newTestWorkspaceRunnerWithExecutor(t, exec)
	handler := extensionsREPLHandler{runner: runner}

	_, err := handler.HandleCommand(context.Background(), repl.Command{
		Name: extensionsREPLCommand,
		Args: []string{extensionsREPLCommandStart, "alpha", "/bin/ext", "--config", `{"foo":"bar"}`},
	}, repl.NopProgressWriter())
	require.NoError(t, err)

	states := runner.listExtensions()
	require.Len(t, states, 1)
	value, err := states[0].Config.GetString("foo")
	require.NoError(t, err)
	assert.Equal(t, "bar", value)
}

func TestExtensionsREPLStopCommand(t *testing.T) {
	t.Parallel()

	runner := newTestWorkspaceRunner(t)
	require.NoError(t, runner.Run("alpha", "/bin/ext", config.NopConfig()))
	handler := extensionsREPLHandler{runner: runner}

	it, err := handler.HandleCommand(context.Background(), repl.Command{
		Name: extensionsREPLCommand,
		Args: []string{extensionsREPLCommandStop, "alpha"},
	}, repl.NopProgressWriter())
	require.NoError(t, err)
	items, err := sdkiterator.ToSlice(context.Background(), it)
	require.NoError(t, err)

	assert.Equal(t, []string{"Stopped extension alpha"}, extensionsResponsiveStrings(t, items))
	states := runner.listExtensions()
	require.Len(t, states, 1)
	assert.False(t, states[0].Running)
}

func TestExtensionsREPLRestartCommandReusesStoredCommandAndConfig(t *testing.T) {
	t.Parallel()

	exec := &protocolDrivingExecutor{extensionID: "alpha"}
	runner := newTestWorkspaceRunnerWithExecutor(t, exec)
	require.NoError(t, runner.Run("alpha", "/bin/ext --serve", config.MapConfig(map[string]any{"foo": "bar"})))
	handler := extensionsREPLHandler{runner: runner}

	it, err := handler.HandleCommand(context.Background(), repl.Command{
		Name: extensionsREPLCommand,
		Args: []string{extensionsREPLCommandRestart, "alpha"},
	}, repl.NopProgressWriter())
	require.NoError(t, err)
	items, err := sdkiterator.ToSlice(context.Background(), it)
	require.NoError(t, err)

	assert.Equal(t, []string{"Restarted extension alpha"}, extensionsResponsiveStrings(t, items))
	states := runner.listExtensions()
	require.Len(t, states, 1)
	assert.Equal(t, 2, states[0].StartCount)
	assert.Equal(t, workspaceapi.Pid(2), states[0].Pid)
	value, err := states[0].Config.GetString("foo")
	require.NoError(t, err)
	assert.Equal(t, "bar", value)
}

func TestExtensionsREPLCompleteSubcommandsAndIDs(t *testing.T) {
	t.Parallel()

	runner := newTestWorkspaceRunner(t)
	require.NoError(t, runner.Run("alpha", "/bin/ext", config.NopConfig()))
	handler := extensionsREPLHandler{runner: runner}

	it, err := handler.Complete(context.Background(), extensionsREPLCommand, nil)
	require.NoError(t, err)
	items, err := sdkiterator.ToSlice(context.Background(), it)
	require.NoError(t, err)
	assert.Equal(t, []string{
		extensionsREPLCommandStatus,
		extensionsREPLCommandInfo,
		extensionsREPLCommandStart,
		extensionsREPLCommandStop,
		extensionsREPLCommandRestart,
	}, items)

	it, err = handler.Complete(context.Background(), extensionsREPLCommand, []string{"st"})
	require.NoError(t, err)
	items, err = sdkiterator.ToSlice(context.Background(), it)
	require.NoError(t, err)
	assert.Equal(t, []string{extensionsREPLCommandStatus, extensionsREPLCommandStart, extensionsREPLCommandStop}, items)

	it, err = handler.Complete(context.Background(), extensionsREPLCommand, []string{extensionsREPLCommandStop, "al"})
	require.NoError(t, err)
	items, err = sdkiterator.ToSlice(context.Background(), it)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha"}, items)

	it, err = handler.Complete(context.Background(), extensionsREPLCommand, []string{extensionsREPLCommandInfo, "al"})
	require.NoError(t, err)
	items, err = sdkiterator.ToSlice(context.Background(), it)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha"}, items)
}

func TestExtensionsREPLErrors(t *testing.T) {
	t.Parallel()

	handler := extensionsREPLHandler{runner: newTestWorkspaceRunner(t)}

	_, err := handler.HandleCommand(context.Background(), repl.Command{
		Name: extensionsREPLCommand,
		Args: []string{"wat"},
	}, repl.NopProgressWriter())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown extensions subcommand "wat"`)

	_, err = handler.HandleCommand(context.Background(), repl.Command{
		Name: extensionsREPLCommand,
		Args: []string{extensionsREPLCommandStart, "alpha"},
	}, repl.NopProgressWriter())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "usage: extensions start")

	_, err = handler.HandleCommand(context.Background(), repl.Command{
		Name: extensionsREPLCommand,
		Args: []string{extensionsREPLCommandInfo},
	}, repl.NopProgressWriter())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "usage: extensions info <id>")

	_, err = handler.HandleCommand(context.Background(), repl.Command{
		Name: extensionsREPLCommand,
		Args: []string{extensionsREPLCommandStop, "missing"},
	}, repl.NopProgressWriter())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `extension "missing" not found`)

	_, err = handler.HandleCommand(context.Background(), repl.Command{
		Name: extensionsREPLCommand,
		Args: []string{extensionsREPLCommandRestart, "missing"},
	}, repl.NopProgressWriter())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `extension "missing" not found`)

	_, err = handler.HandleCommand(context.Background(), repl.Command{
		Name: extensionsREPLCommand,
		Args: []string{extensionsREPLCommandInfo, "missing"},
	}, repl.NopProgressWriter())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `extension "missing" not found`)

	_, err = handler.HandleCommand(context.Background(), repl.Command{
		Name: extensionsREPLCommand,
		Args: []string{extensionsREPLCommandStart, "alpha", "/bin/ext", "--config", "{"},
	}, repl.NopProgressWriter())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse config json")
}

func newTestWorkspaceRunner(t *testing.T) *workspaceRunner {
	t.Helper()
	return newTestWorkspaceRunnerWithExecutor(t, &recordingExecutor{})
}

func newTestWorkspaceRunnerWithExecutor(t *testing.T, exec schemeapi.Executor) *workspaceRunner {
	t.Helper()
	keys, err := auth.GenerateKeys()
	require.NoError(t, err)
	uri, err := workspaceapi.ParseURI("file:///tmp")
	require.NoError(t, err)
	return newWorkspaceRunner(
		exec, exec, extension.GrantAll(), uri,
		"/tmp/ext.sock", "/tmp/ext-data", []byte("cert"), keys,
	)
}

func extensionsResponsiveStrings(t *testing.T, items []component.Responsive) []string {
	t.Helper()
	return extensionsResponsiveStringsWithWidth(t, items, 120)
}

func extensionsResponsiveStringsWithWidth(
	t *testing.T, items []component.Responsive, width int,
) []string {
	t.Helper()
	out := make([]string, 0, len(items))
	for _, item := range items {
		height := item.Height(width)
		if height <= 0 {
			height = 1
		}
		writer := term.NewStringWriter(width, height)
		item.Resize(width, height)
		item.Draw(writer)
		require.NoError(t, writer.Flush())
		out = append(out, strings.TrimRight(writer.String(), " \n"))
	}
	return out
}
