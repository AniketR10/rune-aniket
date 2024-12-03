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

package ide

import (
	"context"
	"errors"
	"fmt"
	"io"

	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	"unstable.build/go-tui"
	browserapi "unstable.build/go-tui/api/browser"
	"unstable.build/go-tui/api/config"
	schemeapi "unstable.build/go-tui/api/scheme"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/browser/browsertest"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/clipboard"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/component/notifications"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/handler/handlertest"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/term/vte"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/text/texttest"
	"unstable.build/go-tui/workspace"
)

// handlertest.TestHandlerSequence maps ':' characters to the following event
// this is to work around ex's assumptions on underlying handler.
var testCommandKey = term.KeyComb{Ch: '\\', Mod: term.ModCtrl}

type browserConstructor func(ed text.Editor, opts ...text.Option) (tui.Handler, browser.Browser, error)

type testFileBuffer struct {
	flushErr error
	closeErr error
	closed   bool
}

func (t *testFileBuffer) Flush() error {
	return t.flushErr
}

func (t *testFileBuffer) Close() error {
	t.closed = true
	return t.closeErr
}

type testLoader struct {
	buf *testFileBuffer
}

func (w *testLoader) Remove(string) error {
	return nil
}
func (w *testLoader) Close() error {
	return nil
}
func (w *testLoader) StartCommand(ctx context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
	panic("unimplemented")
}

func (w *testLoader) Signal(workspaceapi.Pid, syscall.Signal) error {
	panic("unimplemented")
}

func (w *testLoader) Load(
	filePath workspaceapi.URI, buf *cell.Buffer,
	swapDir workspaceapi.URI, readOnly bool,
) (
	workspace.FlusherCloser, error,
) {
	if w.buf != nil {
		return w.buf, nil
	}
	return &testFileBuffer{}, nil
}

func (w *testLoader) Recover(
	filePath, swapFilePath workspaceapi.URI,
	buf *cell.Buffer, force bool,
) (workspace.FlusherCloser, error) {
	return w.Load(filePath, buf, swapFilePath, false)
}

func (w *testLoader) ReadDir(name string) ([]os.DirEntry, error) {
	return nil, nil
}

func (w *testLoader) Stat(name string) (os.FileInfo, error) {
	return testFileInfo{name: name}, nil
}

func (w *testLoader) Open(
	path string, flag int, perm os.FileMode,
) (workspaceapi.File, *workspaceapi.Error) {
	panic("unimplemented")
}
func (w *testLoader) NewPty(context.Context) (workspaceapi.Pty, error) {
	panic("unimplemented")
}

func (w *testLoader) SetPtySize(p workspaceapi.Pty, width, height int) error {
	panic("unimplemented")
}

type testFileInfo struct {
	name string
}

func (t testFileInfo) Name() string {
	return t.name
}

func (t testFileInfo) IsDir() bool {
	return t.name == "/" || t.name == "" || t.name == "."
}

func (t testFileInfo) ModTime() time.Time {
	return time.Time{}
}

func (t testFileInfo) Mode() os.FileMode {
	return 0
}

func (t testFileInfo) Size() int64 {
	return 0
}

func (t testFileInfo) Sys() any {
	return nil
}

func (w *testLoader) URI(path string) (workspaceapi.URI, error) {
	return workspaceapi.CurrentUserHostURI(path)
}

func TestBrowserHandlerDraw(t *testing.T) {
	testBrowserHandlerDraw(t, func(ed text.Editor, opts ...text.Option) (tui.Handler, browser.Browser, error) {
		b := newExForTesting(t, ed, opts...)
		return b, b.Browser(), nil
	})
}

func TestComponentOpenEditorIntegration(t *testing.T) {
	b := newExForTesting(t, texttest.NopEditor(),
		text.WithCommandKey(testCommandKey),
		text.WithCommandOverlayConfig(testCommandOverlayConfig()),
	)
	uri, err := workspaceapi.ParseURI("file:///bugz")
	require.NoError(t, err)

	tab, err := b.editFileURI(uri, b.invokeWindow())
	require.NoError(t, err)
	assert.NotPanics(t, func() {
		_ = tab.Handler().(text.Handler)
	})
	assert.NoError(t, b.Close())
}

