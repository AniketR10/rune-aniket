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
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/text"
)

// TestBYOEVimSetCursorAtScroll guards the SetCursorAtScroll wiring in
// byoe end-to-end against vim. The fix it locks in: byoe's
// SetCursorAtScroll synthesises term.Event from the rendered goto
// KeyComb sequence, and vte's input path falls back to ev.Raw for
// ordinary keys (digits, ':', '|', '<enter>', '<esc>'). Without raw
// bytes populated the entire goto sequence is silently dropped, the
// cursor stays at (1,1), and an `i<marker><esc>:wq` prepends the
// marker to line 1 instead of the requested position.
//
// The test prepares a known multi-line file, asks byoe to place the
// cursor at (line=3, col=2), inserts a marker, writes, and asserts
// the on-disk content matches the expectation for that exact
// position. Any regression in cursor injection makes the marker
// appear on the wrong line/column and the assertion fails with the
// actual placement.
func TestBYOEVimSetCursorAtScroll(t *testing.T) {
	if _, err := exec.LookPath("vim"); err != nil {
		t.Skip("vim binary not available")
	}

	dir := t.TempDir()
	canonical, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	dir = canonical

	// Fixed-width lines so column math is unambiguous. Lines are
	// AAAA, BBBB, CCCC, DDDD, EEEE with a trailing newline.
	initial := "AAAA\nBBBB\nCCCC\nDDDD\nEEEE\n"
	relFile := "cursor.txt"
	filePath := filepath.Join(dir, relFile)
	require.NoError(t, os.WriteFile(filePath, []byte(initial), 0o644))

	cfg := defaultConfigWithWrap(false)
	editorCfg := cfg.cfg["editor"].(map[string]any)
	editorCfg["mode"] = "byoe"
	editorCfg["byoe"] = map[string]any{
		// Deterministic vim: no rc, no swap. Crucially we do NOT
		// pass +startinsert! so vim lands in normal mode and the
		// goto sequence below ('<esc>:{line}<enter>{col}|') can
		// position the cursor before we switch to insert.
		"command": `vim -Nu NONE -n {file}`,
		// Default goto: ESC, :<line><enter>, then <col>| in normal
		// mode. `|` is vim's go-to-column motion (1-based).
		"goto": "<esc>:{line}<enter>{col}|",
	}
	cfg.ringBell = func() {}
	require.Equal(t, "byoe", cfg.editorMode())

	uri, err := workspaceapi.ParseURI("file://" + dir)
	require.NoError(t, err)

	m := newTestWorkspaceManagerHandlerWithDir(t, cfg, dir,
		nopShutdownShaderConfig())
	t.Cleanup(func() { _ = m.Close() })

	require.NoError(t, m.addOrCreateWorkspace(uri))
	m.drainPendingWorkspaces()

	h := newSafeHandler(m)
	h.Resize(80, 24)

	dispatch := func(evs ...term.Event) {
		for _, ev := range evs {
			_, _ = h.Handle(ev)
		}
	}
	openSeq, err := term.ParseKeys(
		`<c-\\>edit<space>` + relFile + `<enter>`)
	require.NoError(t, err)
	for _, k := range openSeq {
		dispatch(keyEvent(k))
	}

	// Let vim start before we interact with the byoe handler.
	time.Sleep(750 * time.Millisecond)

	// Reach into the workspace to grab the actual text.Handler for
	// the file. SetCursorAtScroll is part of text.Handler, not the
	// outer browser.Tab, so we go through ex.comp.Editor.
	fileURI, err := m.workspaces[m.focus].ex.workspace.URI(relFile)
	require.NoError(t, err)
	eh, err := m.workspaces[m.focus].ex.comp.Editor(fileURI)
	require.NoError(t, err)

	// Position cursor on line 3 ("CCCC"), column 2: i.e. between
	// the first and second 'C'. The goto template renders
	// <esc>:3<enter>2|. Without the Raw-bytes fix in
	// byoe.editorHandler.SetCursorAtScroll, those keys never reach
	// the pty and vim stays at (1,1).
	require.True(t, eh.SetCursorAtScroll(term.Coordinates{Y: 2, X: 1}),
		"SetCursorAtScroll must report success when a goto "+
			"template is configured")

	// Tiny pause so vim consumes the goto sequence before we type.
	time.Sleep(150 * time.Millisecond)

	// Insert a marker at the cursor position then write.
	const marker = "XX"
	dispatch(keyEvent(term.KeyComb{Ch: 'i'}))
	for _, r := range marker {
		dispatch(keyEvent(term.KeyComb{Ch: r}))
	}
	dispatch(
		keyEvent(term.KeyComb{Key: term.KeyEsc}),
		keyEvent(term.KeyComb{Ch: ':'}),
		keyEvent(term.KeyComb{Ch: 'w'}),
		keyEvent(term.KeyComb{Ch: 'q'}),
		keyEvent(term.KeyComb{Key: term.KeyEnter}),
	)

	// Expected on-disk content after the edit: line 3 becomes
	// "CXXCCC" because `i` in vim inserts BEFORE the cursor and the
	// cursor was on the second character of "CCCC".
	expected := "AAAA\nBBBB\nCXXCCC\nDDDD\nEEEE\n"

	require.Eventually(t, func() bool {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return false
		}
		return string(data) == expected
	}, 10*time.Second, 100*time.Millisecond,
		"file %q must equal expected content after vim :wq:\n"+
			"want:\n%q\n",
		filePath, expected)

	got, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, expected, string(got),
		"SetCursorAtScroll must place the marker at the exact "+
			"position requested; mismatch means the goto "+
			"sequence did not reach vim and the cursor stayed "+
			"elsewhere")
}

// compile-time guard: text.Handler must expose SetCursorAtScroll.
var _ interface {
	SetCursorAtScroll(term.Coordinates) bool
} = (text.Handler)(nil)
