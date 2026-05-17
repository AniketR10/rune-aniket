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

package byoefallback

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/text"
)

// stubEditor is a minimal text.Editor used to observe which child the
// router dispatched to.
type stubEditor struct {
	name string

	editURIs   []string
	editorURIs []string
	subEvts    int
	unsubCalls int
	cmdRegs    int
	cmdUnregs  int
	replRegs   int
	replUnregs int

	editErr     error
	subErr      error
	unsubResult bool
	unsubErr    error
}

func (s *stubEditor) Edit(
	_ context.Context, uri workspaceapi.URI, _ *cell.Buffer, _, _ bool,
) (text.Handler, error) {
	s.editURIs = append(s.editURIs, uri.String())
	if s.editErr != nil {
		return nil, s.editErr
	}
	return stubHandler{name: s.name}, nil
}

func (s *stubEditor) Editor(uri workspaceapi.URI) (text.Handler, error) {
	s.editorURIs = append(s.editorURIs, uri.String())
	return stubHandler{name: s.name}, nil
}

func (s *stubEditor) SubscribeCommand(
	textapi.CommandManual, text.CommandHandler,
) error {
	s.cmdRegs++
	return nil
}

func (s *stubEditor) RegisterREPLCommand(
	textapi.CommandManual, textapi.REPLHandler,
) error {
	s.replRegs++
	return nil
}

func (s *stubEditor) UnsubscribeCommand(string) error {
	s.cmdUnregs++
	return nil
}

func (s *stubEditor) UnregisterREPLCommand(string) error {
	s.replUnregs++
	return nil
}

func (s *stubEditor) SubscribeEvents(
	[]textapi.EventType, text.EventHandler,
) error {
	s.subEvts++
	return s.subErr
}

func (s *stubEditor) UnsubscribeEvents(text.EventHandler) (bool, error) {
	s.unsubCalls++
	return s.unsubResult, s.unsubErr
}

// IsExternal: stubs default to false. Tests that need true can use
// the byoeEd argument of newWithEditors; the wrapper's IsExternal()
// is the property under test, not the children's.
func (s *stubEditor) IsExternal() bool { return false }

// stubHandler is a no-op text.Handler used only to verify identity.
type stubHandler struct {
	name string
	text.Handler
}

func mustURI(t *testing.T, raw string) workspaceapi.URI {
	t.Helper()
	uri, err := workspaceapi.ParseURI(raw)
	require.NoError(t, err)
	return uri
}

// TestNewPanicsOnNilFallback verifies the constructor refuses a
// nil fallback. (BYOE-side missing arguments panic inside byoe.New
// and are exercised by text/byoe's own tests.)
func TestNewPanicsOnNilFallback(t *testing.T) {
	assert.Panics(t, func() {
		_ = newWithEditors(&stubEditor{}, nil)
	})
}

// TestIsExternalAlwaysTrue asserts the router always reports
// IsExternal()==true regardless of its children.
func TestIsExternalAlwaysTrue(t *testing.T) {
	r := newWithEditors(&stubEditor{}, &stubEditor{})
	assert.True(t, r.IsExternal())
}

// TestEditRouting drives Edit on a variety of URIs and asserts the
// router dispatched to the right child.
func TestEditRouting(t *testing.T) {
	cases := []struct {
		name   string
		uri    string
		expect string // "byoe" or "fallback"
	}{
		{"file", "file:///tmp/a.go", "byoe"},
		{"ssh", "ssh://host/tmp/a.go", "byoe"},
		{"memory", "memory:///fexplorer", "fallback"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			byoeEd := &stubEditor{name: "byoe"}
			fallback := &stubEditor{name: "fallback"}
			r := newWithEditors(byoeEd, fallback)
			uri := mustURI(t, tc.uri)

			h, err := r.Edit(context.Background(), uri, cell.NewBuffer(), false, false)
			require.NoError(t, err)
			sh, ok := h.(stubHandler)
			require.True(t, ok)
			assert.Equal(t, tc.expect, sh.name)
		})
	}
}

