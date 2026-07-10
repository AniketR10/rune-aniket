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
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
)

// TestRootContains asserts the containment relation used to route a
// file URI to the most-specific initialized root.
func TestRootContains(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		root    string
		file    string
		contain bool
	}{
		{"identical", "file:///ws", "file:///ws", true},
		{"direct child", "file:///ws", "file:///ws/a.py", true},
		{"nested child", "file:///ws", "file:///ws/sub/a.py", true},
		{"trailing slash root", "file:///ws/", "file:///ws/a.py", true},
		{"sibling prefix not contained", "file:///ws", "file:///wsx/a.py", false},
		{"outside", "file:///ws/sub", "file:///ws/a.py", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.contain, rootContains(tt.root, tt.file))
		})
	}
}

// TestServerForURILongestRoot asserts serverForURI returns the server
// rooted at the most-specific (longest) root URI that contains the
// file, so a nested project takes precedence over the workspace root.
func TestServerForURILongestRoot(t *testing.T) {
	t.Parallel()
	uri := makeURI(t, "file:///workspace")
	m := New(uri, nil, nil, nil, nil, nil, Config{NoInitializeServer: true})
	t.Cleanup(func() { _ = m.Close() })

	rootSrv := &fakeChild{name: "python"}
	nestedSrv := &fakeChild{name: "python"}
	m.mu.Lock()
	m.servers[serverKey{languageID: "python", rootURI: "file:///workspace"}] = rootSrv
	m.servers[serverKey{languageID: "python", rootURI: "file:///workspace/sub"}] = nestedSrv
	m.mu.Unlock()

	nested, err := m.serverForURI("file:///workspace/sub/app.py")
	require.NoError(t, err)
	assert.Same(t, nestedSrv, nested, "nested file must route to nested root server")

	root, err := m.serverForURI("file:///workspace/top.py")
	require.NoError(t, err)
	assert.Same(t, rootSrv, root, "top-level file must route to workspace-root server")

	outside, err := m.serverForURI("file:///elsewhere/x.py")
	require.NoError(t, err)
	assert.Same(t, rootSrv, outside,
		"file outside any initialized root must fall back to the "+
			"broadest same-language server")

	_, err = m.serverForURI("file:///elsewhere/x.go")
	require.ErrorIs(t, err, ErrNoServer,
		"a language with no running server has no fallback")
}

// TestServerForURIBroadestFallback asserts out-of-root files route to
// the same-language server with the broadest root: shortest rootURI,
// with a lexicographic tie-break for equal lengths.
func TestServerForURIBroadestFallback(t *testing.T) {
	t.Parallel()
	uri := makeURI(t, "file:///workspace")
	m := New(uri, nil, nil, nil, nil, nil, Config{NoInitializeServer: true})
	t.Cleanup(func() { _ = m.Close() })

	aaSrv := &fakeChild{name: "python"}
	abSrv := &fakeChild{name: "python"}
	m.mu.Lock()
	m.servers[serverKey{languageID: "python", rootURI: "file:///workspace/ab"}] = abSrv
	m.servers[serverKey{languageID: "python", rootURI: "file:///workspace/aa"}] = aaSrv
	m.mu.Unlock()

	srv, err := m.serverForURI("file:///home/u/go/pkg/mod/dep/x.py")
	require.NoError(t, err)
	assert.Same(t, aaSrv, srv,
		"equal-length roots must tie-break lexicographically")

	rootSrv := &fakeChild{name: "python"}
	m.mu.Lock()
	m.servers[serverKey{languageID: "python", rootURI: "file:///workspace"}] = rootSrv
	m.mu.Unlock()

	srv, err = m.serverForURI("file:///home/u/go/pkg/mod/dep/x.py")
	require.NoError(t, err)
	assert.Same(t, rootSrv, srv,
		"the shortest root must win once available")
}

// TestSendPendingOpensOutOfWorkspaceFallback asserts a pending open
// for a file outside the workspace root flushes to the first
// same-language server, since no root can ever contain it.
func TestSendPendingOpensOutOfWorkspaceFallback(t *testing.T) {
	t.Parallel()
	uri := makeURI(t, "file:///workspace")
	m := New(uri, nil, nil, nil, nil, nil, Config{NoInitializeServer: true})
	t.Cleanup(func() { _ = m.Close() })

	outsideURI := makeURI(t, "file:///goroot/lib/dep.py")
	m.mu.Lock()
	m.pendingOpens[outsideURI.String()] = textapi.Event{
		Type: textapi.EventTypeOpen, URI: outsideURI, Content: "x = 1\n"}
	m.mu.Unlock()

	srv := &langServer{
		cfg:     langConfig{id: "python"},
		rootURI: "file:///workspace",
		log:     slog.Default(),
	}
	m.sendPendingOpens(
		serverKey{languageID: "python", rootURI: "file:///workspace"}, srv)

	m.mu.Lock()
	_, stillPending := m.pendingOpens[outsideURI.String()]
	m.mu.Unlock()
	assert.False(t, stillPending,
		"out-of-workspace pending open must flush to the first "+
			"same-language server")
}

