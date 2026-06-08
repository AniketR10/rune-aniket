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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/llm/openai"
)

func TestAliasSetAndList(t *testing.T) {
	ctx := context.Background()
	h := newAliasHandler(newRouterForTest(t))

	_, err := h.HandleCommand(ctx,
		repl.Command{Name: "alias", Args: []string{"set", "default", "openai/" + openai.GPT5Dot5}}, nil)
	require.NoError(t, err)

	stored, err := h.router.Aliases(ctx)
	require.NoError(t, err)
	assert.Equal(t, openai.GPT5Dot5, stored["default"].Name)
	assert.Equal(t, "openai", stored["default"].Provider)
}

func TestAliasBareSetForm(t *testing.T) {
	ctx := context.Background()
	h := newAliasHandler(newRouterForTest(t))

	_, err := h.HandleCommand(ctx,
		repl.Command{Name: "alias", Args: []string{"default", "openai/" + openai.GPT5Dot5}}, nil)
	require.NoError(t, err)

	stored, err := h.router.Aliases(ctx)
	require.NoError(t, err)
	assert.Equal(t, openai.GPT5Dot5, stored["default"].Name)
	assert.Equal(t, "openai", stored["default"].Provider)
}

func TestAliasSetRejectsUnknownTarget(t *testing.T) {
	ctx := context.Background()
	h := newAliasHandler(newRouterForTest(t))
	_, err := h.HandleCommand(ctx,
		repl.Command{Name: "alias", Args: []string{"set", "default", "openai/nope"}}, nil)
	require.Error(t, err)
}

func TestAliasRemove(t *testing.T) {
	ctx := context.Background()
	h := newAliasHandler(newRouterForTest(t))
	require.NoError(t, h.router.SetAlias(ctx, "default", "openai/"+openai.GPT5Dot5))

	_, err := h.HandleCommand(ctx,
		repl.Command{Name: "alias", Args: []string{"remove", "default"}}, nil)
	require.NoError(t, err)

	stored, err := h.router.Aliases(ctx)
	require.NoError(t, err)
	assert.NotContains(t, stored, "default")
}

func TestAliasRemoveUnsetErrors(t *testing.T) {
	ctx := context.Background()
	h := newAliasHandler(newRouterForTest(t))
	_, err := h.HandleCommand(ctx,
		repl.Command{Name: "alias", Args: []string{"remove", "default"}}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not set")
}

func TestAliasListShowsReservedNames(t *testing.T) {
	ctx := context.Background()
	h := newAliasHandler(newRouterForTest(t))
	it, err := h.HandleCommand(ctx, repl.Command{Name: "alias", Args: nil}, nil)
	require.NoError(t, err)
	v, ok := it.Next(ctx)
	require.True(t, ok)
	require.NotNil(t, v)
}

func TestAliasCompleteTopLevel(t *testing.T) {
	ctx := context.Background()
	h := newAliasHandler(newRouterForTest(t))
	it, err := h.Complete(ctx, "alias", nil)
	require.NoError(t, err)
	names, err := iterator.ToSlice(ctx, it)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"list", "set", "remove"}, names)
}

func TestAliasCompleteNames(t *testing.T) {
	ctx := context.Background()
	h := newAliasHandler(newRouterForTest(t))
	require.NoError(t, h.router.SetAlias(ctx, "fast", "openai/"+openai.GPT5Dot5))

	it, err := h.Complete(ctx, "alias", []string{"remove", ""})
	require.NoError(t, err)
	names, err := iterator.ToSlice(ctx, it)
	require.NoError(t, err)
	assert.Contains(t, names, "default")
	assert.Contains(t, names, "query")
	assert.Contains(t, names, "compact")
	assert.Contains(t, names, "dream")
	assert.Contains(t, names, "fast")
}

func TestAliasCompleteSetModelTargets(t *testing.T) {
	ctx := context.Background()
	h := newAliasHandler(newRouterForTest(t))

	it, err := h.Complete(ctx, "alias", []string{"set", "default", ""})
	require.NoError(t, err)
	names, err := iterator.ToSlice(ctx, it)
	require.NoError(t, err)
	assert.Contains(t, names, "openai/"+openai.GPT5Dot5)
	for _, n := range names {
		assert.Contains(t, n, "/", "target completions must be provider/model qualified")
	}
}

func TestAliasCompleteSetModelTargetsFiltersByPrefix(t *testing.T) {
	ctx := context.Background()
	h := newAliasHandler(newRouterForTest(t))

	it, err := h.Complete(ctx, "alias", []string{"set", "default", "openai/"})
	require.NoError(t, err)
	names, err := iterator.ToSlice(ctx, it)
	require.NoError(t, err)
	require.NotEmpty(t, names)
	for _, n := range names {
		assert.True(t, strings.HasPrefix(n, "openai/"), "got %q", n)
	}
}

func TestAliasCompleteBareFormModelTargets(t *testing.T) {
	ctx := context.Background()
	h := newAliasHandler(newRouterForTest(t))

	it, err := h.Complete(ctx, "alias", []string{"default", ""})
	require.NoError(t, err)
	names, err := iterator.ToSlice(ctx, it)
	require.NoError(t, err)
	assert.Contains(t, names, "openai/"+openai.GPT5Dot5)
}

// TestAliasDispatchThroughParentHandler verifies the `models alias`
// subtree is reachable through the parent Handler's dispatch and
// completion, guarding the wiring in llmshell.go.
func TestAliasDispatchThroughParentHandler(t *testing.T) {
	ctx := context.Background()
	h := newHandlerForTest(t)

	_, err := h.HandleCommand(ctx,
		repl.Command{Name: "models", Args: []string{"alias", "set", "default", "openai/" + openai.GPT5Dot5}}, nil)
	require.NoError(t, err)

	it, err := h.Complete(ctx, "models", []string{""})
	require.NoError(t, err)
	names, err := iterator.ToSlice(ctx, it)
	require.NoError(t, err)
	assert.Contains(t, names, "alias")
}