func testBrowserHandlerDraw(t *testing.T, constructor browserConstructor) {
	cases := []handlertest.SequenceTestCase{
		{"a",
			`┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│                  │
│                  │
│                  │
│                  │
│                  │
└──────────────────┘`},
		{":edit cabin.go>",
			`┌──────────────────┐
│cabin.go          │
├──────────────────┤
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`},
		{"a",
			`┌──────────────────┐
│cabin.go          │
├──────────────────┤
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`},
		{":edit /tmp/other.go>",
			`┌──────────────────┐
│cabin.go  other.go│
├──────────────────┤
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`},
		{"#", // simulates ctrl-h
			`┌──────────────────┐
│cabin.go  other.go│
├──────────────────┤
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`},
		{"#", // simulates ctrl-h
			`┌──────────────────┐
│cabin.go  other.go│
├──────────────────┤
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`},
		{"$", // simulates ctrl-l
			`┌──────────────────┐
│cabin.go  other.go│
├──────────────────┤
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
└──────────────────┘`},
		{"$", // simulates ctrl-l
			`┌──────────────────┐
│cabin.go  other.go│
├──────────────────┤
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
└──────────────────┘`},
		{":cTab>",
			`┌──────────────────┐
│cabin.go          │
├──────────────────┤
│DDDDDDDDDDDDDDDDDD│
│DDDDDDDDDDDDDDDDDD│
│DDDDDDDDDDDDDDDDDD│
│DDDDDDDDDDDDDDDDDD│
│DDDDDDDDDDDDDDDDDD│
│DDDDDDDDDDDDDDDDDD│
└──────────────────┘`},
		{":wq!^^^^^",
			`┌──────────────────┐
│cabin.go          │
├──────────────────┤
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
└──────────────────┘`},
		{":<",
			`┌──────────────────┐
│cabin.go          │
├──────────────────┤
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
└──────────────────┘`},
		{":edit other.go>1111",
			`┌──────────────────┐
│cabin.go  other.go│
├──────────────────┤
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
└──────────────────┘`},
		{"$$##",
			`┌──────────────────┐
│cabin.go  other.go│
├──────────────────┤
│GGGGGGGGGGGGGGGGGG│
│GGGGGGGGGGGGGGGGGG│
│GGGGGGGGGGGGGGGGGG│
│GGGGGGGGGGGGGGGGGG│
│GGGGGGGGGGGGGGGGGG│
│GGGGGGGGGGGGGGGGGG│
└──────────────────┘`},
	}
	bh, b, err := constructor(texttest.NopEditor(),
		text.WithCommandKey(testCommandKey),
		text.WithCommandKeyBinding(term.KeyComb{Ch: '4'}, []string{"closeWindow"}),
		text.WithNotificationsConfig(notificationsConfig()),
	)
	require.NoError(t, err)

	handlertest.TestHandlerSequence(t, bh, 20, 10, cases)

	focus, err := b.Focus()
	require.NoError(t, err)

	win, err := b.Split(browserapi.OrientationLeft, focus, browsertest.NewTestHandler())
	require.NoError(t, err)

	focus = win

	h := browsertest.NewTestHandler()
	h.Ch = 'Z' // helps identify in tests

	_, err = b.Split(browserapi.OrientationBottom, focus, h)
	require.NoError(t, err)

	cases = []handlertest.SequenceTestCase{
		{"",
			`┌──────────────────┐
│cabin.go  other.go│
├────────┐┌────────┤
│AAAAAAAA││GGGGGGGG│
│AAAAAAAA││GGGGGGGG│
└────────┘│GGGGGGGG│
┌────────┐│GGGGGGGG│
│ZZZZZZZZ││GGGGGGGG│
│ZZZZZZZZ││GGGGGGGG│
└────────┘└────────┘`},
		{":<111111111",
			`┌──────────────────┐
│cabin.go  other.go│
├────────┐┌────────┤
│AAAAAAAA││GGGGGGGG│
│AAAAAAAA││GGGGGGGG│
└────────┘│GGGGGGGG│
┌────────┐│GGGGGGGG│
│cccccccc││GGGGGGGG│
│cccccccc││GGGGGGGG│
└────────┘└────────┘`},
	}

	handlertest.TestHandlerSequence(t, bh, 20, 10, cases)

	var closed int
	hx := browsertest.NewTestHandler()
	hx.Ch = '$'
	hx.CloseCallback = func() error { closed++; return nil }
	require.NoError(t, focus.SetContent(hx))

	cases = []handlertest.SequenceTestCase{
		{"",
			`┌──────────────────┐
│cabin.go  other.go│
├────────┐┌────────┤
│$$$$$$$$││GGGGGGGG│
│$$$$$$$$││GGGGGGGG│
└────────┘│GGGGGGGG│
┌────────┐│GGGGGGGG│
│cccccccc││GGGGGGGG│
│cccccccc││GGGGGGGG│
└────────┘└────────┘`},
	}

	handlertest.TestHandlerSequence(t, bh, 20, 10, cases)

	require.NoError(t, win.Close())
	require.NoError(t, focus.Close())

	cases = []handlertest.SequenceTestCase{
		// test CommandKeyBindings
		{"4$$$",
			`┌──────────────────┐
│cabin.go  other.go│
├──────────────────┤
│HHHHHHHHHHHHHHHHHH│
│HHHHHHHHHHHHHHHHHH│
│HHHHHHHHHHHHHHHHHH│
│HHHHHHHHHHHHHHHHHH│
│HHHHHHHHHHHHHHHHHH│
│HHHHHHHHHHHHHHHHHH│
└──────────────────┘`},
		{":closeAllT>:edit other.go>bcde####",
			`┌──────────────────┐
│other.go          │
├──────────────────┤
│IIIIIIIIIIIIIIIIII│
│IIIIIIIIIIIIIIIIII│
│IIIIIIIIIIIIIIIIII│
│IIIIIIIIIIIIIIIIII│
│IIIIIIIIIIIIIIIIII│
│IIIIIIIIIIIIIIIIII│
└──────────────────┘`},
	}
	handlertest.TestHandlerSequence(t, bh, 20, 10, cases)

	cases = []handlertest.SequenceTestCase{
		{"", `┌──┐
│..│
├II┤
IIII`},
	}
	handlertest.TestHandlerSequence(t, bh, 4, 4, cases)

	require.NoError(t, b.Notify(notifications.LevelInfo, "wasup: %s", "Z"))
	cases = []handlertest.SequenceTestCase{
		{"",
			`┌────┌─────────────┐
│othe│ wasup: Z    │
├────└─────────────┘
│IIIIIIIIIIIIIIIIII│
│IIIIIIIIIIIIIIIIII│
│IIIIIIIIIIIIIIIIII│
│IIIIIIIIIIIIIIIIII│
│IIIIIIIIIIIIIIIIII│
│IIIIIIIIIIIIIIIIII│
└──────────────────┘`},
	}
	handlertest.TestHandlerSequence(t, bh, 20, 10, cases)

	uri, err := workspaceapi.ParseURI("file:///bugz")
	require.NoError(t, err)
	nh, err := b.Open(uri)
	require.NoError(t, err)
	focus, err = b.Focus()
	require.NoError(t, err)
	err = focus.SetContent(nh)
	require.NoError(t, err)

	cases = []handlertest.SequenceTestCase{
		{"b",
			`┌────┌─────────────┐
│othe│ wasup: Z    │
├────└─────────────┘
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`},
		{":3>",
			`┌────┌─────────────┐
│othe│ wasup: Z    │
├────└─────────────┘
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│▐BBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`},
		{":0>",
			`┌────┌─────────────┐
│othe│ wasup: Z    │
├────└─────────────┘
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`},
	}
	handlertest.TestHandlerSequence(t, bh, 20, 10, cases)

	floating1, err := b.Floating(browsertest.NewTestFloating(4, 2),
		component.FloatingConfig{Offset: term.Coordinates{X: 1, Y: 1}})
	require.NoError(t, err)

	focus, err = b.Focus()
	require.NoError(t, err)

	// should not be able to split over a floating window, which is currently in focus
	_, err = b.Split(browserapi.OrientationTop, focus, browsertest.NewTestHandler())
	require.Error(t, err)
	cases = []handlertest.SequenceTestCase{
		{"",
			`┌────┌─────────────┐
│othe│ wasup: Z    │
├────└─────────────┘
│┌────┐BBBBBBBBBBBB│
││AAAA│BBBBBBBBBBBB│
││AAAA│BBBBBBBBBBBB│
│└────┘BBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`},
		{":e!>", // test reload non file
			`┌────┌─────────────┐
│othe│ not a file  │
├────└─────────────┘
│┌───┌─────────────┐
││AAA│ wasup: Z    │
││AAA└─────────────┘
│└────┘BBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`},
	}

	handlertest.TestHandlerSequence(t, bh, 20, 10, cases)

	require.NoError(t, floating1.Close())

	cases = []handlertest.SequenceTestCase{
		{"",
			`┌────┌─────────────┐
│othe│ not a file  │
├────└─────────────┘
│BBBB┌─────────────┐
│BBBB│ wasup: Z    │
│BBBB└─────────────┘
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`},
	}
	handlertest.TestHandlerSequence(t, bh, 20, 10, cases)

	o := browserapi.BarConfig{Size: 1, Orientation: browserapi.OrientationTop}
	for i := 0; i < 4; i++ {
		b1 := browsertest.NewTestHandler()
		b1.Ch = rune(strconv.Itoa(i)[0])
		err = b.Bar(o, b1)
		require.NoError(t, err)
		o.Orientation++
	}

	// test case for issue #27
	cases = []handlertest.SequenceTestCase{
		{":edit ait^^^aix^^^^ airsoft.map>",
			`┌──────────────────────────────────┌─────────────┐
│other.go  bugz  airsoft.map       │ not a file  │
├──────────────────────────────────└─────────────┘
│0000000000000000000000000000000000┌─────────────┐
├─┬────────────────────────────────│ wasup: Z    │
│2│AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA└─────────────┘
│2│AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA│3│
├─┴────────────────────────────────────────────┴─┤
│111111111111111111111111111111111111111111111111│
└────────────────────────────────────────────────┘`},
	}
	handlertest.TestHandlerSequence(t, bh, 50, 10, cases)

	assert.NoError(t, bh.(io.Closer).Close())
	assert.NoError(t, b.Close())
	assert.Equal(t, 1, closed)
}

