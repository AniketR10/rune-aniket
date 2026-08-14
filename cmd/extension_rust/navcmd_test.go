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
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// recordingOpener returns a fixed handler and records that it was asked
// to open, standing in for the browser's resource opener whose returned
// token is the only handler SetWindowContent can accept.
type recordingOpener struct {
	h      browserapi.Handler
	opened []workspaceapi.URI
}

func (o *recordingOpener) Open(uri workspaceapi.URI) (browserapi.Handler, error) {
	o.opened = append(o.opened, uri)
	return o.h, nil
}

// handleEditor is a fakeEditor whose Editor returns a distinct symbolic
// handle, mirroring the production textrpc client where the editor
// handle is not a browser content token.
type handleEditor struct {
	fakeEditor
	h textapi.Handler
}

func (e *handleEditor) Editor(workspaceapi.URI) (textapi.Handler, error) { return e.h, nil }

// openLocation must set the opener's handler as the window content. The
// production browser stream only short-circuits on its own opener
// tokens; the editor's handle is symbolic, and streaming it as content
// panics the extension on the IDE's first Resize callback (seen live as
// a dead rust extension after every picker jump).
func TestOpenLocationSetsOpenerHandlerAsContent(t *testing.T) {
	opened := &stubResource{}
	editorHandle := &stubResource{}
	opener := &recordingOpener{h: opened}
	editor := &handleEditor{h: editorHandle}
	wm := &fakeWM{}

	err := openLocation(editor, wm, opener, fakeWindow{id: editorWinID}, newTestURI(t),
		semanticapi.Location{URI: "file:///ws/src/lib.rs"})
	require.NoError(t, err)
	win, content := wm.lastContent()
	require.NotNil(t, content, "the jump must set the window content")
	require.Same(t, opened, content,
		"window content must be the opener's handler, not the editor handle")
	require.Equal(t, uint64(editorWinID), win.WindowID(),
		"the jump must land in the window it was given")
}
