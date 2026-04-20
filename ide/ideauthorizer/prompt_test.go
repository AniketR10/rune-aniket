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

package ideauthorizer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/browser/browsertest"
)

type fakePluginPromptOpener struct {
	option  string
	close   bool
	calls   int
	message string
}

func (f *fakePluginPromptOpener) Prompt(
	message string, options []string,
	bindings []term.KeyComb,
	promptHandler handler.PromptHandler,
) browser.Window {
	f.calls++
	f.message = message
	if f.close {
		if err := promptHandler.OnClose(); err != nil {
			panic(err)
		}
		return browsertest.NopWindow()
	}
	for i, opt := range options {
		if opt == f.option {
			promptHandler.OnSelect(i, opt)
			return browsertest.NopWindow()
		}
	}
	panic("test prompt option was not provided")
}

func TestPermissionPrompter(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		option       string
		close        bool
		wantDecision PermissionDecision
	}{
		{name: "yes", option: PromptOptionYes,
			wantDecision: PermissionAllowOnce},
		{name: "yes all", option: PromptOptionYesAlways,
			wantDecision: PermissionAllowAlways},
		{name: "no", option: PromptOptionNo,
			wantDecision: PermissionDenyOnce},
		{name: "no never", option: PromptOptionNoNever,
			wantDecision: PermissionDenyAlways},
		{name: "close", option: PromptOptionYes, close: true,
			wantDecision: PermissionDenyOnce},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			promptOpener := &fakePluginPromptOpener{
				option: tc.option,
				close:  tc.close,
			}
			prompter := newPermissionPrompter(promptOpener, func(fn func()) bool {
				fn()
				return true
			})

			decision, err := prompter.PromptPermission(context.Background(),
				PermissionRequest{
					Path:       "/bin/test",
					Args:       []string{"--flag"},
					Permission: extensionapi.PermissionBrowserWindowManager,
				})
			require.NoError(t, err)
			assert.Equal(t, tc.wantDecision, decision)
			assert.Equal(t, 1, promptOpener.calls)
			assert.Contains(t, promptOpener.message,
				"Program /bin/test with args [--flag] wants to **manage the Window Manager**.")
		})
	}
}

func TestPluginPermissionPromptMessageWithLauncher(t *testing.T) {
	t.Parallel()

	message := pluginPermissionPromptMessage(PermissionRequest{
		Path:         "/usr/local/bin/trusted-cli",
		Args:         []string{"run"},
		LauncherPath: "/bin/zsh",
		LauncherArgs: []string{"--login"},
		Permission:   extensionapi.PermissionBrowserWindowManager,
	})
	assert.Contains(t, message,
		"Program /usr/local/bin/trusted-cli with args [run] running inside /bin/zsh [--login] wants to **manage the Window Manager**.")
}

func TestPluginPermissionPromptMessageWithCommand(t *testing.T) {
	t.Parallel()

	message := pluginPermissionPromptMessage(PermissionRequest{
		Path:              "/usr/local/bin/plugin-cli",
		Args:              []string{"run"},
		Permission:        extensionapi.PermissionExecute,
		CommandPath:       "/bin/grep",
		CommandArgs:       []string{"foo", "/tmp/workspace"},
		CommandDir:        "/tmp/workspace",
		CommandScopeLabel: "/bin/grep *",
	})
	assert.Contains(t, message, "Program **/usr/local/bin/plugin-cli** with args **[run]**")
	assert.Contains(t, message, "wants to run **/bin/grep**")
	assert.Contains(t, message, "with args **[foo /tmp/workspace]**")
	assert.Contains(t, message, "in **/tmp/workspace**")
	assert.Contains(t, message, "Choosing **Yes, All** approves **/bin/grep ***")
}

func TestPluginPermissionPromptMessageWithLauncherAndCommand(t *testing.T) {
	t.Parallel()

	message := pluginPermissionPromptMessage(PermissionRequest{
		Path:              "/usr/local/bin/plugin-cli",
		Args:              []string{"run"},
		LauncherPath:      "/bin/zsh",
		LauncherArgs:      []string{"--login"},
		Permission:        extensionapi.PermissionExecute,
		CommandPath:       "/bin/rm",
		CommandArgs:       []string{"-rf", "foo"},
		CommandScopeLabel: "/bin/rm *",
	})
	assert.Contains(t, message,
		"Program **/usr/local/bin/plugin-cli** with args **[run]** running inside **/bin/zsh** **[--login]**")
	assert.Contains(t, message, "wants to run **/bin/rm**")
	assert.Contains(t, message, "with args **[-rf foo]**")
	assert.Contains(t, message, "Choosing **Yes, All** approves **/bin/rm ***")
}

func TestPermissionPromptMessageHighlightsExtensionCommand(t *testing.T) {
	t.Parallel()

	message := pluginPermissionPromptMessage(PermissionRequest{
		ExtensionID:       "test-extension",
		ExtensionName:     "Test Extension",
		DeveloperID:       "dev-id",
		Permission:        extensionapi.PermissionExecute,
		CommandPath:       "/bin/grep",
		CommandArgs:       []string{"foo"},
		CommandDir:        "/tmp/workspace",
		CommandScopeLabel: "/bin/grep *",
	})
	assert.Contains(t, message,
		"Extension Test Extension by dev-id wants to run **/bin/grep** with args **[foo]** in **/tmp/workspace**.")
	assert.Contains(t, message,
		"Choosing **Yes, All** approves **/bin/grep ***")
}

