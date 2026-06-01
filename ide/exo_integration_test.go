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
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/text"
)

// TestExoVimEndToEnd drives the full ide → exo → vte → vim pipeline:
// it boots an ide pointed at a real file scheme workspace, configures
// editor.mode = "exo" with vim, opens a real file from disk, types
// content via the embedded vte, and asserts the file was modified on
// disk after vim's `:wq` writes and quits.
//
// Reproduces the regression where exo pre-tokenised the configured
// argv via shell.Fields, then vte.Component.createPty joined and
// shell.Fields-tokenised it again, causing
//
//	edit: exo: new vte handler: expand shell arguments: 1:18: ( is not a valid word
//
// when the configured command contained punctuation such as
// `vim "+call cursor({line}, {col})" {file}`.
//
// Also acts as the end-to-end guard for the exo noise-suppression
// invariant that autoSaver must not be wired under an external editor
// (otherwise external saves race the saver and spam ErrStaleData
// warnings).
func TestExoVimEndToEnd(t *testing.T) {
	vimBin, err := exec.LookPath("vim")
	if err != nil {
		t.Skip("vim binary not available")
	}
	_ = vimBin

	// EvalSymlinks because t.TempDir() returns /var/... on macOS but
	// the file scheme canonicalises to /private/var/... so URI lookup
	// must match.
	dir := t.TempDir()
	canonical, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	dir = canonical

	const initialContent = "hello\n"
	const insertedContent = "exo"
	relFile := "exo.txt"
	filePath := filepath.Join(dir, relFile)
	require.NoError(t, os.WriteFile(filePath, []byte(initialContent), 0o644))

	cfg := defaultConfigWithWrap(false)
	editorCfg := cfg.cfg["editor"].(map[string]any)
	editorCfg["mode"] = "exo"
	// Turning auto_save on lets the e2e also guard the
	// "auto-save must be skipped under exo" wiring: the
	// autoSaverFactory hook below must never fire under an
	// external editor.
	editorCfg["auto_save"] = true
	editorCfg["exo"] = map[string]any{
		// Quoted argv segments intentionally exercise the parens
		// that regressed in shell.Fields double-tokenisation. The
		// extra flags make startup deterministic across machines:
		//   -Nu NONE     skip user vimrc.
		//   -n           disable swap.
		//   +startinsert! drop straight into insert mode at EOL so
		//                 subsequent keystrokes type literal text.
		"command": `vim -Nu NONE -n "+call cursor({line}, {col})" "+startinsert!" {file}`,
		// editor.exo.goto is required by validateExo. The
		// concrete value is irrelevant for this test (we don't
		// drive SetCursorAtScroll) but it must be non-empty and
		// parse, otherwise the IDE silently falls back to modal
		// mode and the test stops exercising exo at all.
		"goto": "<esc>:{line}<enter>{col}|",
		"quit": "<esc>:qa<enter>",
	}
	// terminalConfig() reads cfg.ringBell; vte panics on a nil bell.
	cfg.ringBell = func() {}
	require.Equal(t, "exo", cfg.editorMode())

	// Hook the autoSaver factory so we can assert exo never wires
	// the saver up. Done before constructing the test handler so
	// the swap is in place when subscribeAllEvents runs.
	prevFactory := autoSaverFactory
	t.Cleanup(func() { autoSaverFactory = prevFactory })
	var autoSaverConstructed atomic.Bool
	autoSaverFactory = func(
		_ autoSaverFlusher, _ browserapi.Notifications,
		_ func(func()) bool, _ time.Duration,
	) *autoSaver {
		autoSaverConstructed.Store(true)
		return nil
	}

	uri, err := workspaceapi.ParseURI("file://" + dir)
	require.NoError(t, err)

	m := newTestWorkspaceManagerHandlerWithDir(t, cfg, dir,
		nopShutdownShaderConfig())
	t.Cleanup(func() { _ = m.Close() })

	require.NoError(t, m.addOrCreateWorkspace(uri))
	m.drainPendingWorkspaces()

	assert.False(t, autoSaverConstructed.Load(),
		"exo must skip the autoSaver wiring; otherwise external "+
			"saves race the saver and surface ErrStaleData warnings")

	// Resize the handler so the embedded vte has a viewport big
	// enough for vim to render its statusline and command area.
	h := newSafeHandler(m)
	h.Resize(80, 24)

	// Open the file via :edit so the same path that crashed in
	// production is exercised end-to-end. We dispatch events
	// directly rather than using handlertest.RunHandlerSequence
	// because the latter asserts the rendered frame and we only
	// care that the editor opens and writes the file.
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

	// Give vim a moment to launch and process its `+startinsert!`
	// before we drive keystrokes. There is no portable in-band
	// readiness signal across editors (and -n disables the swap
	// file we could otherwise poll), so we use a generous fixed
	// delay. The test still asserts modification on disk.
	time.Sleep(750 * time.Millisecond)

	// Drive vim: <text><esc>:wq<enter>. We started vim with
	// +startinsert! so we are already in insert mode.
	for _, r := range insertedContent {
		dispatch(keyEvent(term.KeyComb{Ch: r}))
	}
	dispatch(
		keyEvent(term.KeyComb{Key: term.KeyEsc}),
		keyEvent(term.KeyComb{Ch: ':'}),
		keyEvent(term.KeyComb{Ch: 'w'}),
		keyEvent(term.KeyComb{Ch: 'q'}),
		keyEvent(term.KeyComb{Key: term.KeyEnter}),
	)

	// vim should write and exit; the file is then modified on disk.
	require.Eventually(t, func() bool {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return false
		}
		return string(data) != initialContent &&
			strings.Contains(string(data), insertedContent)
	}, 10*time.Second, 100*time.Millisecond,
		"file %q must contain inserted text %q after vim :wq",
		filePath, insertedContent)

	got, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Contains(t, string(got), insertedContent,
		"on-disk file must reflect vim's writes")

	assert.False(t, autoSaverConstructed.Load(),
		"autoSaver must remain unwired for the duration of a "+
			"exo session, even after the external editor saves")
}

