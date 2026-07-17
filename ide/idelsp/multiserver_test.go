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

package idelsp

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
)

// fakeChild is a server stand-in that records the methods routed to
// it via call and notify. It lets multiLangServer routing be tested
// without spawning real language-server processes.
type fakeChild struct {
	childName string

	mu      sync.Mutex
	calls   []string
	notifs  []string
	events  []string
	opens   []semanticapi.DidOpenTextDocumentParams
	started bool
	stopped bool
	closed  bool
	alive   bool

	// diagReport and diagErr are returned by pullDiagnostics so the
	// multi-server merge can be exercised without a real server.
	diagReport semanticapi.DocumentDiagnosticReport
	diagErr    error
}

func (f *fakeChild) call(_ context.Context, method string, _, _ any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, method)
	f.events = append(f.events, "call:"+method)
	return nil
}

func (f *fakeChild) pullDiagnostics(
	_ context.Context, _ semanticapi.DocumentDiagnosticParams,
) (semanticapi.DocumentDiagnosticReport, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "textDocument/diagnostic")
	f.events = append(f.events, "call:textDocument/diagnostic")
	return f.diagReport, f.diagErr
}

func (f *fakeChild) notify(_ context.Context, method string, params any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.notifs = append(f.notifs, method)
	f.events = append(f.events, "notify:"+method)
	if p, ok := params.(semanticapi.DidOpenTextDocumentParams); ok {
		f.opens = append(f.opens, p)
	}
	return nil
}

func (f *fakeChild) initialize(_ context.Context) (semanticapi.InitializeResult, error) {
	return semanticapi.InitializeResult{}, nil
}

func (f *fakeChild) start(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started = true
	f.alive = true
	return nil
}

func (f *fakeChild) stop(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped = true
	f.alive = false
	return nil
}

func (f *fakeChild) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *fakeChild) config() langConfig {
	return langConfig{id: f.childName, command: f.childName}
}

func (f *fakeChild) key() serverKey {
	return serverKey{languageID: f.childName}
}

func (f *fakeChild) name() string {
	return f.childName
}

func (f *fakeChild) initResult() semanticapi.InitializeResult {
	return semanticapi.InitializeResult{}
}

func (f *fakeChild) isAlive() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.alive
}

func (f *fakeChild) callMethods() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func (f *fakeChild) notifyMethods() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.notifs...)
}

// eventLog returns call and notify methods in the order they arrived,
// prefixed with "call:" or "notify:".
func (f *fakeChild) eventLog() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.events...)
}

// didOpens returns the params of every textDocument/didOpen notify.
func (f *fakeChild) didOpens() []semanticapi.DidOpenTextDocumentParams {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]semanticapi.DidOpenTextDocumentParams(nil), f.opens...)
}

// TestServerConformance asserts that both backend implementations
// satisfy the server interface (which embeds io.Closer).
func TestServerConformance(t *testing.T) {
	t.Parallel()
	var _ server = (*langServer)(nil)
	var _ server = (*multiLangServer)(nil)
	var _ server = (*fakeChild)(nil)
}

func newTestMultiServer(def, alt *fakeChild, routes map[string]int) *multiLangServer {
	return &multiLangServer{
		cfg:      langConfig{id: "python", command: def.childName},
		children: []server{def, alt},
		routes:   routes,
	}
}

// TestMultiServerRouting asserts that requests (call) dispatch to the
// routed child and fall back to the default child for unrouted
// methods, while notifications are always fanned out to every child.
func TestMultiServerRouting(t *testing.T) {
	t.Parallel()

	t.Run("call routes to mapped child and default otherwise", func(t *testing.T) {
		t.Parallel()
		def := &fakeChild{childName: "ty"}
		alt := &fakeChild{childName: "ruff"}
		mls := newTestMultiServer(def, alt, map[string]int{
			"textDocument/formatting": 1,
		})

		require.NoError(t, mls.call(t.Context(), "textDocument/formatting", nil, nil))
		require.NoError(t, mls.call(t.Context(), "textDocument/hover", nil, nil))

		assert.Equal(t, []string{"textDocument/formatting"}, alt.callMethods())
		assert.Equal(t, []string{"textDocument/hover"}, def.callMethods())
	})

	t.Run("notifications always fan out to all children", func(t *testing.T) {
		t.Parallel()
		def := &fakeChild{childName: "ty"}
		alt := &fakeChild{childName: "ruff"}
		// A routed request method (formatting) must not narrow
		// notification fan-out: notifications have no response, so
		// every child must observe them to keep a consistent view.
		mls := newTestMultiServer(def, alt, map[string]int{
			"textDocument/formatting": 1,
		})

		// Document-sync and workspace notifications alike: both go to
		// every child, regardless of any request route.
		methods := []string{
			"textDocument/didOpen",
			"textDocument/didChange",
			"textDocument/didClose",
			"textDocument/didSave",
			"textDocument/formatting",
			"workspace/didChangeWatchedFiles",
			"workspace/didChangeConfiguration",
		}
		for _, method := range methods {
			require.NoError(t, mls.notify(t.Context(), method, nil))
		}

		assert.Equal(t, methods, def.notifyMethods())
		assert.Equal(t, methods, alt.notifyMethods())
	})
}

