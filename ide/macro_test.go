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
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/handler/handlertest"
	"unstable.build/go-tui/text"
)

func TestMacroRecordAndEchoIntegration(t *testing.T) {
	tests := []struct {
		name string
		run  func(*testing.T, *macroIntegrationHarness)
	}{
		{
			name: "recorded insert keys echo back through event loop",
			run: func(t *testing.T, tc *macroIntegrationHarness) {
				handleKeys(t, tc, ":record<space>a<enter>ione<esc>:record<space>a<enter>")
				require.Equal(t, "one", editorString(t, tc.ide))

				handleKeys(t, tc, "Go<esc>:echo<space>{register}a<enter>")
				tc.drainPublishedEvents(t)
				require.Equal(t, "one\none", editorString(t, tc.ide))
			},
		},
		{
			name: "different registers hold different key sequences",
			run: func(t *testing.T, tc *macroIntegrationHarness) {
				handleKeys(t, tc, ":record<space>a<enter>ia<esc>:record<space>a<enter>")
				handleKeys(t, tc, "Go<esc>:record<space>b<enter>ib<esc>:record<space>b<enter>")
				require.Equal(t, "a\nb", editorString(t, tc.ide))

				handleKeys(t, tc, "Go<esc>:echo<space>{register}a<enter>")
				tc.drainPublishedEvents(t)
				handleKeys(t, tc, "Go<esc>:echo<space>{register}b<enter>")
				tc.drainPublishedEvents(t)

				require.Equal(t, "a\nb\na\nb", editorString(t, tc.ide))
			},
		},
		{
			name: "vim style qq q atq workflow works",
			run: func(t *testing.T, tc *macroIntegrationHarness) {
				handleKeys(t, tc, "qqabound<esc>q@q")
				tc.drainPublishedEvents(t)

				require.Equal(t, "boundbound", editorString(t, tc.ide))
			},
		},
		{
			name: "recorded command prompt command replays via echo",
			run: func(t *testing.T, tc *macroIntegrationHarness) {
				handleKeys(t, tc, ":record<space>a<enter>:tabrename<space>macroed<enter>:record<space>a<enter>")
				handleKeys(t, tc, ":notificationcloseall<enter>")
				require.Equal(t, `┌──────────────────────────────────────┐
│o macroed                             │
├──────────────────────────────────────┤
│▐                                     │
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
│                                NORMAL│
└──────────────────────────────────────┘`, drawIDE(t, tc))
				handleKeys(t, tc, ":tabrename<space>reset<enter>")
				handleKeys(t, tc, ":notificationcloseall<enter>")
				require.Equal(t, `┌──────────────────────────────────────┐
│o reset                               │
├──────────────────────────────────────┤
│▐                                     │
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
│                                NORMAL│
└──────────────────────────────────────┘`, drawIDE(t, tc))

				handleKeys(t, tc, ":echo<space>{register}a<enter>")
				tc.drainPublishedEvents(t)
				handleKeys(t, tc, ":notificationcloseall<enter>")

				require.Equal(t, `┌──────────────────────────────────────┐
│o macroed                             │
├──────────────────────────────────────┤
│▐                                     │
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
│                                NORMAL│
└──────────────────────────────────────┘`, drawIDE(t, tc))
			},
		},
		{
			name: "record in one workspace and replay in another workspace",
			run: func(t *testing.T, tc *macroIntegrationHarness) {
				handleKeys(t, tc, ":record<space>a<enter>iws<esc>:record<space>a<enter>")
				require.Equal(t, "ws", editorString(t, tc.ide))

				secondWorkspace := t.TempDir()
				secondFile := filepath.Join(secondWorkspace, "second.txt")
				require.NoError(t, os.WriteFile(secondFile, nil, 0666))
				secondURI, err := workspaceapi.CurrentUserHostURI(secondFile)
				require.NoError(t, err)
				handleKeys(t, tc, ":workspacenew<space>"+secondWorkspace+"<enter>")
				withLockedIDE(t, tc.mu, func() {
					require.NoError(t, tc.ide.Open(secondURI))
				})

				handleKeys(t, tc, ":echo<space>{register}a<enter>")
				tc.drainPublishedEvents(t)

				require.Equal(t, "ws", editorString(t, tc.ide))
			},
		},
		{
			name: "nested record command is rejected and not recorded",
			run: func(t *testing.T, tc *macroIntegrationHarness) {
				handleKeys(t, tc, ":record<space>a<enter>ix<esc>:record<space>b<enter>ay<esc>:record<space>a<enter>")
				require.Equal(t, "xy", editorString(t, tc.ide))

				handleKeys(t, tc, "Go<esc>:echo<space>{register}a<enter>")
				tc.drainPublishedEvents(t)

				require.Equal(t, "xy\nxy", editorString(t, tc.ide))
			},
		},
		{
			name: "prompt-open replay can be recorded without recording the stop command",
			run: func(t *testing.T, tc *macroIntegrationHarness) {
				handleKeys(t, tc, ":record<space>a<enter>:tabrename<space>macroed<enter>:record<space>a<enter>")
				handleKeys(t, tc, ":tabrename<space>reset<enter>:notificationcloseall<enter>")

				handleKeys(t, tc, ":record<space>b<enter>:echo<space>{register}a<enter>")
				tc.drainPublishedEvents(t)
				handleKeys(t, tc, ":record<space>b<enter>:notificationcloseall<enter>")

				require.Equal(t, `┌──────────────────────────────────────┐
│o macroed                             │
├──────────────────────────────────────┤
│▐                                     │
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
│                                NORMAL│
└──────────────────────────────────────┘`, drawIDE(t, tc))

				handleKeys(t, tc, ":tabrename<space>reset<enter>:echo<space>{register}b<enter>")
				tc.drainPublishedEvents(t)
				handleKeys(t, tc, ":notificationcloseall<enter>")

				require.Equal(t, `┌──────────────────────────────────────┐
│o macroed                             │
├──────────────────────────────────────┤
│▐                                     │
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
│                                NORMAL│
└──────────────────────────────────────┘`, drawIDE(t, tc))
			},
		},
		{
			name: "echoing the active recording register fails fast instead of looping",
			run: func(t *testing.T, tc *macroIntegrationHarness) {
				handleKeys(t, tc, ":record<space>q<enter>iok<esc>:echo<space>{register}q<enter>:notificationcloseall<enter>")
				tc.drainPublishedEvents(t)

				require.Equal(t, "ok", editorString(t, tc.ide))

				handleKeys(t, tc, ":record<space>q<enter>")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tc := newMacroIntegrationHarness(t)
			test.run(t, tc)
		})
	}
}

