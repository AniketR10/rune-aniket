// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.

package extension

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/tui"
)

// recordingWindowManager is a stub browserapi.WindowManager that records the
// arguments passed to Tab so tests can verify the visible label.
type recordingWindowManager struct {
	gotURI  workspaceapi.URI
	gotIcon rune
	gotName string
}

func (m *recordingWindowManager) Focus() (browserapi.Window, error) { return nil, nil }
func (m *recordingWindowManager) Split(
	_ browserapi.Orientation, _ browserapi.Window, _ browserapi.Handler,
) (browserapi.Window, error) {
	return nil, nil
}
func (m *recordingWindowManager) Floating(
	_ browserapi.Floating, _ browserapi.FloatingConfig,
) (browserapi.Window, error) {
	return nil, nil
}
func (m *recordingWindowManager) Bar(_ browserapi.BarConfig, _ tui.Handler) error { return nil }
func (m *recordingWindowManager) Tab(
	uri workspaceapi.URI, icon rune, name string, h browserapi.Handler,
) (browserapi.Handler, error) {
	m.gotURI = uri
	m.gotIcon = icon
	m.gotName = name
	return h, nil
}
func (m *recordingWindowManager) SetWindowContent(_ browserapi.Window, _ browserapi.Handler) error {
	return nil
}
func (m *recordingWindowManager) CloseWindow(_ browserapi.Window) error { return nil }

// TestOpenChatTabUsesDialogueIDAsLabel verifies that the visible tab name is
// the dialogue's petname ID (e.g. "rolling-fox") and not the internal
// "rune-agent://<model>/<id>" URI.
func TestOpenChatTabUsesDialogueIDAsLabel(t *testing.T) {
	const dialogueID = "rolling-fox"
	uri, err := getModelUri(dialogueID, "gpt-5")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(uri.String(), "rune-agent://"),
		"precondition: URI must use the rune-agent scheme")

	wm := &recordingWindowManager{}
	_, err = openChatTab(wm, uri, dialogueID, nil)
	require.NoError(t, err)

	assert.Equal(t, dialogueID, wm.gotName,
		"visible tab label must be the dialogue ID, not the internal URI")
	assert.False(t, strings.HasPrefix(wm.gotName, "rune-agent://"),
		"tab label must not be the internal rune-agent:// URI")
	assert.Equal(t, uri, wm.gotURI, "URI must still be passed unchanged as tab identity")
}