// TestMultiServerLifecycle asserts that start, stop, and Close fan
// out to every child and that isAlive reflects any live child.
func TestMultiServerLifecycle(t *testing.T) {
	t.Parallel()
	def := &fakeChild{childName: "ty"}
	alt := &fakeChild{childName: "ruff"}
	mls := newTestMultiServer(def, alt, nil)

	require.NoError(t, mls.start(t.Context()))
	assert.True(t, def.started)
	assert.True(t, alt.started)
	assert.True(t, mls.isAlive())

	require.NoError(t, mls.stop(t.Context()))
	assert.True(t, def.stopped)
	assert.True(t, alt.stopped)
	assert.False(t, mls.isAlive())

	require.NoError(t, mls.Close())
	assert.True(t, def.closed)
	assert.True(t, alt.closed)
}

// TestMultiServerReplaceChild asserts that replaceChild swaps a child
// by interface identity, leaving the others untouched.
func TestMultiServerReplaceChild(t *testing.T) {
	t.Parallel()
	def := &fakeChild{childName: "ty"}
	alt := &fakeChild{childName: "ruff"}
	mls := newTestMultiServer(def, alt, map[string]int{
		"textDocument/formatting": 1,
	})

	replacement := &fakeChild{childName: "ruff-restarted"}
	mls.replaceChild(alt, replacement)

	require.NoError(t, mls.call(t.Context(), "textDocument/formatting", nil, nil))
	assert.Equal(t, []string{"textDocument/formatting"}, replacement.callMethods())
	assert.Empty(t, alt.callMethods())
}

// TestMultiServerPullDiagnosticsMerges asserts that pullDiagnostics
// fans out to every child and concatenates their reports, so a single
// pull returns findings from all backends (ty type errors and ruff
// lint) rather than only the default child's.
func TestMultiServerPullDiagnosticsMerges(t *testing.T) {
	t.Parallel()

	def := &fakeChild{childName: "ty", diagReport: semanticapi.DocumentDiagnosticReport{
		Kind:  "full",
		Items: []semanticapi.Diagnostic{{Source: "ty", Code: "invalid-argument-type"}},
	}}
	alt := &fakeChild{childName: "ruff", diagReport: semanticapi.DocumentDiagnosticReport{
		Kind:  "full",
		Items: []semanticapi.Diagnostic{{Source: "Ruff", Code: "F401"}},
	}}
	mls := newTestMultiServer(def, alt, map[string]int{
		"textDocument/formatting": 1,
	})

	report, err := mls.pullDiagnostics(t.Context(),
		semanticapi.DocumentDiagnosticParams{})
	require.NoError(t, err)

	bySource := map[string]string{}
	for _, d := range report.Items {
		bySource[d.Source] = d.Code
	}
	assert.Equal(t, "invalid-argument-type", bySource["ty"])
	assert.Equal(t, "F401", bySource["Ruff"])
	assert.Len(t, report.Items, 2)
}

// TestMultiServerPullDiagnosticsSkipsFailingChild asserts that a child
// which errors on the pull (e.g. one that does not support it) is
// skipped while the other child's findings are still returned.
func TestMultiServerPullDiagnosticsSkipsFailingChild(t *testing.T) {
	t.Parallel()

	def := &fakeChild{childName: "ty", diagErr: errors.New("method not found")}
	alt := &fakeChild{childName: "ruff", diagReport: semanticapi.DocumentDiagnosticReport{
		Kind:  "full",
		Items: []semanticapi.Diagnostic{{Source: "Ruff", Code: "F401"}},
	}}
	mls := newTestMultiServer(def, alt, nil)

	report, err := mls.pullDiagnostics(t.Context(),
		semanticapi.DocumentDiagnosticParams{})
	require.NoError(t, err)
	require.Len(t, report.Items, 1)
	assert.Equal(t, "F401", report.Items[0].Code)
}

// TestMultiServerPullDiagnosticsAllFail asserts that when every child
// fails the joined error is returned rather than an empty report.
func TestMultiServerPullDiagnosticsAllFail(t *testing.T) {
	t.Parallel()

	def := &fakeChild{childName: "ty", diagErr: errors.New("boom-ty")}
	alt := &fakeChild{childName: "ruff", diagErr: errors.New("boom-ruff")}
	mls := newTestMultiServer(def, alt, nil)

	_, err := mls.pullDiagnostics(t.Context(),
		semanticapi.DocumentDiagnosticParams{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom-ty")
	assert.Contains(t, err.Error(), "boom-ruff")
}