func assertHandled(
	t *testing.T, h *browsertest.TestHandler, startingRune rune, exit, handled bool,
) {
	// test handler increments the character that it displays next
	// upon handling a new event
	require.False(t, exit)
	require.True(t, handled)
	assert.NotEqual(t, startingRune, h.Ch)
}

func TestBrowserHandlerInterrupts(t *testing.T) {
	t.Run("Interrupt calls interrupt handle", func(t *testing.T) {
		var wg sync.WaitGroup
		opts := []text.Option{text.WithEventPublisher(
			func(ev term.Event) bool {
				assert.Equal(t, term.EventInterrupt, ev.Type)
				wg.Done()
				return true
			},
		),
			text.WithCommandOverlayConfig(testCommandOverlayConfig()),
		}
		browser := newExForTesting(t, texttest.NopEditor(), opts...)
		defer browser.Close()

		wg.Add(1)
		browser.Browser().PublishEvent(term.Event{Type: term.EventInterrupt})

		wg.Wait()
	})
	t.Run("SendEventNone calls interrupt handle", func(t *testing.T) {
		var wg sync.WaitGroup
		opts := []text.Option{text.WithEventPublisher(func(ev term.Event) bool {
			assert.Equal(t, term.EventNone, ev.Type)
			wg.Done()
			return true
		}),
			text.WithCommandOverlayConfig(testCommandOverlayConfig()),
		}
		browser := newExForTesting(t, texttest.NopEditor(), opts...)
		defer browser.Close()

		wg.Add(1)
		browser.Browser().PublishEvent(term.Event{Type: term.EventNone})

		wg.Wait()
	})
}

func TestMultipleFilesStartup(t *testing.T) {
	cases := []handlertest.SequenceTestCase{
		{"",
			`┌──────────────────┐
│cabin.go  wi.go   │
├──────────────────┤
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`},
		{"aa",
			`┌──────────────────┐
│cabin.go  wi.go   │
├──────────────────┤
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
└──────────────────┘`},
		{"#", // simulates ctrl-h
			`┌──────────────────┐
│cabin.go  wi.go   │
├──────────────────┤
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`},
		{"#:reloadFile>",
			`┌──────────────────┐
│wi.go  cabin.go   │
├──────────────────┤
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`},
	}

	file1, err := workspaceapi.ParseURI("file:///cabin.go")
	require.NoError(t, err)
	file2, err := workspaceapi.ParseURI("file:///wi.go")
	require.NoError(t, err)
	opts := []text.Option{
		text.WithFile(file1),
		text.WithFile(file2),
		text.WithCommandKey(testCommandKey),
		text.WithCommandOverlayConfig(testCommandOverlayConfig()),
	}
	mockBuf := testFileBuffer{}
	workspace := testLoader{buf: &mockBuf}
	b := newExForTestingWithWorkspace(t, &workspace, texttest.NopEditor(),
		vte.DefaultConfig(), nopPublishEvent, clipboard.NewInMemory(), opts...)
	defer b.Close()

	handlertest.TestHandlerSequence(t, b, 20, 10, cases)

}

func TestWriteExclamationNoQuit(t *testing.T) {
	cases := []handlertest.SequenceTestCase{
		{":w!>",
			`┌──────────────────┐
│Cannot save       │
│this buffer       │
└──────────────────┘
│                  │
│                  │
│                  │
│                  │
│                  │
└──────────────────┘`},
	}
	opts := []text.Option{
		text.WithCommandKey(testCommandKey),
		text.WithCommandOverlayConfig(testCommandOverlayConfig()),
	}
	mockBuf := testFileBuffer{}
	workspace := testLoader{buf: &mockBuf}
	b := newExForTestingWithWorkspace(t, &workspace, texttest.NopEditor(),
		vte.DefaultConfig(), nopPublishEvent, clipboard.NewInMemory(), opts...)
	defer b.Close()

	handlertest.TestHandlerSequence(t, b, 20, 10, cases)
	assert.False(t, b.exit)
}

func TestBrowserCloseLastWindow(t *testing.T) {
	cases := []handlertest.SequenceTestCase{
		{":closeWindow>",
			`┌──────────────────┐
│Cannot close      │
│last tiled        │
│window            │
└──────────────────┘
│                  │
│                  │
│                  │
│                  │
└──────────────────┘`},
	}
	opts := []text.Option{
		text.WithCommandKey(testCommandKey),
		text.WithCommandOverlayConfig(testCommandOverlayConfig()),
	}
	mockBuf := testFileBuffer{}
	workspace := testLoader{buf: &mockBuf}
	b := newExForTestingWithWorkspace(t, &workspace, texttest.NopEditor(),
		vte.DefaultConfig(), nopPublishEvent, clipboard.NewInMemory(), opts...)
	defer b.Close()

	handlertest.TestHandlerSequence(t, b, 20, 10, cases)
	assert.False(t, b.exit)
}

func TestExCommandResponsive(t *testing.T) {
	cases := []handlertest.SequenceTestCase{
		{":edit",
			`                    
                    
                    
                    
edit▐               
edit                
                    
                    
                    
                    `},
		{":eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
			`                    
                    
                    
                    
eeeeeeeeeeeeeeeee   
eeeeeeeeeeeeeeeee   
eeeeeeeeeeeeeeeee   
eeeeeeeeeeeeeeeee   
eeeeeeeeeeeeeeeee   
eeeeeeeeeeeeeeee▐   `},
		{":edit eeeeeeeeeeeeeeeeeeeeeeeee",
			`                    
                    
                    
                    
edit eeeeeeeeeeee   
eeeeeeeeeeeee▐      
                    
                    
                    
                    `},
	}

	var closeFns []func() error
	fn := func(t *testing.T) tui.Handler {
		opts := []text.Option{
			text.WithCommandKey(testCommandKey),
			text.WithWindowManagerConfig(handler.WindowManagerConfig{
				WindowManagerConfig: component.WindowManagerConfig{Frame: false}}),
			text.WithCommandOverlayConfig(testCommandOverlayConfig()),
		}
		b := newExForTesting(t, texttest.NopEditor(), opts...)
		closeFns = append(closeFns, b.Close)
		return b
	}
	handlertest.TestHandlerIsolated(t, fn, 20, 10, cases)
	for _, close := range closeFns {
		close()
	}
}