// keyEvent renders a KeyComb into a term.Event with the Raw bytes
// vte expects. Unlike handlertest.RunHandlerSequence, vte's input
// path falls back to ev.Raw for non-special keys (e.g. plain ASCII
// runes), so Ch alone is insufficient — the pty would receive an
// empty write and the embedded editor would not see the keystroke.
func keyEvent(k term.KeyComb) term.Event {
	ev := term.Event{
		Type: term.EventKey, Mod: k.Mod, Key: k.Key, Ch: k.Ch,
	}
	switch {
	case k.Key == term.KeyEsc:
		ev.Raw = []byte{0x1b}
	case k.Key == term.KeyEnter:
		ev.Raw = []byte{0x0d}
	case k.Key == term.KeySpace:
		ev.Raw = []byte{' '}
	case k.Key == term.KeyTab:
		ev.Raw = []byte{0x09}
	case k.Key == term.KeyBackspace:
		ev.Raw = []byte{0x7f}
	case k.Mod == term.ModCtrl && k.Ch >= 'a' && k.Ch <= 'z':
		ev.Raw = []byte{byte(k.Ch - 'a' + 1)}
	case k.Mod == term.ModCtrl && k.Ch == '\\':
		ev.Raw = []byte{0x1c}
	case k.Ch != 0:
		ev.Raw = []byte(string(k.Ch))
	}
	return ev
}