type macroIntegrationHarness struct {
	mu        *sync.Mutex
	events    chan term.Event
	ide       *IDE
	h         *macroTestHandler
	scheduler *queuedScheduler
}

type macroTestHandler struct {
	tc *macroIntegrationHarness
}

func (h *macroTestHandler) Handle(ev term.Event) (bool, bool) {
	h.tc.mu.Lock()
	quit, handled := h.tc.ide.root.Handle(ev)
	h.tc.mu.Unlock()
	flushMacroHarness(h.tc)
	return quit, handled
}

func (h *macroTestHandler) Draw(w term.Writer) {
	flushMacroHarness(h.tc)
	h.tc.mu.Lock()
	defer h.tc.mu.Unlock()
	h.tc.ide.root.Draw(w)
}

func (h *macroTestHandler) Resize(width, height int) {
	h.tc.mu.Lock()
	defer h.tc.mu.Unlock()
	h.tc.ide.root.Resize(width, height)
}

func (h *macroTestHandler) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	h.tc.mu.Lock()
	defer h.tc.mu.Unlock()
	return h.tc.ide.root.Cursor()
}

func (h *macroTestHandler) Selection() (string, bool) {
	h.tc.mu.Lock()
	defer h.tc.mu.Unlock()
	return h.tc.ide.root.Selection()
}

