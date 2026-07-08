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

package main

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
)

// captureLSP records the InitializeParams it received and otherwise
// no-ops every request, so the discovery tests assert against the
// captured params without running a real gopls. The inits channel lets a
// test synchronize on the asynchronous, event-driven bring-up.
type captureLSP struct {
	noopLSP
	mu         sync.Mutex
	initParams *semanticapi.InitializeParams
	initCount  int
	inits      chan struct{}
}

func (l *captureLSP) Initialize(
	_ context.Context, p semanticapi.InitializeParams,
) (semanticapi.InitializeResult, error) {
	l.mu.Lock()
	cp := p
	l.initParams = &cp
	l.initCount++
	signal := l.inits
	l.mu.Unlock()
	if signal != nil {
		signal <- struct{}{}
	}
	return semanticapi.InitializeResult{}, nil
}

func (l *captureLSP) captured() (semanticapi.InitializeParams, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.initParams == nil {
		return semanticapi.InitializeParams{}, l.initCount
	}
	return *l.initParams, l.initCount
}

// waitForInit blocks until the next Initialize call completes or the
// timeout elapses, so tests can synchronize on the event-driven bring-up.
func (l *captureLSP) waitForInit(t *testing.T, timeout time.Duration) {
	t.Helper()
	l.mu.Lock()
	if l.inits == nil {
		l.inits = make(chan struct{}, 1)
	}
	signal := l.inits
	l.mu.Unlock()
	select {
	case <-signal:
	case <-time.After(timeout):
		t.Fatal("timed out waiting for lsp.Initialize")
	}
}

// TestExtendWorkspaceNestedDiscovery verifies that a workspace with no
// root go.mod is not initialized on startup, but opening a .go file under
// a nested module brings up gopls rooted at that module. A marker-less
// .go open is ignored.
func TestExtendWorkspaceNestedDiscovery(t *testing.T) {
	root := t.TempDir()
	mod := filepath.Join(root, "services", "api")
	require.NoError(t, os.MkdirAll(mod, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(mod, "go.mod"), []byte("module example.com/api\n\ngo 1.22\n"), 0o644))
	require.NoError(t, os.WriteFile(
		filepath.Join(mod, "main.go"), []byte("package main\n"), 0o644))

	stray := filepath.Join(root, "stray")
	require.NoError(t, os.MkdirAll(stray, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(stray, "loose.go"), []byte("package stray\n"), 0o644))

	scheme := newTestSchemeRooted(root)
	lsp := &captureLSP{}
	editor := newMockEditor()
	ext := &goExtension{}

	registered := false
	err := ext.extendWorkspaceWith(context.Background(),
		scheme, scheme, &mockNotifications{}, lsp, editor,
		nil, nil, nil, nil, "/data", nil,
		func(textapi.CommandManual, textapi.CommandHandler) error {
			registered = true
			return nil
		})
	require.NoError(t, err)
	assert.True(t, registered, "the go command must register independent of any module")

	// No root go.mod, so nothing is initialized on startup.
	_, count := lsp.captured()
	require.Zero(t, count)

	// A .go with no enclosing go.mod must not spawn a server.
	editor.open(t, filepath.Join(stray, "loose.go"))

	// Opening the nested module's source initializes gopls rooted there.
	editor.open(t, filepath.Join(mod, "main.go"))
	lsp.waitForInit(t, 5*time.Second)

	params, count := lsp.captured()
	require.Equal(t, 1, count, "only the marked nested module must initialize")
	assert.Equal(t, "file://"+mod, params.RootURI)
}
