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

package texttest_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/ide/syntax"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/text/texttest"
	"unstable.build/go-tui/workspace"
)

// TestSyntaxSizeGuardSkipsTreeForLargeBuffers reproduces the freeze
// case: opening a multi-MB file (e.g. ~/.runedev/debug.log at 8 GB)
// blocked the host event loop for seconds inside
// tree_sitter.Parser.ParseWithOptions during tab open. The fix at
// text.Component skips syntax-tree installation entirely when the
// buffer exceeds Config.MaxSyntaxParseSize; this test asserts the
// buffer's view is NOT a *syntax.Tree for a file above the limit,
// while a file below the limit still installs the tree.
func TestSyntaxSizeGuardSkipsTreeForLargeBuffers(t *testing.T) {
	dir := t.TempDir()
	wsURI, err := workspaceapi.ParseURI("file://" + dir)
	require.NoError(t, err)

	scheme, err := workspace.NewFileScheme(
		context.Background(), config.NopConfig(), wsURI)
	require.NoError(t, err)
	t.Cleanup(func() { _ = scheme.Close() })

	ws := workspace.NewSchemeWorkspace(wsURI, scheme, inlineSchedule)

	const threshold = 1024 // 1 KiB
	cfg := text.DefaultConfig()
	cfg.ScheduleNextTick = inlineSchedule
	cfg.MaxSyntaxParseSize = threshold
	c, err := text.NewComponent(texttest.NopEditor(), ws, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })

	// Small file (well below threshold) — tree should be installed.
	smallPath := filepath.Join(dir, "small.txt")
	require.NoError(t, os.WriteFile(smallPath, []byte("hello\n"), 0o644))
	smallURI, err := workspaceapi.ParseURI("file://" + smallPath)
	require.NoError(t, err)

	_, err = c.OpenFileTab(smallURI, false)
	require.NoError(t, err)
	smallEd, err := c.Editor(smallURI)
	require.NoError(t, err)
	_, smallHasTree := smallEd.CellView().(*syntax.Tree)
	assert.True(t, smallHasTree,
		"buffers below MaxSyntaxParseSize must have a syntax tree installed")

	// Large file (well above threshold) — tree must be skipped.
	largePath := filepath.Join(dir, "large.txt")
	require.NoError(t, os.WriteFile(
		largePath, []byte(strings.Repeat("x", threshold*4)+"\n"), 0o644))
	largeURI, err := workspaceapi.ParseURI("file://" + largePath)
	require.NoError(t, err)

	_, err = c.OpenFileTab(largeURI, false)
	require.NoError(t, err)
	largeEd, err := c.Editor(largeURI)
	require.NoError(t, err)
	_, largeHasTree := largeEd.CellView().(*syntax.Tree)
	assert.False(t, largeHasTree,
		"buffers above MaxSyntaxParseSize must NOT have a syntax tree installed")
}

// TestSyntaxSizeGuardZeroDisablesGuard documents the contract that
// Config.MaxSyntaxParseSize == 0 means no limit: even huge buffers
// follow the regular code path and install a syntax tree. This is
// what tests that do not opt in observe (DefaultConfig sets a real
// limit; tests that override MaxSyntaxParseSize to 0 keep the
// historical "always install" behaviour).
func TestSyntaxSizeGuardZeroDisablesGuard(t *testing.T) {
	dir := t.TempDir()
	wsURI, err := workspaceapi.ParseURI("file://" + dir)
	require.NoError(t, err)

	scheme, err := workspace.NewFileScheme(
		context.Background(), config.NopConfig(), wsURI)
	require.NoError(t, err)
	t.Cleanup(func() { _ = scheme.Close() })

	ws := workspace.NewSchemeWorkspace(wsURI, scheme, inlineSchedule)

	cfg := text.DefaultConfig()
	cfg.ScheduleNextTick = inlineSchedule
	cfg.MaxSyntaxParseSize = 0
	c, err := text.NewComponent(texttest.NopEditor(), ws, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })

	fpath := filepath.Join(dir, "huge.txt")
	require.NoError(t, os.WriteFile(
		fpath, []byte(strings.Repeat("y", 64*1024)+"\n"), 0o644))
	fileURI, err := workspaceapi.ParseURI("file://" + fpath)
	require.NoError(t, err)

	_, err = c.OpenFileTab(fileURI, false)
	require.NoError(t, err)
	ed, err := c.Editor(fileURI)
	require.NoError(t, err)
	_, hasTree := ed.CellView().(*syntax.Tree)
	assert.True(t, hasTree,
		"MaxSyntaxParseSize=0 must behave as no limit")
}