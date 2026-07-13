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

package llamaserver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
)

func TestServiceNew_PanicsOnNilDeps(t *testing.T) {
	exec := &fakeExecutor{}
	loc := NewFixedLocator("/usr/bin/llama-server")
	notis := &recordingNotifications{}

	assert.Panics(t, func() { New(Config{}, nil, loc, notis) }, "nil exec")
	assert.Panics(t, func() { New(Config{}, exec, nil, notis) }, "nil locator")
	assert.Panics(t, func() { New(Config{}, exec, loc, nil) }, "nil notifications")

	require.NotNil(t, New(Config{}, exec, loc, notis))
}

func TestService_CreateCompletion_ServerNotInstalled(t *testing.T) {
	svc := New(Config{}, &fakeExecutor{}, NewFixedLocator(""), &recordingNotifications{})
	t.Cleanup(func() { _ = svc.Close() })

	_, err := svc.CreateCompletion(context.Background(),
		llmapi.ModelEntry{Name: "local", Provider: LLMProvider}, llmapi.Request{})
	require.Error(t, err)
	assert.Equal(t, installMessage, err.Error())
}

func TestService_CreateCompletion_DelegatesAndReleases(t *testing.T) {
	exec := &fakeExecutor{}
	svc := New(
		Config{StartupTimeout: 5 * time.Second, IdleTimeout: time.Hour},
		exec, NewFixedLocator("/usr/bin/llama-server"), &recordingNotifications{},
	)
	t.Cleanup(func() { _ = svc.Close() })

	entry := llmapi.ModelEntry{
		Name: "local", Provider: LLMProvider, BaseURL: "/models/local.gguf",
		ContextWindow: 8192,
	}
	it, err := svc.CreateCompletion(context.Background(), entry, llmapi.Request{
		Messages: []llmapi.Message{{Role: llmapi.RoleUser, Content: "hi"}},
	})
	require.NoError(t, err)

	// Drain the SSE stream from the stub OpenAI-compatible server.
	ctx := context.Background()
	var sawText bool
	for {
		ev, ok := it.Next(ctx)
		if !ok {
			break
		}
		if ev.Type == llmapi.EventTextDelta && ev.Text != "" {
			sawText = true
		}
	}
	require.NoError(t, it.Err())
	assert.True(t, sawText, "stub server response should surface text")

	// Before Close, the server is still in-flight (idle timer not armed).
	svc.mu.Lock()
	proc := svc.servers[modelFromEntry(entry).key()]
	svc.mu.Unlock()
	require.NotNil(t, proc)
	assert.Equal(t, 1, proc.inFlight(), "server ref held until iterator Close")

	require.NoError(t, it.Close())
	assert.Equal(t, 0, proc.inFlight(), "iterator Close releases the server ref")
}

func TestService_CountTokens_UsesOfflineEstimate(t *testing.T) {
	svc := New(Config{}, &fakeExecutor{}, NewFixedLocator("/usr/bin/llama-server"),
		&recordingNotifications{})
	t.Cleanup(func() { _ = svc.Close() })

	n, err := svc.CountTokens(llmapi.ModelEntry{Name: "local"},
		[]llmapi.Message{{Role: llmapi.RoleUser, Content: "hello world"}})
	require.NoError(t, err)
	assert.Positive(t, n)
}

func TestService_ImplementsLLMAPIService(t *testing.T) {
	var _ llmapi.Service = New(Config{}, &fakeExecutor{},
		NewFixedLocator("/usr/bin/llama-server"), &recordingNotifications{})
}