func TestPluginPermissionPromptMessageWithCommandHighlightsApprovalScope(t *testing.T) {
	t.Parallel()

	message := pluginPermissionPromptMessage(PermissionRequest{
		Path:              "/usr/local/bin/plugin-cli",
		Args:              []string{"run"},
		Permission:        extensionapi.PermissionExecute,
		CommandPath:       "/bin/grep",
		CommandArgs:       []string{"foo", "file.txt"},
		CommandDir:        "/tmp/workspace",
		CommandScopeLabel: "/bin/grep *",
	})

	assert.Contains(t, message, "run **/bin/grep** with args **[foo file.txt]**")
	assert.Contains(t, message, "Choosing **Yes, All** approves **/bin/grep ***")
	assert.Contains(t, message, "future runs with any args")
}

func TestPluginPermissionPromptMessageWithShellWrappedCommandUsesInnerScope(t *testing.T) {
	t.Parallel()

	message := pluginPermissionPromptMessage(PermissionRequest{
		Path:              "/usr/local/bin/plugin-cli",
		Args:              []string{"run"},
		Permission:        extensionapi.PermissionExecute,
		CommandPath:       "/bin/bash",
		CommandArgs:       []string{"-c", "grep foo file.txt"},
		CommandScopeLabel: "grep *",
	})

	assert.Contains(t, message, "run **/bin/bash** with args **[-c grep foo file.txt]**")
	assert.Contains(t, message, "Choosing **Yes, All** approves **grep ***")
}

func TestPluginPermissionPromptMessageWithUnknownShellScriptUsesExactScopeLabel(t *testing.T) {
	t.Parallel()

	message := pluginPermissionPromptMessage(PermissionRequest{
		Path:              "/usr/local/bin/plugin-cli",
		Args:              []string{"run"},
		Permission:        extensionapi.PermissionExecute,
		CommandPath:       "/bin/bash",
		CommandArgs:       []string{"-c", "echo foo; grep bar file.txt"},
		CommandScopeLabel: "/bin/bash -c echo foo; grep bar file.txt @ /tmp",
		CommandDir:        "/tmp",
	})

	assert.Contains(t, message, "run **/bin/bash** with args **[-c echo foo; grep bar file.txt]** in **/tmp**")
	assert.Contains(t, message,
		"Choosing **Yes, All** approves **/bin/bash -c echo foo; grep bar file.txt @ /tmp**")
	assert.NotContains(t, message, "approves **bash ***")
}

func TestPluginPermissionPromptMessageWithMultipleScopeLabels(t *testing.T) {
	t.Parallel()

	message := pluginPermissionPromptMessage(PermissionRequest{
		Path:               "/usr/local/bin/plugin-cli",
		Args:               []string{"run"},
		Permission:         extensionapi.PermissionExecute,
		CommandPath:        "/bin/bash",
		CommandArgs:        []string{"-c", "grep foo && make test && rm file"},
		CommandScopeLabels: []string{"grep *", "make *", "rm *"},
		CommandScopeLabel:  "grep *, make *, rm *",
	})

	assert.Contains(t, message, "run **/bin/bash**")
	assert.Contains(t, message,
		"Choosing **Yes, All** approves **grep ***, **make ***, **rm *** for future runs")
}

func TestPermissionPromptMessageHighlightsExtensionIntent(t *testing.T) {
	t.Parallel()

	message := pluginPermissionPromptMessage(PermissionRequest{
		ExtensionID:   "test-extension",
		ExtensionName: "Test Extension",
		DeveloperID:   "dev-id",
		Permission:    extensionapi.PermissionLSP,
	})
	assert.Equal(t,
		"Extension Test Extension by dev-id wants to **communicate with LSP servers**.",
		message)
}

func TestNewPermissionPrompterNilDependencies(t *testing.T) {
	t.Parallel()

	promptOpener := &fakePluginPromptOpener{option: PromptOptionYes}
	schedule := func(fn func()) bool { return true }

	assert.Nil(t, newPermissionPrompter(nil, schedule))
	assert.Nil(t, newPermissionPrompter(promptOpener, nil))
	assert.NotNil(t, newPermissionPrompter(promptOpener, schedule))
}

func TestPermissionPrompterUnscheduledDeniesOnce(t *testing.T) {
	t.Parallel()

	promptOpener := &fakePluginPromptOpener{option: PromptOptionYes}
	prompter := newPermissionPrompter(promptOpener, func(fn func()) bool {
		return false
	})

	decision, err := prompter.PromptPermission(context.Background(),
		PermissionRequest{
			Path:       "/bin/test",
			Permission: extensionapi.PermissionBrowserWindowManager,
		})
	require.NoError(t, err)
	assert.Equal(t, PermissionDenyOnce, decision)
	assert.Zero(t, promptOpener.calls)
}

func TestPermissionPrompterContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	promptOpener := &fakePluginPromptOpener{option: "not selected"}
	prompter := newPermissionPrompter(promptOpener, func(fn func()) bool {
		return true
	})

	decision, err := prompter.PromptPermission(ctx, PermissionRequest{
		Path:       "/bin/test",
		Permission: extensionapi.PermissionBrowserWindowManager,
	})
	assert.Equal(t, PermissionDenyOnce, decision)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Zero(t, promptOpener.calls)
}

func TestPluginPromptDecisionUnknownOptionDeniesOnce(t *testing.T) {
	t.Parallel()

	assert.Equal(t, PermissionDenyOnce, pluginPromptDecision("something else"))
}

var _ browser.Window = browsertest.NopWindow()