func TestExKeySequence(t *testing.T) {
	cases := []handlertest.SequenceTestCase{
		{"zgl",
			`┌──────────────────┐
│10k.go  button.go │
├──────────────────┤
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`},
		{"g",
			`┌──────────────────┐
│10k.go  button.go │
├──────────────────┤
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`},
		{"go",
			`┌──────────────────┐
│10k.go  button.go │
├──────────────────┤
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
└──────────────────┘`},
		// 2 seconds of wait should be plenty for sequencer to deem 'g' sequence
		// stale and re-issue event.
		{"g____________________",
			`┌──────────────────┐
│10k.go  button.go │
├──────────────────┤
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`},
		{"gg",
			`┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│                  │
│                  │
│                  │
│                  │
│                  │
└──────────────────┘`},
	}

	var closeFns []func() error
	fn := func(t *testing.T) tui.Handler {
		var mu sync.Mutex
		file1, err := workspaceapi.ParseURI("file:///10k.go")
		require.NoError(t, err)
		file2, err := workspaceapi.ParseURI("file:///button.go")
		require.NoError(t, err)
		opts := []text.Option{
			text.WithFile(file1),
			text.WithFile(file2),
			text.WithCommandKey(testCommandKey),
			text.WithCommandSequenceBinding(handler.Sequence{
				First: term.KeyComb{Ch: 'g'},
				Last:  term.KeyComb{Ch: 'l'},
			}, []string{"nextTab"}),
			text.WithCommandSequenceBinding(handler.Sequence{
				First: term.KeyComb{Ch: 'g'},
				Last:  term.KeyComb{Ch: 'g'},
			}, []string{"closeAllTabs"}),
			text.WithSequencerTimeout(1 * time.Second),
			text.WithCommandOverlayConfig(testCommandOverlayConfig()),
		}
		ex := new(ex)
		ex.syncCommandPrompt = true
		require.NoError(t, ex.init(texttest.NopEditor(), &testLoader{}, document.NewInMemoryService(),
			vte.DefaultConfig(), func(ev term.Event) bool {
				// do not confuse interrupt from list with sequence re-issue commands
				if ev.Type == term.EventInterrupt {
					return true
				}
				mu.Lock()
				defer mu.Unlock()
				ex.Handle(ev)
				return true
			}, clipboard.NewInMemory(), opts...))
		ex.subscribeCommands()
		b := testEx{ex}
		closeFns = append(closeFns, func() error {
			mu.Lock()
			defer mu.Unlock()
			return b.Close()
		})
		return handler.Sync(&mu, b)
	}
	handlertest.TestHandlerIsolated(t, fn, 20, 10, cases)
	for _, close := range closeFns {
		close()
	}
}

func TestExTabIntegration(t *testing.T) {
	cases := []handlertest.SequenceTestCase{
		{"",
			`┌──────────────────┐
│Fieshta  Pahty    │
├──────────────────┤
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`},
		{":closeAllTabs>",
			`┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│                  │
│                  │
│                  │
│                  │
│                  │
└──────────────────┘`},
	}

	var closeFns []func() error
	fn := func(t *testing.T) tui.Handler {
		opts := []text.Option{
			text.WithCommandKey(testCommandKey),
			text.WithCommandOverlayConfig(testCommandOverlayConfig()),
		}
		b := newExForTesting(t, texttest.NopEditor(), opts...)
		uri1, err := workspaceapi.ParseURI("file:///Fieshta")
		require.NoError(t, err)
		uri2, err := workspaceapi.ParseURI("file:///Pahty")
		require.NoError(t, err)
		tab, err := b.comp.Tab(uri1, "Fieshta", browsertest.NewTestHandler())
		require.NoError(t, err)
		_, err = b.comp.Tab(uri2, "Pahty", browsertest.NewTestHandler())
		require.NoError(t, err)
		focus, err := b.comp.Focus()
		require.NoError(t, err)
		require.NoError(t, focus.SetContent(tab))
		closeFns = append(closeFns, b.Close)
		return b
	}
	handlertest.TestHandlerIsolated(t, fn, 20, 10, cases)
	for _, close := range closeFns {
		close()
	}
}

func TestExExit(t *testing.T) {
	commands := []string{
		"writeQuit",
		"writeForceQuit!",
		"forceQuit!",
		"quit",
	}

	for _, cmd := range commands {
		t.Run(fmt.Sprintf("ex exits %s command is issued", cmd), func(t *testing.T) {
			b := newExForTesting(t, texttest.NopEditor(),
				text.WithCommandKey(testCommandKey),
				text.WithCommandOverlayConfig(testCommandOverlayConfig()),
			)
			defer b.Close()

			// start command prompt
			ev := term.Event{
				Type: term.EventKey,
				Ch:   testCommandKey.Ch,
				Mod:  testCommandKey.Mod,
				Key:  testCommandKey.Key,
			}
			exit, handled := b.Handle(ev)
			assert.True(t, handled)
			require.False(t, exit)

			for _, ch := range cmd {
				exit, handled := b.Handle(term.Event{Ch: ch, Type: term.EventKey})
				assert.True(t, handled)
				require.False(t, exit)
			}

			exit, handled = b.Handle(
				term.Event{Key: term.KeyEnter, Type: term.EventKey})
			assert.True(t, handled)
			require.True(t, exit)

		})
	}

	t.Run("ex does not exit when inner handler returns exit=true", func(t *testing.T) {
		b := newExForTesting(t, texttest.NopEditor(),
			text.WithCommandOverlayConfig(testCommandOverlayConfig()),
		)
		defer b.Close()

		h := browsertest.NewTestHandler()
		h.Exit = true
		h.Handled = true

		uri, err := workspaceapi.ParseURI("file:///bols")
		require.NoError(t, err)

		tab, err := b.comp.Tab(uri, "bleh", h)
		require.NoError(t, err)

		w, err := b.comp.Focus()
		require.NoError(t, err)

		err = w.SetContent(tab)
		require.NoError(t, err)

		exit, handled := b.Handle(term.Event{Ch: 'a', Type: term.EventKey})
		assert.True(t, handled)
		assert.False(t, exit)
	})
}

// remove non-determinism of search.List async search
type testEx struct {
	*ex
}

func (t testEx) Handle(ev term.Event) (bool, bool) {
	quit, handle := t.ex.Handle(ev)
	t.ex.Wait()
	return quit, handle
}

func defCommandKeyBindings() (opts []text.Option) {
	opts = append(opts, text.WithCommandKeyBinding(
		term.KeyComb{Mod: term.ModCtrl, Ch: 'w'}, []string{"closeTab"}))
	opts = append(opts, text.WithCommandKeyBinding(
		term.KeyComb{Mod: term.ModCtrl, Ch: 'l'}, []string{"nextTab"}))
	opts = append(opts, text.WithCommandKeyBinding(
		term.KeyComb{Mod: term.ModCtrl, Ch: 'h'}, []string{"previousTab"}))
	return
}

func newExForTestingTerminal(
	t *testing.T, workspace workspaceLoader,
	ed text.Editor,
	emulatorCfg vte.Config,
	publishEvent func(term.Event) bool,
	opts ...text.Option,
) testEx {
	ex := new(ex)
	ex.syncCommandPrompt = true
	opts = append(opts, text.WithCommandOverlayConfig(testCommandOverlayConfig()))
	opts = append(opts, defCommandKeyBindings()...)
	require.NoError(t, ex.init(ed, workspace, document.NewInMemoryService(),
		emulatorCfg, publishEvent, clipboard.NewInMemory(), opts...))
	ex.subscribeCommands()
	return testEx{ex}
}

func newExForTestingWithWorkspace(
	t *testing.T, workspace workspaceLoader,
	ed text.Editor,
	emulatorCfg vte.Config,
	publishEvent func(term.Event) bool,
	clip clipboard.Register,
	opts ...text.Option,
) testEx {
	ex := new(ex)
	ex.syncCommandPrompt = true
	opts = append(opts, text.WithCommandOverlayConfig(testCommandOverlayConfig()))
	opts = append(opts, defCommandKeyBindings()...)
	require.NoError(t, ex.init(ed, workspace, document.NewInMemoryService(),
		emulatorCfg, publishEvent, clip, opts...))
	ex.subscribeCommands()
	ex.newEmulatorHandler = func(initialCmd string, cfg vte.Config) (vteHandler, error) {
		return newTestVteWithConfig(initialCmd, cfg), nil
	}
	ex.newPluginHandler = func(args ...string) (pluginHandler, error) {
		return newTestVteWithConfig(strings.Join(args, " "), emulatorCfg), nil
	}
	return testEx{ex}
}