func newMacroIntegrationHarness(t *testing.T) *macroIntegrationHarness {
	t.Helper()
	dir := t.TempDir()
	cfgName := macroTestConfig(t, dir)
	fileName := filepath.Join(dir, "macro.txt")
	require.NoError(t, os.WriteFile(fileName, nil, 0666))
	uri, err := workspaceapi.CurrentUserHostURI(fileName)
	require.NoError(t, err)

	mu := new(sync.Mutex)
	events := make(chan term.Event, 4096)
	scheduler := newQueuedScheduler()
	i, err := New(dir, cfgName, dir,
		WithLocker(mu),
		WithScheduleNextTick(scheduler.ScheduleNextTick),
		WithPublishEvent(func(ev term.Event) bool {
			if ev.Type == term.EventInterrupt {
				return true
			}
			select {
			case events <- ev:
			default:
				t.Fatalf("event queue is full")
			}
			return true
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, i.Close()) })
	_ = i.Ready()
	tc := &macroIntegrationHarness{mu: mu, events: events, ide: i, scheduler: scheduler}
	tc.h = &macroTestHandler{tc: tc}

	withLockedIDE(t, mu, func() {
		require.NoError(t, i.Open(uri))
		i.root.Resize(40, 12)
	})
	return tc
}

func (h *macroIntegrationHarness) drainPublishedEvents(t *testing.T) {
	t.Helper()
	drainPublishedEvents(t, h, h.events)
}

func drawIDE(t *testing.T, tc *macroIntegrationHarness) string {
	t.Helper()
	return handlertest.DrawHandler(tc.h, 40, 12)
}

func macroTestConfig(t *testing.T, dir string) string {
	t.Helper()
	name := filepath.Join(dir, "rune.yaml")
	require.NoError(t, os.WriteFile(name, []byte(`
editor:
  mode: modal
command:
  show_manual_after: 1h
  key: ":"
  key_bindings:
    qq: record q
    '@q': echo {register}q
`), 0666))
	return name
}

func handleKeys(t *testing.T, tc *macroIntegrationHarness, seq string) {
	t.Helper()
	keys, err := term.ParseKeys(seq)
	require.NoError(t, err)
	for _, key := range keys {
		tc.h.Handle(term.Event{
			Type: term.EventKey,
			Ch:   key.Ch,
			Mod:  key.Mod,
			Key:  key.Key,
		})
		flushMacroHarness(tc)
	}
}

func drainPublishedEvents(t *testing.T, tc *macroIntegrationHarness, events <-chan term.Event) {
	t.Helper()
	for range 4096 {
		select {
		case ev := <-events:
			if ev.UserFunc != nil {
				withLockedIDE(t, tc.mu, func() {
					ev.UserFunc()
				})
			} else {
				tc.h.Handle(ev)
			}
			flushMacroHarness(tc)
		default:
			return
		}
	}
	t.Fatalf("published event drain limit exceeded")
}

func editorString(t *testing.T, i *IDE) string {
	t.Helper()
	win, err := i.Browser().Focus()
	require.NoError(t, err)
	h, err := win.Content()
	require.NoError(t, err)
	if tab, ok := h.(*browser.Tab); ok {
		h = tab.Handler()
	}
	ed, ok := h.(text.Handler)
	require.True(t, ok)
	return ed.CellView().String()
}

func withLockedIDE(t *testing.T, mu *sync.Mutex, fn func()) {
	t.Helper()
	mu.Lock()
	defer mu.Unlock()
	fn()
}
func flushMacroHarness(tc *macroIntegrationHarness) {
	if tc == nil {
		return
	}
	if tc.scheduler != nil {
		tc.scheduler.Flush(tc.mu)
	}
	tc.mu.Lock()
	handler := tc.ide.workspaceHandler.focusHandler()
	ex, ok := handler.(*ex)
	if !ok {
		ex = handler.(*workspaceHandler).ex
	}
	cmd := ex.cmd
	tc.mu.Unlock()
	if cmd != nil {
		cmd.Cancel()
	}
	ex.Wait()
}