// TestInitializeRejectsRootOutsideWorkspace asserts Initialize refuses
// to start a server rooted outside the manager's workspace root.
func TestInitializeRejectsRootOutsideWorkspace(t *testing.T) {
	t.Parallel()
	uri := makeURI(t, "file:///workspace")
	m := New(uri, nil, nil, nil, nil, nil, Config{NoInitializeServer: true})
	t.Cleanup(func() { _ = m.Close() })

	params := semanticapi.InitializeParams{RootURI: "file:///other"}
	_, err := m.Initialize(t.Context(), params)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the workspace")
}

// TestSendPendingOpensRootScoped asserts pending opens are replayed
// only to the server whose root contains them, so a workspace-root
// server does not receive files that belong to a nested root.
func TestSendPendingOpensRootScoped(t *testing.T) {
	t.Parallel()
	uri := makeURI(t, "file:///workspace")
	m := New(uri, nil, nil, nil, nil, nil, Config{NoInitializeServer: true})
	t.Cleanup(func() { _ = m.Close() })

	topURI := makeURI(t, "file:///workspace/top.py")
	nestedURI := makeURI(t, "file:///workspace/sub/app.py")
	m.mu.Lock()
	m.pendingOpens[topURI.String()] = textapi.Event{
		Type: textapi.EventTypeOpen, URI: topURI, Content: "x = 1\n"}
	m.pendingOpens[nestedURI.String()] = textapi.Event{
		Type: textapi.EventTypeOpen, URI: nestedURI, Content: "y = 2\n"}
	m.mu.Unlock()

	// A nested-root server must claim only the nested file and leave
	// the top-level file pending for the workspace-root server. The
	// server is unstarted (alive=false), so notify is a harmless
	// no-op after the pending entry is consumed.
	srv := &langServer{
		cfg:     langConfig{id: "python"},
		rootURI: "file:///workspace/sub",
		log:     slog.Default(),
	}
	m.sendPendingOpens(
		serverKey{languageID: "python", rootURI: "file:///workspace/sub"}, srv)

	m.mu.Lock()
	_, topStillPending := m.pendingOpens[topURI.String()]
	_, nestedConsumed := m.pendingOpens[nestedURI.String()]
	m.mu.Unlock()

	assert.True(t, topStillPending,
		"workspace-root file must remain pending for its own root server")
	assert.False(t, nestedConsumed,
		"nested file must be consumed by the nested-root server")
}

// TestInstallRestartedIsRootScoped asserts that restarting a child in
// one root's multiLangServer does not touch the children of another
// root's server for the same language.
func TestInstallRestartedIsRootScoped(t *testing.T) {
	t.Parallel()
	uri := makeURI(t, "file:///workspace")
	m := New(uri, nil, nil, nil, nil, nil, Config{NoInitializeServer: true})
	t.Cleanup(func() { _ = m.Close() })

	rootChild := &langServer{cfg: langConfig{id: "python"}, rootURI: "file:///workspace"}
	nestedChild := &langServer{
		cfg: langConfig{id: "python"}, rootURI: "file:///workspace/sub"}
	rootMLS := &multiLangServer{
		cfg:      langConfig{id: "python"},
		children: []server{rootChild},
	}
	nestedMLS := &multiLangServer{
		cfg:      langConfig{id: "python"},
		children: []server{nestedChild},
	}
	rootKey := serverKey{languageID: "python", rootURI: "file:///workspace"}
	nestedKey := serverKey{languageID: "python", rootURI: "file:///workspace/sub"}
	m.mu.Lock()
	m.servers[rootKey] = rootMLS
	m.servers[nestedKey] = nestedMLS
	m.mu.Unlock()

	replacement := &langServer{cfg: langConfig{id: "python"}, rootURI: "file:///workspace"}
	m.installRestarted(rootKey, rootChild, replacement)

	assert.Same(t, replacement, rootMLS.children[0].(*langServer),
		"restart must replace the crashed child in its own root")
	assert.Same(t, nestedChild, nestedMLS.children[0].(*langServer),
		"restarting the workspace-root child must not replace nested-root children")
}