func newExForTesting(t *testing.T, ed text.Editor, opts ...text.Option) testEx {
	return newExForTestingWithWorkspace(t, &testLoader{}, ed, vte.DefaultConfig(),
		nopPublishEvent, clipboard.NewInMemory(), opts...)
}

func newExForTestingClipboard(
	t *testing.T, ed text.Editor, clip clipboard.Register, opts ...text.Option,
) testEx {
	return newExForTestingWithWorkspace(t, &testLoader{}, ed, vte.DefaultConfig(),
		nopPublishEvent, clip, opts...)
}

func TestNewWindow(t *testing.T) {
	cases := []handlertest.SequenceTestCase{
		{":newWindow>:changeSplitOrientation h>:newWindow>",
			`┌──────────────────┐
│ changed split    │
│ direction to     │
│ horizontal       │
└──────────────────┘
│        │└────────┘
│        │┌────────┐
│        ││        │
│        ││        │
└────────┘└────────┘`},
		{":closeW>:closeW>aaaaaaa",
			`┌──────────────────┐
│ changed split    │
│ direction to     │
│ horizontal       │
└──────────────────┘
│                  │
│                  │
│                  │
│                  │
└──────────────────┘`},
		{":notificationsCloseAll>",
			`┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│                  │
│                  │
│                  │
│                  │
│                  │
└──────────────────┘`},
	}

	notifications := browser.DefaultConfig().Notifications
	notifications.ProgressBar = false // deterministic tests
	notifications.Width = 20
	opts := []text.Option{
		text.WithCommandKey(testCommandKey),
		text.WithNotificationsConfig(notifications),
	}
	b := newExForTesting(t, texttest.NopEditor(), opts...)
	defer b.Close()

	handlertest.TestHandlerSequence(t, b, 20, 10, cases)
}

func TestCommandHistory(t *testing.T) {
	cases := []handlertest.SequenceTestCase{
		{":edit hello.go>:edit wi.go>1234",
			`┌──────────────────┐
│hello.go  wi.go   │
├──────────────────┤
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
└──────────────────┘`},
		{"::::>",
			`┌──────────────────┐
│hello.go  wi.go   │
├──────────────────┤
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
└──────────────────┘`},
	}

	opts := []text.Option{
		text.WithCommandKey(testCommandKey),
	}
	b := newExForTesting(t, texttest.NopEditor(), opts...)
	defer b.Close()

	handlertest.TestHandlerSequence(t, b, 20, 10, cases)
}

func TestCommandAliases(t *testing.T) {
	cases := []handlertest.SequenceTestCase{
		{":todo>1234",
			`┌──────────────────┐
│hello.go  wi.go   │
├──────────────────┤
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
└──────────────────┘`},
		{":bp>",
			`┌──────────────────┐
│hello.go  wi.go   │
├──────────────────┤
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`},
		{":e x.go>",
			`┌──────────────────┐
│..  wi.go  x.go   │
├──────────────────┤
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`},
	}

	opts := []text.Option{
		text.WithCommandOverlayConfig(testCommandOverlayConfig()),
		text.WithCommandKey(testCommandKey),
		text.WithCommandAliases(map[string]text.CommandAlias{
			"todo": text.CommandAlias{Commands: []string{"edit hello.go", "edit wi.go"}},
			"e":    text.CommandAlias{Commands: []string{"edit"}},
			"bp":   text.CommandAlias{Commands: []string{"nextTab"}},
		}),
	}
	b := newExForTesting(t, texttest.NopEditor(), opts...)
	defer b.Close()

	handlertest.TestHandlerSequence(t, b, 20, 10, cases)
}

func TestIntegrationEphemeralTerminal(t *testing.T) {
	cases := []handlertest.SequenceTestCase{
		{":! sleep 20>",
			`┌──────────────────┐
│                  │
├┌────────────────┐┤
││ ◦   sleep 20 0s││
││────────────────││
││▐               ││
││                ││
││                ││
││                ││
└└────────────────┘┘`,
		},
		{":closeTab>",
			`┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│                  │
│                  │
│                  │
│                  │
│                  │
└──────────────────┘`,
		},
		{":! sleep 20>",
			`┌──────────────────┐
│                  │
├┌────────────────┐┤
││ ◦   sleep 20 0s││
││────────────────││
││▐               ││
││                ││
││                ││
││                ││
└└────────────────┘┘`,
		},
		{":closeWindow>",
			`┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│                  │
│                  │
│                  │
│                  │
│                  │
└──────────────────┘`,
		},
	}

	opts := []text.Option{
		text.WithCommandKey(testCommandKey),
		text.WithCommandOverlayConfig(testCommandOverlayConfig()),
		text.WithEventPublisher(nopPublishEvent),
	}

	tempDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	uri, err := workspaceapi.ParseURI(filepath.Join("file://", tempDir))
	require.NoError(t, err)
	ctx := context.Background()
	fileScheme, err := workspace.NewFileScheme(ctx, config.NopConfig(), uri)
	require.NoError(t, err)
	defer fileScheme.Close()

	workspace := workspace.NewSchemeWorkspace(uri, fileScheme)
	b := newExForTestingTerminal(t, workspace,
		texttest.NopEditor(), vte.DefaultConfig(), nopPublishEvent, opts...)
	defer b.Close()

	handlertest.TestHandlerSequence(t, b, 20, 10, cases)
}

func TestIntegrationCompanionTerminal(t *testing.T) {

	cases := []handlertest.SequenceTestCase{
		{":!>_______",
			`┌──────────────────┐
│                  │
├┌────────────────┐┤
││sh ▐            ││
││                ││
││                ││
││                ││
││                ││
││                ││
└└────────────────┘┘`,
		},
		{"<$:notificationsCloseAll>",
			`┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│                  │
│                  │
│                  │
│                  │
│                  │
└──────────────────┘`,
		},
		{":!>", // no need to wait now, it should pick previous session
			`┌──────────────────┐
│                  │
├┌────────────────┐┤
││sh ▐            ││
││                ││
││                ││
││                ││
││                ││
││                ││
└└────────────────┘┘`,
		},
		{"#:notificationsCloseAll>",
			`┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│                  │
│                  │
│                  │
│                  │
│                  │
└──────────────────┘`,
		},
		{":!>`", // ` simulates ctrl-v
			`┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│                  │
│                  │
│                  │
│                  │
│                  │
└──────────────────┘`,
		},
	}

	opts := []text.Option{
		text.WithCommandKey(testCommandKey),
		text.WithCommandKeyBinding(term.KeyComb{Mod: term.ModCtrl, Ch: 'h'}, []string{"closeWindow"}),
		text.WithCommandKeyBinding(term.KeyComb{Mod: term.ModCtrl, Ch: 'l'}, []string{"nextTab"}),
		text.WithCommandKeyBinding(term.KeyComb{Mod: term.ModCtrl, Ch: 'v'}, []string{"closeTab"}),
		text.WithCommandOverlayConfig(testCommandOverlayConfig()),
	}

	tempDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	uri, err := workspaceapi.ParseURI(filepath.Join("file://", tempDir))
	require.NoError(t, err)
	ctx := context.Background()
	fileScheme, err := workspace.NewFileScheme(ctx, config.NopConfig(), uri)
	require.NoError(t, err)
	defer fileScheme.Close()

	// do not depend on host shell, which can vary across hosts
	cfg := vte.DefaultConfig()
	cfg.Shell = "sh"

	// do not depend on default shell prompt, as it can change
	// and it does change accross versions
	ps1 := os.Getenv("PS1")
	os.Setenv("PS1", "sh ")
	defer os.Setenv("PS1", ps1)

	workspace := workspace.NewSchemeWorkspace(uri, fileScheme)
	b := newExForTestingTerminal(t, workspace,
		texttest.NopEditor(), cfg, nopPublishEvent, opts...)
	defer b.Close()

	handlertest.TestHandlerSequence(t, b, 20, 10, cases)
}