// TestExoVimSwapfileGracefulClose asserts that closing a exo tab
// hosting vim leaves no .swp file behind.
func TestExoVimSwapfileGracefulClose(t *testing.T) {
	if _, err := exec.LookPath("vim"); err != nil {
		t.Skip("vim binary not available")
	}
	// Isolate HOME so vim writes its .viminfo (and the spinner of
	// .viminf[a-z].tmp files it uses when the primary is locked)
	// under a fresh dir. A real $HOME can carry leftover viminf*.tmp
	// files from prior crashes, causing vim to bail with E929 before
	// it has a chance to react to our quit sequence — and the .swp
	// file would then linger past the Close.
	t.Setenv("HOME", t.TempDir())

	// t.TempDir() returns /var/... on macOS but the file scheme
	// canonicalises to /private/var/... so URI lookup must match.
	dir := t.TempDir()
	canonical, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	dir = canonical

	relFile := "exo.txt"
	filePath := filepath.Join(dir, relFile)
	require.NoError(t, os.WriteFile(filePath, []byte("hello\n"), 0o644))

	cfg := defaultConfigWithWrap(false)
	editorCfg := cfg.cfg["editor"].(map[string]any)
	editorCfg["mode"] = "exo"
	editorCfg["exo"] = map[string]any{
		// No -n so vim creates a swap file while running.
		"command": `vim -Nu NONE {file}`,
		"goto":    "<esc>:{line}<enter>{col}|",
		"quit":    "<esc>:qa!<enter>",
	}
	cfg.ringBell = func() {}
	require.Equal(t, "exo", cfg.editorMode())

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

	time.Sleep(750 * time.Millisecond)

	// Confirm vim actually created a swap so the post-close
	// assertion below is not vacuously true.
	swapFile := filepath.Join(dir, "."+relFile+".swp")
	require.Eventually(t, func() bool {
		_, err := os.Stat(swapFile)
		return err == nil
	}, 5*time.Second, 100*time.Millisecond,
		"vim must have created a swap file at %q while running",
		swapFile)

	fileURI, err := m.workspaces[m.focus].ex.workspace.URI(relFile)
	require.NoError(t, err)
	eh, err := m.workspaces[m.focus].ex.comp.Editor(fileURI)
	require.NoError(t, err)

	require.NoError(t, eh.Close())

	require.Eventually(t, func() bool {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return false
		}
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".swp") {
				return false
			}
		}
		return true
	}, 10*time.Second, 100*time.Millisecond,
		"editor.exo.quit must let vim clean up its swap file "+
			"before the PTY is torn down")
}

// TestExoVimSetCursorAtScroll guards the SetCursorAtScroll wiring in
// exo end-to-end against vim. The fix it locks in: exo's
// SetCursorAtScroll synthesises term.Event from the rendered goto
// KeyComb sequence, and vte's input path falls back to ev.Raw for
// ordinary keys (digits, ':', '|', '<enter>', '<esc>'). Without raw
// bytes populated the entire goto sequence is silently dropped, the
// cursor stays at (1,1), and an `i<marker><esc>:wq` prepends the
// marker to line 1 instead of the requested position.
//
// The test prepares a known multi-line file, asks exo to place the
// cursor at (line=3, col=2), inserts a marker, writes, and asserts
// the on-disk content matches the expectation for that exact
// position. Any regression in cursor injection makes the marker
// appear on the wrong line/column and the assertion fails with the
// actual placement.
func TestExoVimSetCursorAtScroll(t *testing.T) {
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
	editorCfg["mode"] = "exo"
	editorCfg["exo"] = map[string]any{
		// Deterministic vim: no rc, no swap. Crucially we do NOT
		// pass +startinsert! so vim lands in normal mode and the
		// goto sequence below ('<esc>:{line}<enter>{col}|') can
		// position the cursor before we switch to insert.
		"command": `vim -Nu NONE -n {file}`,
		// Default goto: ESC, :<line><enter>, then <col>| in normal
		// mode. `|` is vim's go-to-column motion (1-based).
		"goto": "<esc>:{line}<enter>{col}|",
		"quit": "<esc>:qa<enter>",
	}
	cfg.ringBell = func() {}
	require.Equal(t, "exo", cfg.editorMode())

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

	// Let vim start before we interact with the exo handler.
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
	// exoeditor.editorHandler.SetCursorAtScroll, those keys never reach
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
