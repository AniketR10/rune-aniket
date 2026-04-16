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
	blueauth "github.com/unstablebuild/blue/auth"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	sdkiterator "github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/extension/extensionv2/peerprocess"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/text/texttest"
)

type capturingEditor struct {
	*texttest.TestEditor
	manual  textapi.CommandManual
	handler textapi.REPLHandler
	calls   int
}

func newCapturingEditor() *capturingEditor {
	return &capturingEditor{TestEditor: texttest.NopEditor()}
}

func (e *capturingEditor) RegisterREPLCommand(
	manual textapi.CommandManual, handler textapi.REPLHandler,
) error {
	e.calls++
	e.manual = manual
	e.handler = handler
	return nil
}

func TestAuthorizerRegistersRevokeREPLCommand(t *testing.T) {
	t.Parallel()

	editor := newCapturingEditor()
	_, err := newAuthorizer(nil, storagestub.NewInMemoryService(), editor)
	require.NoError(t, err)

	require.Equal(t, 1, editor.calls)
	assert.Equal(t, authorizerREPLCommand, editor.manual.Name)
	require.Len(t, editor.manual.Commands, 2)
	assert.Equal(t, authorizerREPLCommandList, editor.manual.Commands[0].Name)
	assert.Equal(t, authorizerREPLCommandRevoke, editor.manual.Commands[1].Name)
	assert.NotNil(t, editor.handler)
}