func TestFullScreen(t *testing.T) {
	cases := []handlertest.SequenceTestCase{
		{":splitWindow>:edit aaa>:edit bbb>:toggleFullscreen>",
			`AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA`,
		},
		{":toggleFullScreen>",
			`┌──────────────────┐
│aaa  bbb          │
├────────┐┌────────┐
│        ││AAAAAAAA│
│        ││AAAAAAAA│
│        ││AAAAAAAA│
│        ││AAAAAAAA│
│        ││AAAAAAAA│
│        ││AAAAAAAA│
└────────┘└────────┘`,
		},
	}

	opts := []text.Option{
		text.WithCommandKey(testCommandKey),
		text.WithCommandOverlayConfig(testCommandOverlayConfig()),
	}

	tempDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	uri, err := workspaceapi.ParseURI(filepath.Join("file://", tempDir))
	require.NoError(t, err)
	ctx := context.Background()
	fileScheme, err := workspace.NewFileScheme(ctx, config.NopConfig(), uri)
	require.NoError(t, err)
	defer fileScheme.Close()

	workspace := workspace.NewSchemeWorkspace(uri, fileScheme)
	b := newExForTestingWithWorkspace(t, workspace,
		texttest.NopEditor(), vte.DefaultConfig(), nopPublishEvent,
		clipboard.NewInMemory(), opts...)
	defer b.Close()

	handlertest.TestHandlerSequence(t, b, 20, 10, cases)
}

func TestExposedRootNodeIssue(t *testing.T) {
	notifications := browser.DefaultConfig().Notifications
	notifications.ProgressBar = false // deterministic tests
	notifications.Width = 20
	opts := []text.Option{
		text.WithCommandOverlayConfig(testCommandOverlayConfig()),
		text.WithCommandKey(testCommandKey),
		text.WithNotificationsConfig(notifications),
		text.WithCommandAliases(map[string]text.CommandAlias{
			"boom": text.CommandAlias{
				Commands: []string{
					"newWindow",
					"changeSplitOrientation h",
					"newWindow",
					"changeSplitOrientation v",
					"newWindow",
					"focusPrevWindow",
					"focusPrevWindow",
				},
			},
		}),
	}
	cases := []handlertest.SequenceTestCase{
		{":boom>",
			`┌──────────────────┐
│ changed split    │
┌ direction to     │
│ vertical         │
└──────────────────┘
┌──────────────────┐
│ changed split    │
│ direction to     │
│ horizontal       │
└──────────────────┘`,
		},
	}

	b := newExForTesting(t, texttest.NopEditor(), opts...)
	defer b.Close()
	handlertest.TestHandlerSequence(t, b, 20, 10, cases)
}