// TestEditErrorNotCached verifies a failed Edit does not record a
// route so a subsequent Editor() lookup re-dispatches by scheme.
func TestEditErrorNotCached(t *testing.T) {
	byoeEd := &stubEditor{name: "byoe", editErr: errors.New("boom")}
	fallback := &stubEditor{name: "fallback"}
	r := newWithEditors(byoeEd, fallback)
	uri := mustURI(t, "file:///x")

	_, err := r.Edit(context.Background(), uri, cell.NewBuffer(), false, false)
	require.Error(t, err)

	_, err = r.Editor(uri)
	require.NoError(t, err)
	assert.Equal(t, []string{"file:///x"}, byoeEd.editorURIs)
	assert.Empty(t, fallback.editorURIs)
}

// TestEditorLookupUsesRecordedRoute proves a successful Edit makes
// subsequent Editor() lookups hit the same child even if accepts()
// would have picked differently.
func TestEditorLookupUsesRecordedRoute(t *testing.T) {
	byoeEd := &stubEditor{name: "byoe"}
	fallback := &stubEditor{name: "fallback"}
	r := newWithEditors(byoeEd, fallback)

	uri := mustURI(t, "memory:///fexplorer")
	_, err := r.Edit(context.Background(), uri, cell.NewBuffer(), false, false)
	require.NoError(t, err)

	_, err = r.Editor(uri)
	require.NoError(t, err)
	assert.Empty(t, byoeEd.editorURIs)
	assert.Equal(t, []string{"memory:///fexplorer"}, fallback.editorURIs)
}

// TestEditorLookupUnseenURIUsesScheme verifies the accepts() rule
// applies when no Edit has been called for the URI yet.
func TestEditorLookupUnseenURIUsesScheme(t *testing.T) {
	byoeEd := &stubEditor{name: "byoe"}
	fallback := &stubEditor{name: "fallback"}
	r := newWithEditors(byoeEd, fallback)

	_, err := r.Editor(mustURI(t, "file:///never-edited"))
	require.NoError(t, err)
	assert.Equal(t, []string{"file:///never-edited"}, byoeEd.editorURIs)

	_, err = r.Editor(mustURI(t, "memory:///never-edited"))
	require.NoError(t, err)
	assert.Equal(t, []string{"memory:///never-edited"}, fallback.editorURIs)
}

// TestSubscribeAndUnsubscribeForwardToBYOE verifies SubscribeEvents
// and UnsubscribeEvents forward only to the byoe child. The fallback
// only serves IDE-owned pseudo-URIs (memory://) whose events the IDE
// does not react to.
func TestSubscribeAndUnsubscribeForwardToBYOE(t *testing.T) {
	byoeEd := &stubEditor{unsubResult: true}
	fallback := &stubEditor{unsubResult: true}
	r := newWithEditors(byoeEd, fallback)

	h := text.FuncEventHandler(
		func(context.Context, textapi.Event) bool { return false })
	require.NoError(t, r.SubscribeEvents(nil, h))
	assert.Equal(t, 1, byoeEd.subEvts)
	assert.Equal(t, 0, fallback.subEvts)

	ok, err := r.UnsubscribeEvents(h)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, 1, byoeEd.unsubCalls)
	assert.Equal(t, 0, fallback.unsubCalls)
}

// TestCommandAndREPLForwardToFallback proves command and REPL
// register/unregister calls hit only the fallback (BYOE returns
// "not supported" for these by design).
func TestCommandAndREPLForwardToFallback(t *testing.T) {
	byoeEd := &stubEditor{}
	fallback := &stubEditor{}
	r := newWithEditors(byoeEd, fallback)

	require.NoError(t, r.SubscribeCommand(textapi.CommandManual{}, nil))
	require.NoError(t, r.RegisterREPLCommand(textapi.CommandManual{}, nil))
	require.NoError(t, r.UnsubscribeCommand(""))
	require.NoError(t, r.UnregisterREPLCommand(""))

	assert.Equal(t, 0, byoeEd.cmdRegs)
	assert.Equal(t, 0, byoeEd.replRegs)
	assert.Equal(t, 0, byoeEd.cmdUnregs)
	assert.Equal(t, 0, byoeEd.replUnregs)
	assert.Equal(t, 1, fallback.cmdRegs)
	assert.Equal(t, 1, fallback.replRegs)
	assert.Equal(t, 1, fallback.cmdUnregs)
	assert.Equal(t, 1, fallback.replUnregs)
}
