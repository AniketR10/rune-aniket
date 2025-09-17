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

package plugin

import (
	"context"
	"errors"

	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/api/browserapi"
	"unstable.build/go-tui/api/config"
	"unstable.build/go-tui/api/workspaceapi"
	"unstable.build/go-tui/component/comptest"
	"unstable.build/go-tui/component/notifications"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/term/vte"
	"unstable.build/go-tui/workspace"
)

func TestPluginHandlerCursor(t *testing.T) {
	makeHandler := func(mock *handler.TestHandler) *Handler {
		h := new(Handler)
		h.initState(nopBrowser{}, "cmd", 100 /* width */, defaultConfig())
		h.liveHandler = mock
		h.Resize(100, 100)
		return h
	}
	t.Run("corrects coordinates past width-1 and height-1", func(t *testing.T) {
		h := makeHandler(&handler.TestHandler{CursorPos: term.Coordinates{X: 1000, Y: 1000}})
		pos, _, show := h.Cursor()
		require.True(t, show)
		assert.Equal(t, term.Coordinates{X: 99, Y: 99}, pos)
	})
	t.Run("corrects negative coordinates", func(t *testing.T) {
		h := makeHandler(&handler.TestHandler{CursorPos: term.Coordinates{X: -99, Y: -99}})
		pos, _, show := h.Cursor()
		require.True(t, show)
		assert.Equal(t, term.Coordinates{X: 0, Y: 0}, pos)
	})
}

func TestPluginHandler(t *testing.T) {
	if ci := os.Getenv("CI"); ci == "true" {
		// it's inherently impossible to know when sh will actually
		// have written in the vte's buffer; do not run
		// on constrained environemnts.
		t.SkipNow()
	}

	suite := []struct {
		description    string
		cmdAndArgs     string
		maxWidth       int
		drawnComponent string
		waitProcess    bool
	}{
		{
			description: "no cmd and args runs a shell by default",
			cmdAndArgs:  "",
			maxWidth:    4,
			waitProcess: false,
			drawnComponent: `
 ◦     sh   0s
$             
              
              
              
              `,
		},
		{
			description: "command with arg",
			cmdAndArgs:  "sleep 2",
			maxWidth:    4,
			waitProcess: true,
			drawnComponent: `
 ◦  sleep 2 0s
              
              
              
              
              `,
		},
		{
			description: "max width 0 doesn't panic",
			cmdAndArgs:  "sleep 2",
			maxWidth:    0,
			waitProcess: true,
			drawnComponent: `
 ◦  sleep 2 0s
              
              
              
              
              `,
		},
	}

	// important so test correctness doesn't depend on host
	shell := os.Getenv("SHELL")
	defer os.Setenv("SHELL", shell)
	os.Setenv("SHELL", "sh")

	ps1 := os.Getenv("PS1")
	os.Setenv("PS1", "$ ")
	defer os.Setenv("PS1", ps1)

	for _, test := range suite {
		t.Run(test.description, func(t *testing.T) {
			ctx := context.Background()
			tempDir, err := os.MkdirTemp("", "")
			require.NoError(t, err)
			uri, err := workspaceapi.ParseURI(filepath.Join("file://", tempDir))
			require.NoError(t, err)
			fileScheme, err := workspace.NewFileScheme(ctx, config.NopConfig(), uri)
			require.NoError(t, err)

			ch := make(chan struct{})
			cherr1 := make(chan error)
			cherr2 := make(chan error)
			cherr3 := make(chan error)
			waitInterrupt := term.FuncInterrupter(func(context.Context) error {
				select {
				case ch <- struct{}{}:
				default:
				}
				return nil
			})
			vteCfg := vte.DefaultConfig()
			vteCfg.Watcher = workspaceapi.ChanProcessWatcher(cherr2)
			h, err := New(nopBrowser{interrupt: waitInterrupt}, nopBrowser{}, fileScheme,
				fileScheme, nopBrowser{}, test.cmdAndArgs, test.maxWidth,
				WithFrame(false),
				// test order of watchers
				WithProcessWatcher(workspaceapi.ChanProcessWatcher(cherr1)),
				WithVTEConfig(vteCfg),
				WithProcessWatcher(workspaceapi.ChanProcessWatcher(cherr3)),
			)
			h.Resize(14, 6)

			w := term.NewStringWriter(14, 6)

			tests := []comptest.TestCase{
				{Action: nil, Expected: test.drawnComponent},
			}

			<-ch
			comptest.TestComponent(t, h, w, tests)
			if test.waitProcess {
				assert.NoError(t, <-cherr1)
				assert.NoError(t, <-cherr2)
				assert.NoError(t, <-cherr3)
			}
		})
	}
}

type nopBrowser struct {
	interrupt term.Interrupter
}

func (n nopBrowser) PublishEvent(ev term.Event) error {
	if ev.Type != term.EventInterrupt {
		return errors.New("unexpected event type")
	}
	if n.interrupt != nil {
		n.interrupt.Interrupt(context.Background())
	}
	return nil
}

func (n nopBrowser) Notify(notifications.Level, string, ...interface{}) (string, error) {
	return "", nil
}

func (n nopBrowser) NotifyOnce(notifications.Level, string, ...interface{}) (string, error) {
	return "", nil
}

func (n nopBrowser) Tab(
	uri workspaceapi.URI, icon rune, name string, h browserapi.Handler,
) (
	browserapi.Handler, error,
) {
	panic("should not be called")
}

func (n nopBrowser) SetTabName(workspaceapi.URI, string, term.Attributes) error {
	panic("should not be called")
}
