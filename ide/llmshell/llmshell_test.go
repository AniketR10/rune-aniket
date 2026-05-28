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


package llmshell

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/llm"
	"unstable.build/go-tui/llm/llamacpp"
	"unstable.build/go-tui/llm/llmrouter"
)

// newRouterForTest constructs a router with the default llm.Config and a
// fresh temp directory for the local registry.
func newRouterForTest(t *testing.T) *llmrouter.Router {
	t.Helper()
	r, err := llmrouter.New(llm.DefaultConfig(), t.TempDir(), storagestub.NewInMemoryService())
	require.NoError(t, err)
	return r
}

// newRegistryForTest returns a fresh llamacpp.Registry rooted in a temp
// directory.
func newRegistryForTest(t *testing.T) *llamacpp.Registry {
	t.Helper()
	r, err := llamacpp.NewRegistry(t.TempDir())
	require.NoError(t, err)
	return r
}

// stubStorageForTest returns an in-memory storage service.
func stubStorageForTest(t *testing.T) storageapi.Service {
	t.Helper()
	return storagestub.NewInMemoryService()
}

// newHandlerForTest constructs a Handler with stub dependencies.
func newHandlerForTest(t *testing.T) *Handler {
	t.Helper()
	router := newRouterForTest(t)
	return New(Config{
		Service:        router,
		LocalRegistry: router.LocalRegistry(),
		Storage:       stubStorageForTest(t),
	})
}

// TestManualHasRequiredSubcommands verifies the public command tree
// matches what the plan promised: `models providers <codex> <login|status>`
// and `models local <list|download|delete>`.
func TestManualHasRequiredSubcommands(t *testing.T) {
	m := Manual()
	assert.Equal(t, "models", m.Name)

	subs := make(map[string]bool)
	for _, c := range m.Commands {
		subs[c.Name] = true
	}
	assert.True(t, subs["providers"], "missing providers")
	assert.True(t, subs["local"], "missing local")

	var providers, local []string
	for _, c := range m.Commands {
		switch c.Name {
		case "providers":
			for _, sc := range c.Commands {
				if sc.Name == "codex" {
					for _, ss := range sc.Commands {
						providers = append(providers, ss.Name)
					}
				}
			}
		case "local":
			for _, sc := range c.Commands {
				local = append(local, sc.Name)
			}
		}
	}
	assert.ElementsMatch(t, []string{"login", "status"}, providers)
	assert.ElementsMatch(t, []string{"list", "download", "delete"}, local)
}

// TestHandleCommandUnknownSubcommandErrors verifies that the parent
// dispatcher returns an error for unknown subcommands rather than
// silently delegating.
func TestHandleCommandUnknownSubcommandErrors(t *testing.T) {
	h := newHandlerForTest(t)
	_, err := h.HandleCommand(context.Background(),
		repl.Command{Name: "models", Args: []string{"nope"}},
		nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command")
}

// TestHandleCommandNoArgsShowsUsage returns a help block instead of
// an error when called with no arguments.
func TestHandleCommandNoArgsShowsUsage(t *testing.T) {
	h := newHandlerForTest(t)
	it, err := h.HandleCommand(context.Background(),
		repl.Command{Name: "models", Args: nil},
		nil)
	require.NoError(t, err)
	require.NotNil(t, it)
	v, ok := it.Next(context.Background())
	require.True(t, ok)
	require.NotNil(t, v)
}

// TestCompleteTopLevel returns the three top-level subcommands.
func TestCompleteTopLevel(t *testing.T) {
	h := newHandlerForTest(t)
	it, err := h.Complete(context.Background(), "models", nil)
	require.NoError(t, err)
	names, err := iterator.ToSlice(context.Background(), it)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"providers", "local", "help"}, names)
}

// TestCompleteFiltersByPrefix filters when a partial first arg is
// supplied.
func TestCompleteFiltersByPrefix(t *testing.T) {
	h := newHandlerForTest(t)
	it, err := h.Complete(context.Background(), "models", []string{"pr"})
	require.NoError(t, err)
	names, err := iterator.ToSlice(context.Background(), it)
	require.NoError(t, err)
	assert.Equal(t, []string{"providers"}, names)
}

// TestNewPanicsOnNilRouter verifies New refuses to build with a nil
// router.
func TestNewPanicsOnNilRouter(t *testing.T) {
	defer func() {
		assert.NotNil(t, recover(), "expected panic for nil router")
	}()
	_ = New(Config{Service: nil, LocalRegistry: newRegistryForTest(t), Storage: stubStorageForTest(t)})
}
