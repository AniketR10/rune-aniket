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
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/text"
)

// externalEditorStub is a minimal text.Editor that also reports itself
// as externally-managed via text.ExternallyManagedEditor.
type externalEditorStub struct{}

func (externalEditorStub) IsExternal() bool { return true }

func (externalEditorStub) Edit(
	context.Context, workspaceapi.URI, *cell.Buffer, bool, bool,
) (text.Handler, error) {
	return nil, errors.New("not used")
}

func (externalEditorStub) SubscribeCommand(textapi.CommandManual, text.CommandHandler) error {
	return errors.New("not supported")
}

func (externalEditorStub) RegisterREPLCommand(textapi.CommandManual, textapi.REPLHandler) error {
	return errors.New("not supported")
}

func (externalEditorStub) REPLCommands() []textapi.CommandManual { return nil }

func (externalEditorStub) UnsubscribeCommand(string) error { return errors.New("not supported") }

func (externalEditorStub) UnregisterREPLCommand(string) error {
	return errors.New("not supported")
}

func (externalEditorStub) Editor(workspaceapi.URI) (text.Handler, error) {
	return nil, errors.New("not supported")
}

func (externalEditorStub) SubscribeEvents([]textapi.EventType, text.EventHandler) error {
	return nil
}

func (externalEditorStub) UnsubscribeEvents(text.EventHandler) (bool, error) {
	return false, nil
}

// plainEditor is a minimal text.Editor that does NOT implement
// text.ExternallyManagedEditor; the negative case for
// isExternallyManagedEditor.
type plainEditor struct{}

func (plainEditor) Edit(
	context.Context, workspaceapi.URI, *cell.Buffer, bool, bool,
) (text.Handler, error) {
	return nil, errors.New("not used")
}

func (plainEditor) SubscribeCommand(textapi.CommandManual, text.CommandHandler) error {
	return nil
}

func (plainEditor) RegisterREPLCommand(textapi.CommandManual, textapi.REPLHandler) error {
	return nil
}

func (plainEditor) REPLCommands() []textapi.CommandManual         { return nil }
func (plainEditor) UnsubscribeCommand(string) error               { return nil }
func (plainEditor) UnregisterREPLCommand(string) error            { return nil }
func (plainEditor) Editor(workspaceapi.URI) (text.Handler, error) { return nil, nil }
func (plainEditor) SubscribeEvents([]textapi.EventType, text.EventHandler) error {
	return nil
}
func (plainEditor) UnsubscribeEvents(text.EventHandler) (bool, error) { return false, nil }

func TestIsExternallyManagedEditor(t *testing.T) {
	assert.True(t, isExternallyManagedEditor(externalEditorStub{}),
		"editor that returns IsExternal()=true must be detected")
	assert.False(t, isExternallyManagedEditor(plainEditor{}),
		"editor without ExternallyManagedEditor must report false")
	assert.False(t, isExternallyManagedEditor(nil),
		"nil editor must report false")
}

// TestSubscribeAllEventsSkipsAutoSaveForExternalEditor exercises the
// wiring guard in subscribeAllEvents: when the workspace's editor is
// externally managed, the autoSaver must not be constructed even when
// editor.auto_save is true. Otherwise byoe-driven external saves race
// the autoSaver and surface noisy ErrStaleData warnings.
func TestSubscribeAllEventsSkipsAutoSaveForExternalEditor(t *testing.T) {
	prev := autoSaverFactory
	t.Cleanup(func() { autoSaverFactory = prev })
	called := false
	autoSaverFactory = func(
		_ autoSaverFlusher, _ browserapi.Notifications,
		_ func(func()) bool, _ time.Duration,
	) *autoSaver {
		called = true
		return nil
	}

	cfg := ideConfig{cfg: map[string]any{
		"editor": map[string]any{
			"mode":      "byoe",
			"auto_save": true,
		},
	}, errors: map[string]error{}}

	h := &workspaceManagerHandler{}
	ex := &ex{ed: externalEditorStub{}}

	require.NoError(t, h.subscribeAllEvents(cfg, ex))
	assert.False(t, called,
		"autoSaverFactory must not be invoked for an external editor")
}