func TestEditCompletion(t *testing.T) {
	cases := []handlertest.SequenceTestCase{
		{":edit re",
			`                    
                    
                    
                    
edit re▐            
retalls             
                    
                    
                    
                    `},
		{":edit dawo",
			`                    
                    
                    
                    
edit dawo▐          
daworg              
                    
                    
                    
                    `},
		{":edit dawo✌re",
			`                    
                    
                    
                    
edit daworg re▐     
retalls             
                    
                    
                    
                    `},
		{":edit dawo⬇✌re",
			`                    
                    
                    
                    
edit daworg re▐     
retalls             
                    
                    
                    
                    `},
		{":edit dawo⬇✌re✌^^^^^^^^^^^",
			`                    
                    
                    
                    
edit dawo▐          
daworg              
                    
                    
                    
                    `},
		{":edit dawo⬇✌re✌^^^^^^^^^^^✌re",
			`                    
                    
                    
                    
edit daworg re▐     
retalls             
                    
                    
                    
                    `},
		{":edit dawo⬇✌re✌^^^^^^^^^^^^^^^^^",
			`                    
                    
                    
                    
edi▐                
edit                
notificationsSendInf
readFile            
reloadFile!         
notificationsSendWar`},
		{":edit dawo⬇✌re✌^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^",
			`                    
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
	}

	fn := func(t *testing.T) tui.Handler {
		opts := []text.Option{
			text.WithCommandKey(testCommandKey),
			text.WithWindowManagerConfig(handler.WindowManagerConfig{
				WindowManagerConfig: component.WindowManagerConfig{Frame: false}}),
			text.WithCommandOverlayConfig(testCommandOverlayConfig()),
		}
		uri, err := workspaceapi.ParseURI("memory:///")
		require.NoError(t, err)
		scheme, err := workspace.NewMemoryScheme(context.Background(), config.NopConfig(), uri)
		require.NoError(t, err)
		touchTestFile(t, scheme, "daworg")
		touchTestFile(t, scheme, "retalls")
		b := newExForTestingWithWorkspace(t, workspace.NewSchemeWorkspace(uri, scheme),
			texttest.NopEditor(), vte.DefaultConfig(), nopPublishEvent,
			clipboard.NewInMemory(), opts...)
		t.Cleanup(func() { _ = b.Close() })
		return b
	}

	handlertest.TestHandlerIsolated(t, fn, 20, 10, cases)
}

func TestRenameTab(t *testing.T) {
	cases := []handlertest.SequenceTestCase{
		{":edit hello.go>:renameTab 8berSucks>",
			`┌──────────────────┐
│8berSucks         │
├──────────────────┤
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`},
	}

	opts := []text.Option{
		text.WithCommandKey(testCommandKey),
	}
	b := newExForTesting(t, texttest.NopEditor(), opts...)
	defer b.Close()

	handlertest.TestHandlerSequence(t, b, 20, 10, cases)
}

func TestEventNone(t *testing.T) {
	t.Run("delegates to underlying handler", func(t *testing.T) {
		cases := []handlertest.SequenceTestCase{
			{"🎉edit hello.go>",
				`┌──────────────────┐
│hello.go          │
├──────────────────┤
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`},
		}

		testCommandKey := term.KeyComb{Key: term.KeySpace, Mod: term.ModCtrl}
		opts := []text.Option{
			text.WithCommandKey(testCommandKey),
		}
		b := newExForTesting(t, texttest.NopEditor(), opts...)
		defer b.Close()

		handlertest.TestHandlerSequence(t, b, 20, 10, cases)

		b.Handle(term.Event{Type: term.EventNone})

		cases = []handlertest.SequenceTestCase{
			{"",
				`┌──────────────────┐
│hello.go          │
├──────────────────┤
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`},
		}
		handlertest.TestHandlerSequence(t, b, 20, 10, cases)
	})
}

func TestTerminalOnFocus(t *testing.T) {
	t.Run("new terminal tab", func(t *testing.T) {
		uri, err := workspaceapi.ParseURI("memory:///")
		require.NoError(t, err)
		scheme, _ := workspace.NewMemoryScheme(context.Background(), config.NopConfig(), uri)
		testConfig := vte.DefaultConfig()
		ex := newExForTestingWithWorkspace(t, workspace.NewSchemeWorkspace(uri, scheme),
			texttest.NopEditor(), testConfig, nopPublishEvent, clipboard.NewInMemory())
		tvte := newTestVte()
		ex.newEmulatorHandler = func(initialCmd string, cfg vte.Config) (vteHandler, error) {
			assert.Equal(t, "echo bla", initialCmd)
			assert.NotNil(t, cfg.ScheduleNextTick)
			assert.NotNil(t, cfg.RingBell)
			cfg.RingBell = nil
			cfg.ScheduleNextTick = nil
			expected := testConfig
			expected.RingBell = nil
			expected.ScheduleNextTick = nil
			assert.Equal(t, expected, cfg)
			return tvte, nil
		}
		t.Cleanup(func() { _ = ex.Close() })

		ex.newTerminalTab("echo bla")

		require.Len(t, tvte.onFocusChange, 2)
		assert.False(t, tvte.onFocusChange[0])
		assert.True(t, tvte.onFocusChange[1])

		// switch to some other tab, same window
		ex.editFiles("a")
		require.Len(t, tvte.onFocusChange, 3)
		assert.False(t, tvte.onFocusChange[2])

		// switch back to terminal tab, same window
		ex.previousTab()
		require.Len(t, tvte.onFocusChange, 4)
		assert.True(t, tvte.onFocusChange[3])

		// new window, tab still in screen but not focused
		ex.newWindow()
		require.Len(t, tvte.onFocusChange, 5)
		assert.False(t, tvte.onFocusChange[4])

		// focus back to tab window
		ex.focusPrevWindow()
		require.Len(t, tvte.onFocusChange, 6)
		assert.True(t, tvte.onFocusChange[5])

		_, handled := ex.Handle(term.Event{Type: term.EventUnfocus})
		require.True(t, handled)
		require.Len(t, tvte.onFocusChange, 7)
		assert.False(t, tvte.onFocusChange[6])

		_, handled = ex.Handle(term.Event{Type: term.EventFocus})
		require.True(t, handled)
		require.Len(t, tvte.onFocusChange, 8)
		assert.True(t, tvte.onFocusChange[7])

		ex.closeTab()
		require.Len(t, tvte.onFocusChange, 9)
		assert.False(t, tvte.onFocusChange[8])
	})

	t.Run("companion terminal", func(t *testing.T) {
		uri, err := workspaceapi.ParseURI("memory:///")
		require.NoError(t, err)
		scheme, _ := workspace.NewMemoryScheme(context.Background(), config.NopConfig(), uri)
		testConfig := vte.DefaultConfig()
		ex := newExForTestingWithWorkspace(t, workspace.NewSchemeWorkspace(uri, scheme),
			texttest.NopEditor(), testConfig, nopPublishEvent, clipboard.NewInMemory())
		tvte := newTestVte()
		ex.newEmulatorHandler = func(initialCmd string, cfg vte.Config) (vteHandler, error) {
			testConfig := testConfig
			testConfig.WidthHint = 80
			testConfig.HeightHint = 80
			assert.NotNil(t, cfg.ScheduleNextTick)
			assert.NotNil(t, cfg.RingBell)
			cfg.ScheduleNextTick = nil
			cfg.RingBell = nil
			testConfig.ScheduleNextTick = nil
			testConfig.RingBell = nil
			assert.Equal(t, testConfig, cfg)
			return tvte, nil
		}
		t.Cleanup(func() { _ = ex.Close() })
		ex.Resize(100, 100)

		ex.executePlugin()

		require.Len(t, tvte.onFocusChange, 2)
		assert.False(t, tvte.onFocusChange[0])
		assert.True(t, tvte.onFocusChange[1])

		// switching from floating to other window should trigger on focus change
		ex.focusPrevWindow()
		require.Len(t, tvte.onFocusChange, 3)
		assert.False(t, tvte.onFocusChange[2])

		// switching back to floating should trigger again
		ex.Handle(term.Event{Type: term.EventMouse, MouseX: 50, MouseY: 50, Key: term.MouseLeft})
		require.Len(t, tvte.onFocusChange, 4)
		assert.True(t, tvte.onFocusChange[3])

		_, handled := ex.Handle(term.Event{Type: term.EventUnfocus})
		require.True(t, handled)
		require.Len(t, tvte.onFocusChange, 5)
		assert.False(t, tvte.onFocusChange[4])

		_, handled = ex.Handle(term.Event{Type: term.EventFocus})
		require.True(t, handled)
		require.Len(t, tvte.onFocusChange, 6)
		assert.True(t, tvte.onFocusChange[5])

		// indirectly toggle terminal companion
		ex.editFiles("a")
		require.Len(t, tvte.onFocusChange, 7)
		assert.False(t, tvte.onFocusChange[6])
	})

	t.Run("ephemeral terminal", func(t *testing.T) {
		uri, err := workspaceapi.ParseURI("memory:///")
		require.NoError(t, err)
		scheme, _ := workspace.NewMemoryScheme(context.Background(), config.NopConfig(), uri)
		testConfig := vte.DefaultConfig()
		ex := newExForTestingWithWorkspace(t, workspace.NewSchemeWorkspace(uri, scheme),
			texttest.NopEditor(), testConfig, nopPublishEvent, clipboard.NewInMemory())
		tvte := newTestVte()
		ex.newPluginHandler = func(args ...string) (pluginHandler, error) {
			require.Len(t, args, 2)
			assert.Equal(t, "echo", args[0])
			assert.Equal(t, "bla", args[1])
			return tvte, nil
		}
		t.Cleanup(func() { _ = ex.Close() })
		ex.Resize(100, 100)
		ex.editFiles("a", "b") // have tabs available for later

		ex.executePlugin("echo", "bla")

		require.Len(t, tvte.onFocusChange, 2)
		assert.False(t, tvte.onFocusChange[0])
		assert.True(t, tvte.onFocusChange[1])

		// switching from floating to other window should trigger on focus change
		ex.focusPrevWindow()
		require.Len(t, tvte.onFocusChange, 3)
		assert.False(t, tvte.onFocusChange[2])

		// switching back to floating should trigger again
		ex.Handle(term.Event{Type: term.EventMouse, MouseX: 50, MouseY: 50, Key: term.MouseLeft})
		require.Len(t, tvte.onFocusChange, 4)
		assert.True(t, tvte.onFocusChange[3])

		_, handled := ex.Handle(term.Event{Type: term.EventUnfocus})
		require.True(t, handled)
		require.Len(t, tvte.onFocusChange, 5)
		assert.False(t, tvte.onFocusChange[4])

		_, handled = ex.Handle(term.Event{Type: term.EventFocus})
		require.True(t, handled)
		require.Len(t, tvte.onFocusChange, 6)
		assert.True(t, tvte.onFocusChange[5])

		// ephemeral close should trigger another focus event
		ex.nextTab()
		require.Len(t, tvte.onFocusChange, 7)
		assert.False(t, tvte.onFocusChange[6])
	})
}

func TestSwitchToTab(t *testing.T) {
	cases := []handlertest.SequenceTestCase{
		{":edit hello.go>B:edit world.go>",
			`┌────────────────────────────┐
│hello.go  world.go          │
├────────────────────────────┤
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
└────────────────────────────┘`},

		{":switchToTab ",
			`┌────────────────────────────┐
│hello.go  world.go          │
├────────────────────────────┤
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
┌────────────────────────────┐
│switchToTab ▐               │
│1                           │
│2                           │
└────────────────────────────┘
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
└────────────────────────────┘`},
		{"1>",
			`┌────────────────────────────┐
│hello.go  world.go          │
├────────────────────────────┤
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
└────────────────────────────┘`},
		{":switchToTab 3>",
			`┌────────────────────────────┐
│hello.go  world.go          │
├────────────────────────────┤
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
└────────────────────────────┘`},
		{":switchToTab 0>",
			`┌────────────────────────────┐
│The first tab is 1          │
└────────────────────────────┘
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBBBBBBBBBBBB│
└────────────────────────────┘`},
	}

	opts := []text.Option{
		text.WithCommandKey(testCommandKey),
	}
	b := newExForTesting(t, texttest.NopEditor(), opts...)
	defer b.Close()

	handlertest.TestHandlerSequence(t, b, 30, 15, cases)
}

func TestEcho(t *testing.T) {
	cases := []handlertest.SequenceTestCase{
		{`:edit hello.go>:echo 01234>`,
			`┌────────────────────────────┐
│hello.go                    │
├────────────────────────────┤
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
└────────────────────────────┘`},
	}

	opts := []text.Option{
		text.WithCommandKey(testCommandKey),
	}
	var e testEx
	var i int
	publishEvent := func(ev term.Event) bool {
		if ev.Type == term.EventInterrupt {
			return true
		}
		assert.Equal(t, string(ev.Ch), strconv.Itoa(i))
		i++
		return true
	}

	e = newExForTestingWithWorkspace(t, &testLoader{},
		texttest.NopEditor(), vte.DefaultConfig(),
		publishEvent, clipboard.NewInMemory(), opts...)
	defer e.Close()

	handlertest.TestHandlerSequence(t, e, 30, 15, cases)
}

func TestCopyToClipboard(t *testing.T) {
	clip := clipboard.NewInMemory()
	testCopyToClipboard(t, clip, func(ed text.Editor, opts ...text.Option) (
		tui.Handler, browser.Browser, error,
	) {
		b := newExForTestingClipboard(t, texttest.NopEditor(), clip, opts...)
		defer b.Close()
		return b, b.Browser(), nil
	})
}

func testCopyToClipboard(
	t *testing.T, clip clipboard.Register, constructor browserConstructor,
) {
	cases := []handlertest.SequenceTestCase{
		{":edit hello.go>",
			`┌────────────────────────────┐
│hello.go                    │
├────────────────────────────┤
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
└────────────────────────────┘`},
		{":clipboardPaste>",
			`┌────────────────────────────┐
│nothing to paste            │
└────────────────────────────┘
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
└────────────────────────────┘`},
		{":notificationsCloseAll>:clipboardCopy>",
			`┌────────────────────────────┐
│copied to clipboard         │
└────────────────────────────┘
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAAAAAAAAAAAA│
└────────────────────────────┘`},
		{":notificationsCloseAll>:clipboardPaste>",
			`┌────────────────────────────┐
│hello.go                    │
├────────────────────────────┤
│DDDDDDDDDDDDDDDDDDDDDDDDDDDD│
│DDDDDDDDDDDDDDDDDDDDDDDDDDDD│
│DDDDDDDDDDDDDDDDDDDDDDDDDDDD│
│DDDDDDDDDDDDDDDDDDDDDDDDDDDD│
│DDDDDDDDDDDDDDDDDDDDDDDDDDDD│
│DDDDDDDDDDDDDDDDDDDDDDDDDDDD│
│DDDDDDDDDDDDDDDDDDDDDDDDDDDD│
│DDDDDDDDDDDDDDDDDDDDDDDDDDDD│
│DDDDDDDDDDDDDDDDDDDDDDDDDDDD│
│DDDDDDDDDDDDDDDDDDDDDDDDDDDD│
│DDDDDDDDDDDDDDDDDDDDDDDDDDDD│
└────────────────────────────┘`},
	}

	opts := []text.Option{
		text.WithCommandKey(testCommandKey),
	}
	bh, _, err := constructor(texttest.NopEditor(), opts...)
	require.NoError(t, err)

	handlertest.TestHandlerSequence(t, bh, 30, 15, cases)

	data, err := clip.Paste(clipboard.DefaultRegisterID)
	require.NoError(t, err)
	assert.Equal(t, "A", data.Text)
}

func notificationsConfig() notifications.Config {
	ret := browser.DefaultConfig().Notifications
	ret.Width = 15
	ret.ProgressBar = false // deterministic tests
	return ret
}

func touchTestFile(t *testing.T, scheme schemeapi.Scheme, name string) {
	f, werr := scheme.Open(name, os.O_CREATE, 0666)
	require.Nil(t, werr)
	require.NoError(t, f.Sync())
}

func testCommandOverlayConfig() text.CommandOverlayConfig {
	return text.CommandOverlayConfig{
		ShowManualAfter: 1 * time.Hour,
	}
}

type testVte struct {
	component.String
	initialCmd string
	cfg        vte.Config

	calledClose   bool
	defAttr       term.Attributes
	onFocusChange []bool

	isComplete bool
	uri        workspaceapi.URI
	title      string
}

func newTestVte() *testVte {
	return newTestVteWithConfig("", vte.DefaultConfig())
}

func newTestVteWithConfig(initialCmd string, cfg vte.Config) *testVte {
	ret := new(testVte)
	ret.initialCmd = initialCmd
	ret.cfg = cfg
	ret.String = component.NewString(initialCmd)
	return ret
}

func (t *testVte) Handle(ev term.Event) (bool, bool) {
	return false, false
}

func (t *testVte) SeekUp() bool {
	return false
}

func (t *testVte) SeekDown() bool {
	return false
}

func (t *testVte) SeekOffset() int {
	return 0
}

func (t *testVte) MaxSeekOffset() int {
	return 0
}

func (t *testVte) Cursor() (ret term.Coordinates, style term.CursorStyle, show bool) {
	show = true
	ret = term.Coordinates{X: len(t.initialCmd)}
	return
}

func (t *testVte) Selection() (string, bool) {
	return "", false
}

func (t *testVte) Man() tui.Manual {
	return tui.Manual{}
}

func (t *testVte) Dimensions() (int, int) {
	return 10, 10
}

func (v *testVte) Close() error {
	if v.calledClose {
		return errors.New("called close twice")
	}
	v.calledClose = true
	return nil
}

func (v *testVte) OnFocusChange(inFocus bool) {
	v.onFocusChange = append(v.onFocusChange, inFocus)
}

func (v *testVte) SetDefaultAttributes(attr term.Attributes) {
	v.defAttr = attr
}

func (v *testVte) IsComplete() bool {
	return v.isComplete
}

func (v *testVte) URI() workspaceapi.URI {
	return v.uri
}

func (v *testVte) Title() string {
	return v.title
}
