// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package modeless

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/api/textapi"
	"unstable.build/go-tui/api/workspaceapi"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/text/texttest"
)

func TestEditorDispatchFocus(t *testing.T) {
	cwd, err := workspaceapi.ParseURI("file:///")
	require.NoError(t, err)

	ed := Editor(
		// test wrapping/unwrapping of ifcs
		WithWorkspaceCommandRegistry(cwd, texttest.NopWorkspaceRegistry()),
	)
	content := "Clement"
	uri, err := workspaceapi.ParseURI("file:///Jolie")
	require.NoError(t, err)

	var h textapi.Handler
	ed.SubscribeEvents([]textapi.EventType{textapi.EventTypeOpen},
		text.FuncEventHandler(func(ctx context.Context, ev textapi.Event) bool {
			assert.Equal(t, textapi.EventTypeOpen, ev.Type)
			assert.Equal(t, content, ev.Content)
			assert.Equal(t, uri, ev.URI)
			h = ev.Resource
			return false
		}))

	var focusCalled int
	ed.SubscribeEvents([]textapi.EventType{textapi.EventTypeFocus},
		text.FuncEventHandler(func(ctx context.Context, ev textapi.Event) bool {
			focusCalled++
			assert.Equal(t, textapi.EventTypeFocus, ev.Type)
			assert.Equal(t, uri, ev.URI)
			assert.Equal(t, h, ev.Resource)
			return false
		}))

	buf := cell.NewBuffer()
	buf.WriteString(content)
	_, err = ed.Edit(uri, buf, false, false)
	require.NoError(t, err)

	assert.Equal(t, 1, focusCalled)
}

func TestEditorDispatchScroll(t *testing.T) {
	cfg := text.StatusBarConfig{
		Publisher: &texttest.TestEditor{},
		ScheduleNextTick: func(cb func()) bool {
			cb()
			return true
		},
	}
	ed := Editor(WithStatusBarConfig(true, cfg))
	buf := cell.NewBuffer()
	buf.WriteString("Daworg\nSurinach")
	h, err := ed.Edit(workspaceapi.URI{}, buf, false, false)
	require.NoError(t, err)

	h.Resize(2, 2)
	h.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})

	at := term.Coordinates{X: -1}
	ed.SubscribeEvents([]textapi.EventType{textapi.EventTypeScroll},
		text.FuncEventHandler(func(ctx context.Context, ev textapi.Event) bool {
			at = ev.Start
			return false
		}))

	h.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowUp})
	assert.Equal(t, term.Coordinates{}, at)

	at = term.Coordinates{X: -1}
	h.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
	assert.Equal(t, term.Coordinates{Y: 1}, at)

	at = term.Coordinates{X: -1}
	h.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
	assert.Equal(t, term.Coordinates{X: -1}, at)
}
