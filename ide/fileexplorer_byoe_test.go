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

package ide

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// TestFileExplorerBYOEEnter is the regression test for the bug
// where editor.mode = "byoe" prevented the :fexplorer command from
// rendering and reacting to <Enter> because BYOE cannot serve the
// memory:///fexplorer pseudo-URI used by the file explorer's mirror
// tab. The fix wraps the BYOE editor in the byoefallback router,
// which dispatches memory:// URIs to a Rune-native fallback editor
// (modeless by default).
//
// The test boots a real IDE pointed at a temp directory, configures
// editor.mode = "byoe" with a valid vim command + goto template, and
// asserts that:
//
//  1. :fexplorer opens and renders a non-empty tree;
//  2. <Enter> on the first (directory) row toggles the tree;
//  3. the workspace editor still reports IsExternal()=true (so
//     auto-save / FS-event reloads remain disabled workspace-wide).
func TestFileExplorerBYOEEnter(t *testing.T) {
	if _, err := exec.LookPath("vim"); err != nil {
		t.Skip("vim binary not available")
	}

	// EvalSymlinks: macOS t.TempDir() returns /var/... but the FS
	// scheme canonicalises to /private/var/... so URI lookup must
	// match.
	rawDir := t.TempDir()
	dir, err := filepath.EvalSymlinks(rawDir)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(
		filepath.Join(dir, "subdir"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "subdir", "child.txt"),
		[]byte("hi"), 0o644))

	cfg := defaultConfigWithWrap(false)
	editorCfg := cfg.cfg["editor"].(map[string]any)
	editorCfg["mode"] = "byoe"
	editorCfg["byoe"] = map[string]any{
		"command": `vim -Nu NONE -n "+call cursor({line}, {col})" {file}`,
		"goto":    "<esc>:{line}<enter>{col}|",
	}
	cfg.ringBell = func() {}
	require.Equal(t, "byoe", cfg.editorMode())
	// The byoefallback default is modeless; verify it propagated so
	// downstream behaviour (Enter toggles, no vi search) matches.
	require.Equal(t, "modeless", cfg.byoeFallback())

	uri, err := workspaceapi.ParseURI("file://" + dir)
	require.NoError(t, err)
	m := newTestWorkspaceManagerHandlerWithDir(t, cfg, dir,
		nopShutdownShaderConfig())
	t.Cleanup(func() { _ = m.Close() })

	require.NoError(t, m.addOrCreateWorkspace(uri))
	m.drainPendingWorkspaces()

	h := newSafeHandler(m)
	h.Resize(40, 12)

	ex := m.focusEx()
	require.NotNil(t, ex)
	assert.True(t, ex.ed.IsExternal(),
		"byoe workspace must remain externally managed even when "+
			"some URIs route to the fallback")

	m.mu.Lock()
	err = ex.fexplorer(context.Background())
	m.mu.Unlock()
	require.NoError(t, err, "fexplorer must open under byoe via the "+
		"byoefallback router for memory:///fexplorer")
	require.NotNil(t, ex.fileExplorerWin,
		"file explorer window must be present")
	require.NotNil(t, ex.fileExplorerHandler,
		"file explorer handler must be cached")

	explorer := ex.fileExplorerHandler
	beforeRows := explorer.ed.CellView().Rows()
	require.Greater(t, beforeRows, 0,
		"explorer tree must render at least one row")

	// With cursor on the first (directory) row, <Enter> must expand
	// the tree. The modeless fallback delivers <Enter> to the inner
	// editor handler, which the file explorer interprets as
	// expand-or-open since it is not in search mode.
	_, handled := h.Handle(term.Event{
		Type: term.EventKey, Key: term.KeyEnter,
	})
	require.True(t, handled, "<Enter> must be handled")
	require.False(t, explorer.ed.IsSearchMode(),
		"modeless fallback must not enter search mode on <Enter>")
	assert.NotEqual(t, beforeRows, explorer.ed.CellView().Rows(),
		"<Enter> on a directory row must toggle the tree")
}
