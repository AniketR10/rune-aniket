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

package texttest

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/workspace"
)

// externalEditor wraps a TestEditor and reports itself as
// externally-managed.
type externalEditor struct {
	*TestEditor
	readOnlyObserved *bool
	editCalls        *int
}

// IsExternal overrides TestEditor.IsExternal to return true.
func (externalEditor) IsExternal() bool { return true }

func (e externalEditor) Edit(
	ctx context.Context,
	file workspaceapi.URI, buf *cell.Buffer, readOnly, recovered bool,
) (text.Handler, error) {
	if e.readOnlyObserved != nil {
		*e.readOnlyObserved = readOnly
	}
	if e.editCalls != nil {
		*e.editCalls++
	}
	return e.TestEditor.Edit(ctx, file, buf, readOnly, recovered)
}

// recordingLoader wraps testLoader and records the readOnly flag
// passed to Load.
type recordingLoader struct {
	*testLoader
	readOnlyObserved *bool
}

func (l recordingLoader) Load(
	file workspaceapi.URI, buf *cell.Buffer, swapDir workspaceapi.URI,
	readOnly bool,
) (workspace.FlusherCloser, error) {
	*l.readOnlyObserved = readOnly
	return l.testLoader.Load(file, buf, swapDir, readOnly)
}

// TestExternalEditorForcesReadOnly verifies that opening a file in a
// Component backed by an external Editor forces readOnly=true
// at both the editor.Edit and workspace.Load layer, regardless of what
// the caller passed to OpenFileTab.
func TestExternalEditorForcesReadOnly(t *testing.T) {
	var loaderRO, editorRO bool
	ed := externalEditor{TestEditor: NopEditor(), readOnlyObserved: &editorRO}
	loader := &testLoader{}
	wrappedLoader := recordingLoader{testLoader: loader, readOnlyObserved: &loaderRO}
	cfg := text.DefaultConfig()
	cfg.ScheduleNextTick = func(fn func()) bool { fn(); return true }
	c, err := text.NewComponent(ed, wrappedLoader, cfg)
	require.NoError(t, err)

	uri, err := workspaceapi.ParseURI("memory:///tmp/external.txt")
	require.NoError(t, err)

	// Caller asks for read/write; the external editor must coerce it.
	_, err = c.OpenFileTab(uri, false /* readOnly */)
	require.NoError(t, err)
	assert.True(t, loaderRO,
		"workspace.Load must receive readOnly=true under an external editor")
	assert.True(t, editorRO,
		"editor.Edit must receive readOnly=true under an external editor")
}

// TestExternalEditorDelegatesMarkdown locks in the fix for the bug
// where `.md` files opened under an external editor (BYOE) were
// rendered through the built-in markdown viewer instead of being
// handed to the external editor. The root cause was that
// Component.openFileTab conflated the BYOE mirror-buffer
// read-only invariant with the user's view intent: hoisting
// `readOnly = true` for any external editor then took the
// loadView/loadMarkdown branch unconditionally. The fix keeps the
// hoist but gates the view branch on the caller's original intent
// AND `!c.ed.IsExternal()` so the external editor's Edit always
// gets the file, for both :edit (readOnly=false) and :view
// (readOnly=true).
func TestExternalEditorDelegatesMarkdown(t *testing.T) {
	for _, tc := range []struct {
		name     string
		readOnly bool
	}{
		{name: "edit", readOnly: false},
		{name: "view", readOnly: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var editCalls int
			ed := externalEditor{TestEditor: NopEditor(), editCalls: &editCalls}
			loader := &testLoader{
				openFile: workspace.NewMemoryFile(
					"README.md", 1, 0,
					[]byte("# hi\n"), new(sync.Mutex)),
			}
			cfg := text.DefaultConfig()
			cfg.ScheduleNextTick = func(fn func()) bool { fn(); return true }
			c, err := text.NewComponent(ed, loader, cfg)
			require.NoError(t, err)

			uri, err := workspaceapi.ParseURI("memory:///tmp/README.md")
			require.NoError(t, err)

			_, err = c.OpenFileTab(uri, tc.readOnly)
			require.NoError(t, err)
			assert.Equal(t, 1, editCalls,
				"external editor.Edit must be called for .md "+
					"opens regardless of readOnly; got %d "+
					"calls (readOnly=%v)", editCalls, tc.readOnly)
		})
	}
}
