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
	"unstable.build/go-tui/llm/llamacpp"
)

// TestLocalNilRegistryPanics verifies New refuses to build a Handler
// when the local registry dependency is nil.
func TestLocalNilRegistryPanics(t *testing.T) {
	defer func() {
		assert.NotNil(t, recover(), "expected panic for nil local registry")
	}()
	_ = New(Config{Service: newRouterForTest(t), LocalRegistry: nil, Storage: stubStorageForTest(t)})
}

// TestLocalNoArgsUsage requires a subcommand under local.
func TestLocalNoArgsUsage(t *testing.T) {
	reg := newTestRegistry(t)
	h := newLocalHandler(reg)
	_, err := h.HandleCommand(context.Background(),
		repl.Command{Name: "local", Args: nil}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "usage")
}

// TestLocalUnknownSubcommand rejects unknown subcommands.
func TestLocalUnknownSubcommand(t *testing.T) {
	reg := newTestRegistry(t)
	h := newLocalHandler(reg)
	_, err := h.HandleCommand(context.Background(),
		repl.Command{Name: "local", Args: []string{"nope"}}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown local subcommand")
}

// TestLocalListEmptyCache returns a placeholder when the cache is empty.
func TestLocalListEmptyCache(t *testing.T) {
	reg := newTestRegistry(t)
	h := newLocalHandler(reg)
	it, err := h.HandleCommand(context.Background(),
		repl.Command{Name: "local", Args: []string{"list"}}, nil)
	require.NoError(t, err)
	v, ok := it.Next(context.Background())
	require.True(t, ok)
	require.NotNil(t, v)
}

// TestLocalDeleteUnknownReferenceErrors surfaces a clean ErrNotExist
// when the reference is not in the cache.
func TestLocalDeleteUnknownReferenceErrors(t *testing.T) {
	reg := newTestRegistry(t)
	h := newLocalHandler(reg)
	_, err := h.HandleCommand(context.Background(),
		repl.Command{Name: "local", Args: []string{"delete", "huggingface.co/foo/bar:latest"}},
		nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not present in the local cache")
}

// TestLocalCompleteTopLevelSubs returns the three subcommands at top
// level.
func TestLocalCompleteTopLevelSubs(t *testing.T) {
	reg := newTestRegistry(t)
	h := newLocalHandler(reg)
	it, err := h.Complete(context.Background(), "local", nil)
	require.NoError(t, err)
	names, err := iterator.ToSlice(context.Background(), it)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"list", "download", "delete"}, names)
}

// TestLocalCompleteDeleteEmptyCache returns no completions when the
// cache is empty.
func TestLocalCompleteDeleteEmptyCache(t *testing.T) {
	reg := newTestRegistry(t)
	h := newLocalHandler(reg)
	it, err := h.Complete(context.Background(), "local", []string{"delete", ""})
	require.NoError(t, err)
	names, err := iterator.ToSlice(context.Background(), it)
	require.NoError(t, err)
	assert.Empty(t, names)
}

// newTestRegistry constructs a llamacpp.Registry rooted in a fresh
// temp directory.
func newTestRegistry(t *testing.T) *llamacpp.Registry {
	t.Helper()
	r, err := llamacpp.NewRegistry(t.TempDir())
	require.NoError(t, err)
	return r
}
