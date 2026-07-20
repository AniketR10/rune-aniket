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

package starlarktutorial

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"

	"unstable.build/go-tui/browser"
)

// fuzzySearchTutorialPath is the shipped tutorial the fuzzy-search
// package registers via its config.star tutorials entry.
const fuzzySearchTutorialPath = "../../../cmd/extension_fuzzy_search/fuzzy_search.star"

func newFuzzySearchTutorial(t *testing.T) (*Tutorial, *fakeNotis) {
	t.Helper()
	src, err := os.ReadFile(fuzzySearchTutorialPath)
	require.NoError(t, err)
	notis := &fakeNotis{}
	tut, err := New(
		"fuzzy_search", string(src),
		nil, nil, notis, nil,
		term.Attributes{}, component.FrameCharSet{}, browser.PromptConfig{},
		nil, nil,
		term.KeyComb{Ch: ':'},
		"standard", nil,
		nil,
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, tut)
	tut.Resize(80, 24)
	return tut, notis
}

// TestFuzzySearchTutorialRegisters asserts the shipped tutorial parses
// and registers under the id the config.star tutorials entry expects.
func TestFuzzySearchTutorialRegisters(t *testing.T) {
	t.Parallel()
	tut, _ := newFuzzySearchTutorial(t)
	require.Equal(t, "fuzzy_search", tut.id)
	require.Equal(t, "Fuzzy Search", tut.title)
}

// TestFuzzySearchTutorialFlow drives the shipped tutorial through its
// full step sequence: the searchfile/searchtext commands resolve via
// the command observer, and the type/navigate/open/cancel keys advance
// the bottom-anchored teaching windows through their dismiss keys.
func TestFuzzySearchTutorialFlow(t *testing.T) {
	t.Parallel()
	tut, _ := newFuzzySearchTutorial(t)
	resetAndWait(t, tut, time.Second)

	key := func(spec string) {
		t.Helper()
		ks, err := term.ParseKeys(spec)
		require.NoError(t, err)
		require.Len(t, ks, 1)
		kc := ks[0]
		_, _ = tut.Handle(term.Event{
			Type: term.EventKey, Key: kc.Key, Mod: kc.Mod, Ch: kc.Ch,
		})
	}

	// Intro window -> dismiss with the command-prompt key.
	require.Equal(t, "floating_window", activeKindFor(tut))
	key("<enter>")

	// "Search files" window -> dismiss, then dispatch searchfile.
	waitNextActive(t, tut, "floating_window", time.Second)
	key("<enter>")
	waitNextActive(t, tut, "wait_command", time.Second)
	tut.ObserveCommand("searchfile", "searchfile", nil, nil)

	// "Find and open a file": a wait_event step. It must pass every key
	// through to the focused finder so the user can type and navigate;
	// it resolves only on the file-open event, not from keystrokes.
	waitNextActive(t, tut, "wait_event", time.Second)
	ks, err := term.ParseKeys("a")
	require.NoError(t, err)
	_, handled := tut.Handle(term.Event{Type: term.EventKey, Ch: ks[0].Ch})
	require.False(t, handled,
		"wait_event must not swallow typing; keys reach the finder")
	require.Equal(t, "wait_event", activeKindFor(tut),
		"a printable key must not resolve the wait_event step")
	tut.ObserveEvent("open", "file:///workspace/main.go")

	// "Search file contents" window -> dismiss, then dispatch searchtext.
	waitNextActive(t, tut, "floating_window", time.Second)
	key("<enter>")
	waitNextActive(t, tut, "wait_command", time.Second)
	tut.ObserveCommand("searchtext", "searchtext", nil, nil)

	// "Search functions" window -> dismiss, then dispatch searchfunc.
	waitNextActive(t, tut, "floating_window", time.Second)
	key("<enter>")
	waitNextActive(t, tut, "wait_command", time.Second)
	tut.ObserveCommand("searchfunc", "searchast", nil, nil)

	// "Search types" window -> dismiss, then dispatch searchtype.
	waitNextActive(t, tut, "floating_window", time.Second)
	key("<enter>")
	waitNextActive(t, tut, "wait_command", time.Second)
	tut.ObserveCommand("searchtype", "searchast", nil, nil)

	// Wrap-up window -> dismiss to finish.
	waitNextActive(t, tut, "floating_window", time.Second)
	key("<enter>")
	waitFinished(t, tut, time.Second)
}

// TestFloatingWindowEscIsSwallowedNotPassedThrough documents why a
// floating_window cannot teach "press <esc> to cancel the finder": <esc>
// (like <enter>/<space>) is a reserved auto-advance key that the overlay
// consumes (handled=true) before the dismiss_keys/allow_keys checks, so it
// never falls through to the focused window. A tutorial that needs the user
// to actually cancel a finder must not rely on <esc> reaching the IDE root
// through a teaching panel.
func TestFloatingWindowEscIsSwallowedNotPassedThrough(t *testing.T) {
	t.Parallel()
	src := `
def run():
    floating_window(text="cancel the finder", dismiss_keys=["<esc>"])
    floating_window(text="done")
tutorial(entry=run)
`
	tut, _ := newTutorial(t, src)
	resetAndWait(t, tut, time.Second)
	require.Equal(t, "floating_window", activeKindFor(tut))

	exit, handled := tut.Handle(term.Event{Type: term.EventKey, Key: term.KeyEsc})
	assert.False(t, exit)
	assert.True(t, handled,
		"<esc> is swallowed by the overlay; it never reaches the finder, "+
			"so dismiss_keys=[\"<esc>\"] cannot cancel a focused window")

	// It still advances the tutorial (auto-advance), proving the step
	// resolves without the finder ever seeing <esc>.
	waitNextActive(t, tut, "floating_window", time.Second)
}
