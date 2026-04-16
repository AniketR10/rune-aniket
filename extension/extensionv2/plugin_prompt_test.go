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

func TestPluginPermissionPrompter(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		option       string
		close        bool
		wantDecision PluginPermissionDecision
	}{
		{name: "yes", option: pluginPermissionPromptYes,
			wantDecision: PluginPermissionAllowOnce},
		{name: "yes always", option: pluginPermissionPromptYesAlways,
			wantDecision: PluginPermissionAllowAlways},
		{name: "no", option: pluginPermissionPromptNo,
			wantDecision: PluginPermissionDenyOnce},
		{name: "no never", option: pluginPermissionPromptNoNever,
			wantDecision: PluginPermissionDenyAlways},
		{name: "close", option: pluginPermissionPromptYes, close: true,
			wantDecision: PluginPermissionDenyOnce},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			promptOpener := &fakePluginPromptOpener{
				option: tc.option,
				close:  tc.close,
			}
			prompter := newPluginPermissionPrompter(promptOpener, func(fn func()) bool {
				fn()
				return true
			})

			decision, err := prompter.PromptPluginPermission(context.Background(),
				PluginPermissionRequest{
					Path:       "/bin/test",
					Args:       []string{"--flag"},
					Permission: extensionapi.PermissionBrowserWindowManager,
				})
			require.NoError(t, err)
			assert.Equal(t, tc.wantDecision, decision)
			assert.Equal(t, 1, promptOpener.calls)
			assert.Contains(t, promptOpener.message,
				"Program /bin/test with args [--flag] wants to manage the Window Manager.")
		})
	}
}

func TestPluginPermissionPromptMessageWithLauncher(t *testing.T) {
	t.Parallel()

	message := pluginPermissionPromptMessage(PluginPermissionRequest{
		Path:         "/usr/local/bin/trusted-cli",
		Args:         []string{"run"},
		LauncherPath: "/bin/zsh",
		LauncherArgs: []string{"--login"},
		Permission:   extensionapi.PermissionBrowserWindowManager,
	})
	assert.Contains(t, message,
		"Program /usr/local/bin/trusted-cli with args [run] running inside /bin/zsh [--login] wants to manage the Window Manager.")
}

func TestNewPluginPermissionPrompterNilDependencies(t *testing.T) {
	t.Parallel()

	promptOpener := &fakePluginPromptOpener{option: pluginPermissionPromptYes}
	schedule := func(fn func()) bool { return true }

	assert.Nil(t, newPluginPermissionPrompter(nil, schedule))
	assert.Nil(t, newPluginPermissionPrompter(promptOpener, nil))
	assert.NotNil(t, newPluginPermissionPrompter(promptOpener, schedule))
}

func TestPluginPermissionPrompterUnscheduledDeniesOnce(t *testing.T) {
	t.Parallel()

	promptOpener := &fakePluginPromptOpener{option: pluginPermissionPromptYes}
	prompter := newPluginPermissionPrompter(promptOpener, func(fn func()) bool {
		return false
	})

	decision, err := prompter.PromptPluginPermission(context.Background(),
		PluginPermissionRequest{
			Path:       "/bin/test",
			Permission: extensionapi.PermissionBrowserWindowManager,
		})
	require.NoError(t, err)
	assert.Equal(t, PluginPermissionDenyOnce, decision)
	assert.Zero(t, promptOpener.calls)
}

func TestPluginPermissionPrompterContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	promptOpener := &fakePluginPromptOpener{option: "not selected"}
	prompter := newPluginPermissionPrompter(promptOpener, func(fn func()) bool {
		return true
	})

	decision, err := prompter.PromptPluginPermission(ctx, PluginPermissionRequest{
		Path:       "/bin/test",
		Permission: extensionapi.PermissionBrowserWindowManager,
	})
	assert.Equal(t, PluginPermissionDenyOnce, decision)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Zero(t, promptOpener.calls)
}

func TestPluginPromptDecisionUnknownOptionDeniesOnce(t *testing.T) {
	t.Parallel()

	assert.Equal(t, PluginPermissionDenyOnce, pluginPromptDecision("something else"))
}

var _ browser.Window = browsertest.NopWindow()
