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
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
)

// TestProvidersNilDependencyPanics verifies New refuses to build a
// handler against a nil dependency.
func TestProvidersNilDependencyPanics(t *testing.T) {
	defer func() {
		assert.NotNil(t, recover(), "expected panic for nil storage")
	}()
	_ = New(Config{Router: newRouterForTest(t), LocalRegistry: newRegistryForTest(t), Storage: nil})
}

// TestProvidersNoArgsUsage requires at least one arg under providers.
func TestProvidersNoArgsUsage(t *testing.T) {
	h := newProvidersHandler(storagestub.NewInMemoryService())
	_, err := h.HandleCommand(context.Background(),
		repl.Command{Name: "providers", Args: nil}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "usage")
}

// TestProvidersUnknownProvider rejects providers other than codex.
func TestProvidersUnknownProvider(t *testing.T) {
	h := newProvidersHandler(storagestub.NewInMemoryService())
	_, err := h.HandleCommand(context.Background(),
		repl.Command{Name: "providers", Args: []string{"openai", "login"}},
		nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider")
}

// TestProvidersCodexNoArgsUsage requires codex sub-arg.
func TestProvidersCodexNoArgsUsage(t *testing.T) {
	h := newProvidersHandler(storagestub.NewInMemoryService())
	_, err := h.HandleCommand(context.Background(),
		repl.Command{Name: "providers", Args: []string{"codex"}},
		nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "usage")
}

// TestProvidersCompleteCodex returns codex on first arg.
func TestProvidersCompleteCodex(t *testing.T) {
	h := newProvidersHandler(storagestub.NewInMemoryService())
	it, err := h.Complete(context.Background(), "providers", nil)
	require.NoError(t, err)
	names, err := iterator.ToSlice(context.Background(), it)
	require.NoError(t, err)
	assert.Equal(t, []string{"codex"}, names)
}

// TestProvidersCompleteCodexSubs returns login/status on second arg.
func TestProvidersCompleteCodexSubs(t *testing.T) {
	h := newProvidersHandler(storagestub.NewInMemoryService())
	it, err := h.Complete(context.Background(), "providers", []string{"codex", ""})
	require.NoError(t, err)
	names, err := iterator.ToSlice(context.Background(), it)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"login", "status"}, names)
}