func TestAuthorizerRequiresEditor(t *testing.T) {
	t.Parallel()

	_, err := newAuthorizer(nil, storagestub.NewInMemoryService(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "editor is required")
}

func TestAuthorizerListREPLListsPersistedDecisions(t *testing.T) {
	t.Parallel()

	storage := storagestub.NewInMemoryService()
	ext := testPluginExtension(nil)
	authorizer := newPluginPermissionAuthorizer(nil, storage)
	require.NoError(t, authorizer.setStoredDecision(context.Background(),
		pluginPermissionStorageKey(ext.Path, ext.Args,
			extensionapi.PermissionBrowserWindowManager),
		pluginPermissionIdentity{Path: ext.Path, Args: ext.Args},
		extensionapi.PermissionBrowserWindowManager, pluginPermissionDecisionAllow))
	handler := authorizerREPLHandler{authorizer: authorizer}

	it, err := handler.HandleCommand(context.Background(), repl.Command{
		Name: authorizerREPLCommand,
		Args: []string{authorizerREPLCommandList},
	}, repl.NopProgressWriter())
	require.NoError(t, err)
	items, err := sdkiterator.ToSlice(context.Background(), it)
	require.NoError(t, err)

	require.Len(t, items, 2)
	out := responsiveStrings(t, items)
	assert.Contains(t, out[0], "Plugin permission decisions")
	assert.Contains(t, out[1], "persisted:allow:")
	assert.Contains(t, out[1], "persisted")
	assert.Contains(t, out[1], pluginPermissionDecisionAllow)
	assert.Contains(t, out[1], string(extensionapi.PermissionBrowserWindowManager))
	assert.Contains(t, out[1], ext.Path)
}

func TestAuthorizerListREPLListsTransientOnceDecisions(t *testing.T) {
	t.Parallel()

	prompter := &stubPluginPermissionPrompter{decision: PluginPermissionAllowOnce}
	authorizer := newPluginPermissionAuthorizer(prompter, storagestub.NewInMemoryService())
	ext := testPluginExtension(nil)
	ctx := contextWithPeerProcess(context.Background(), peerprocess.Process{
		PID:  123,
		UID:  501,
		Exe:  "/Users/test/.rune/bin/runectl",
		Argv: []string{"runectl", "lsp", "hover", "iterator"},
	})
	require.NoError(t, authorizer.Authorize(ctx, ext,
		extensionapi.PermissionLSP, "/semantic.Semantic/Hover"))

	it, err := authorizerREPLHandler{authorizer: authorizer}.HandleCommand(
		context.Background(), repl.Command{
			Name: authorizerREPLCommand,
			Args: []string{authorizerREPLCommandList},
		}, repl.NopProgressWriter())
	require.NoError(t, err)
	items, err := sdkiterator.ToSlice(context.Background(), it)
	require.NoError(t, err)

	require.Len(t, items, 2)
	out := responsiveStrings(t, items)
	assert.Contains(t, out[0], "Plugin permission decisions")
	assert.Contains(t, out[1], "transient:allow:")
	assert.Contains(t, out[1], "transient")
	assert.Contains(t, out[1], pluginPermissionDecisionAllow)
	assert.Contains(t, out[1], string(extensionapi.PermissionLSP))
	assert.Contains(t, out[1], "/Users/test/.rune/bin/runectl")
	assert.Contains(t, out[1], "[lsp hover iterator]")
}

func TestAuthorizerListREPLHandlesEmptyDecisions(t *testing.T) {
	t.Parallel()

	handler := authorizerREPLHandler{
		authorizer: newPluginPermissionAuthorizer(nil, storagestub.NewInMemoryService()),
	}
	it, err := handler.HandleCommand(context.Background(), repl.Command{
		Name: authorizerREPLCommand,
		Args: []string{authorizerREPLCommandList},
	}, repl.NopProgressWriter())
	require.NoError(t, err)
	items, err := sdkiterator.ToSlice(context.Background(), it)
	require.NoError(t, err)

	require.Len(t, items, 1)
	assert.Contains(t, responsiveStrings(t, items)[0], "No plugin")
}

func TestAuthorizerREPLCompletesSubcommands(t *testing.T) {
	t.Parallel()

	handler := authorizerREPLHandler{
		authorizer: newPluginPermissionAuthorizer(nil, storagestub.NewInMemoryService()),
	}
	it, err := handler.Complete(context.Background(), authorizerREPLCommand, nil)
	require.NoError(t, err)
	items, err := sdkiterator.ToSlice(context.Background(), it)
	require.NoError(t, err)
	assert.Equal(t, []string{authorizerREPLCommandList, authorizerREPLCommandRevoke}, items)

	it, err = handler.Complete(context.Background(), authorizerREPLCommand, []string{"l"})
	require.NoError(t, err)
	items, err = sdkiterator.ToSlice(context.Background(), it)
	require.NoError(t, err)
	assert.Equal(t, []string{authorizerREPLCommandList}, items)
}

func TestAuthorizerRevokeREPLCompletesPersistedDecisions(t *testing.T) {
	t.Parallel()

	storage := storagestub.NewInMemoryService()
	ext := testPluginExtension(nil)
	authorizer := newPluginPermissionAuthorizer(nil, storage)
	require.NoError(t, authorizer.setStoredDecision(context.Background(),
		pluginPermissionStorageKey(ext.Path, ext.Args,
			extensionapi.PermissionBrowserWindowManager),
		pluginPermissionIdentity{Path: ext.Path, Args: ext.Args},
		extensionapi.PermissionBrowserWindowManager, pluginPermissionDecisionAllow))
	require.NoError(t, storage.Set(context.Background(), "unrelated",
		storedPluginPermissionDecision{Key: "unrelated", Decision: pluginPermissionDecisionAllow}))
	handler := authorizerREPLHandler{authorizer: authorizer}

	it, err := handler.Complete(context.Background(), authorizerREPLCommand,
		[]string{authorizerREPLCommandRevoke, "allow"})
	require.NoError(t, err)
	items, err := sdkiterator.ToSlice(context.Background(), it)
	require.NoError(t, err)

	require.Len(t, items, 1)
	assert.NotContains(t, items[0], " ")
	assert.True(t, strings.HasPrefix(items[0], "persisted:allow:"))
	assert.Contains(t, items[0], string(extensionapi.PermissionBrowserWindowManager))
}

func TestAuthorizerRevokeREPLDeletesPersistedDecisionByID(t *testing.T) {
	t.Parallel()

	storage := storagestub.NewInMemoryService()
	ext := testPluginExtension(nil)
	authorizer := newPluginPermissionAuthorizer(nil, storage)
	key := pluginPermissionStorageKey(ext.Path, ext.Args,
		extensionapi.PermissionBrowserWindowManager)
	require.NoError(t, authorizer.setStoredDecision(context.Background(), key,
		pluginPermissionIdentity{Path: ext.Path, Args: ext.Args},
		extensionapi.PermissionBrowserWindowManager, pluginPermissionDecisionDeny))
	entries, err := authorizer.storedDecisions(context.Background())
	require.NoError(t, err)
	require.Len(t, entries, 1)

	_, err = authorizerREPLHandler{authorizer: authorizer}.HandleCommand(
		context.Background(), repl.Command{
			Name: authorizerREPLCommand,
			Args: []string{authorizerREPLCommandRevoke, entries[0].ID()},
		}, repl.NopProgressWriter())
	require.NoError(t, err)

	var stored storedPluginPermissionDecision
	err = storage.Get(context.Background(), key, &stored)
	assert.ErrorIs(t, err, storageapi.ErrNotFound)
}

func TestAuthorizerRevokeREPLDeletesPersistedDecision(t *testing.T) {
	t.Parallel()

	storage := storagestub.NewInMemoryService()
	ext := testPluginExtension(nil)
	authorizer := newPluginPermissionAuthorizer(nil, storage)
	key := pluginPermissionStorageKey(ext.Path, ext.Args,
		extensionapi.PermissionBrowserWindowManager)
	require.NoError(t, authorizer.setStoredDecision(context.Background(), key,
		pluginPermissionIdentity{Path: ext.Path, Args: ext.Args},
		extensionapi.PermissionBrowserWindowManager, pluginPermissionDecisionDeny))
	entries, err := authorizer.storedDecisions(context.Background())
	require.NoError(t, err)
	require.Len(t, entries, 1)

	handler := authorizerREPLHandler{authorizer: authorizer}
	_, err = handler.HandleCommand(context.Background(), repl.Command{
		Name: authorizerREPLCommand,
		Args: []string{authorizerREPLCommandRevoke, entries[0].Display()},
	}, repl.NopProgressWriter())
	require.NoError(t, err)

	var stored storedPluginPermissionDecision
	err = storage.Get(context.Background(), key, &stored)
	assert.ErrorIs(t, err, storageapi.ErrNotFound)
}

func TestAuthorizerRevokeREPLResurrectsNeverDecision(t *testing.T) {
	t.Parallel()

	storage := storagestub.NewInMemoryService()
	ext := testPluginExtension(nil)
	authorizer := newPluginPermissionAuthorizer(&stubPluginPermissionPrompter{
		decision: PluginPermissionDenyAlways,
	}, storage)
	err := authorizer.Authorize(context.Background(), ext,
		extensionapi.PermissionBrowserWindowManager, testWindowManagerResource)
	require.ErrorIs(t, err, blueauth.ErrForbidden)
	err = authorizer.Authorize(context.Background(), ext,
		extensionapi.PermissionBrowserWindowManager, testWindowManagerResource)
	require.ErrorIs(t, err, blueauth.ErrForbidden)

	entries, err := authorizer.storedDecisions(context.Background())
	require.NoError(t, err)
	require.Len(t, entries, 1)
	_, err = authorizerREPLHandler{authorizer: authorizer}.HandleCommand(
		context.Background(), repl.Command{
			Name: authorizerREPLCommand,
			Args: []string{authorizerREPLCommandRevoke, entries[0].Display()},
		}, repl.NopProgressWriter())
	require.NoError(t, err)

	prompter := &stubPluginPermissionPrompter{decision: PluginPermissionAllowOnce}
	afterRevoke := newPluginPermissionAuthorizer(prompter, storage)
	err = afterRevoke.Authorize(context.Background(), ext,
		extensionapi.PermissionBrowserWindowManager, testWindowManagerResource)
	require.NoError(t, err)
	assert.Equal(t, 1, prompter.calls)
}

func responsiveStrings(t *testing.T, items []component.Responsive) []string {
	t.Helper()
	out := make([]string, 0, len(items))
	for _, item := range items {
		width := 120
		height := item.Height(width)
		if height <= 0 {
			height = 1
		}
		writer := term.NewStringWriter(width, height)
		item.Resize(width, height)
		item.Draw(writer)
		require.NoError(t, writer.Flush())
		out = append(out, writer.String())
	}
	return out
}

var _ text.Editor = (*capturingEditor)(nil)
