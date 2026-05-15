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
)

// TestBYOEVimEndToEnd drives the full ide → byoe → vte → vim pipeline:
// it boots an ide pointed at a real file scheme workspace, configures
// editor.mode = "byoe" with vim, opens a real file from disk, types
// content via the embedded vte, and asserts the file was modified on
// disk after vim's `:wq` writes and quits.
//
// Reproduces the regression where byoe pre-tokenised the configured
// argv via shell.Fields, then vte.Component.createPty joined and
// shell.Fields-tokenised it again, causing
//
//	edit: byoe: new vte handler: expand shell arguments: 1:18: ( is not a valid word
//
// when the configured command contained punctuation such as
// `vim "+call cursor({line}, {col})" {file}`.
//
// Also acts as the end-to-end guard for the byoe noise-suppression
// invariant that autoSaver must not be wired under an external editor
// (otherwise external saves race the saver and spam ErrStaleData
// warnings).
func TestBYOEVimEndToEnd(t *testing.T) {
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
	const insertedContent = "BYOE"
	relFile := "byoe.txt"
	filePath := filepath.Join(dir, relFile)
	require.NoError(t, os.WriteFile(filePath, []byte(initialContent), 0o644))

	cfg := defaultConfigWithWrap(false)
	editorCfg := cfg.cfg["editor"].(map[string]any)
	editorCfg["mode"] = "byoe"
	// Turning auto_save on lets the e2e also guard the
	// "auto-save must be skipped under byoe" wiring: the
	// autoSaverFactory hook below must never fire under an
	// external editor.
	editorCfg["auto_save"] = true
	editorCfg["byoe"] = map[string]any{
		// Quoted argv segments intentionally exercise the parens
		// that regressed in shell.Fields double-tokenisation. The
		// extra flags make startup deterministic across machines:
		//   -Nu NONE     skip user vimrc.
		//   -n           disable swap.
		//   +startinsert! drop straight into insert mode at EOL so
		//                 subsequent keystrokes type literal text.
		"command": `vim -Nu NONE -n "+call cursor({line}, {col})" "+startinsert!" {file}`,
		// editor.byoe.goto is required by validateBYOE. The
		// concrete value is irrelevant for this test (we don't
		// drive SetCursorAtScroll) but it must be non-empty and
		// parse, otherwise the IDE silently falls back to modal
		// mode and the test stops exercising byoe at all.
		"goto": "<esc>:{line}<enter>{col}|",
	}
	// terminalConfig() reads cfg.ringBell; vte panics on a nil bell.
	cfg.ringBell = func() {}
	require.Equal(t, "byoe", cfg.editorMode())

	// Hook the autoSaver factory so we can assert byoe never wires
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
		"byoe must skip the autoSaver wiring; otherwise external "+
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
			"byoe session, even after the external editor saves")
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
