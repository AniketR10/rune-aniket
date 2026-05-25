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
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/ide/vctrl"
	"unstable.build/go-tui/term/vte"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/text/texttest"
	"unstable.build/go-tui/workspace"
)

func TestEventDispatching(t *testing.T) {
	t.Run("empty event does not panic", func(t *testing.T) {
		var mu sync.Mutex

		x := newExForEventTesting(t)
		fsev := testEventInfo{}

		assert.NotPanics(t, func() {
			dispatchFilesystemEvent(x, &mu, vctrl.NopMatcher(false), fsev)
		})
	})

	t.Run("dispatches", func(t *testing.T) {
		suite := []struct {
			in  schemeapi.Event
			out textapi.EventType
		}{
			{schemeapi.Create, textapi.EventTypeCreate},
			{schemeapi.Remove, textapi.EventTypeRemove},
			{schemeapi.Write, textapi.EventTypeChange},
			{schemeapi.Rename, textapi.EventTypeRename},
		}
		for _, test := range suite {
			desc := fmt.Sprintf("%s event when %s event is received", test.out, test.in)
			t.Run(desc, func(t *testing.T) {
				var mu sync.Mutex
				testURI, err := workspaceapi.ParseURI("file:///a")
				require.NoError(t, err)

				x := newExForEventTesting(t)
				fsev := testEventInfo{e: test.in, u: testURI}

				var called int
				x.comp.SubscribeEvents([]textapi.EventType{test.out},
					textapi.FuncEventHandler(func(ctx context.Context, ev textapi.Event) bool {
						called++
						assert.Equal(t, testURI, ev.URI)
						return false
					}))

				dispatchFilesystemEvent(x, &mu, vctrl.NopMatcher(false), fsev)
				assert.Equal(t, 1, called)
			})
		}
	})

	t.Run("ignores if matcher matches file", func(t *testing.T) {
		suite := []struct {
			in  schemeapi.Event
			out textapi.EventType
		}{
			{schemeapi.Create, textapi.EventTypeCreate},
			{schemeapi.Remove, textapi.EventTypeRemove},
			{schemeapi.Write, textapi.EventTypeChange},
			{schemeapi.Rename, textapi.EventTypeRename},
		}
		for _, test := range suite {
			desc := fmt.Sprintf("when %s event is received", test.in)
			t.Run(desc, func(t *testing.T) {
				var mu sync.Mutex
				testURI, err := workspaceapi.ParseURI("file:///a")
				require.NoError(t, err)

				ignores := vctrl.NopMatcher(true)

				x := newExForEventTesting(t)
				fsev := testEventInfo{e: test.in, u: testURI}

				var called int
				x.comp.SubscribeEvents([]textapi.EventType{test.out},
					textapi.FuncEventHandler(func(ctx context.Context, ev textapi.Event) bool {
						called++
						return false
					}))

				dispatchFilesystemEvent(x, &mu, ignores, fsev)
				assert.Equal(t, 0, called)
			})
		}
	})

	t.Run("ignores if user just flushed file", func(t *testing.T) {
		suite := []struct {
			in    schemeapi.Event
			dirty bool
		}{
			{schemeapi.Create, true},
			{schemeapi.Remove, true},
			{schemeapi.Write, true},
			{schemeapi.Rename, true},

			{schemeapi.Create, false},
			{schemeapi.Remove, false},
			{schemeapi.Write, false},
			{schemeapi.Rename, false},
		}
		for _, test := range suite {
			desc := fmt.Sprintf("when %s event is received, dirty=%t", test.in, test.dirty)
			t.Run(desc, func(t *testing.T) {
				var mu sync.Mutex

				ignores := vctrl.NopMatcher(true)

				x := newExForEventTesting(t)
				testURI, err := x.workspace.URI("a")
				require.NoError(t, err)

				require.NoError(t, x.editFiles(bgctx, testURI.Path()))
				ed, err := x.comp.Editor(testURI)
				require.NoError(t, err)
				// flush below clears dirty property but it's
				// still useful to make sure that it's integrated correctly
				if test.dirty {
					ctx := context.Background()
					_, _, _ = ed.CellEditor().
						Edit(ctx, term.Coordinates{}, term.Coordinates{}, "ABC")
				}
				win, err := x.comp.Focus()
				require.NoError(t, err)
				ch, err := x.comp.Flush(context.Background(), win)
				require.NoError(t, err)
				require.NoError(t, <-ch)

				res, ok := x.comp.Resource(testURI)
				require.True(t, ok)

				flush, err := x.comp.LastFlush(res)
				require.NoError(t, err)

				fsev := testEventInfo{e: test.in, u: testURI}
				dispatchFilesystemEvent(x, &mu, ignores, fsev)

				assertNoPrompt(t, x)

				lastFlush, err := x.comp.LastFlush(res)
				require.NoError(t, err)
				assert.Equal(t, flush, lastFlush)
			})
		}
	})

	t.Run("is goroutine safe", func(t *testing.T) {
		// Production callers serialize filesystem events on a
		// single goroutine (dispatchFilesystemEvents in
		// workspace_handler.go loops over the scheme's event
		// channel). dispatchFilesystemEvent locks mu internally
		// so concurrent fan-out is safe only when callers also
		// take mu before invoking it; without that, the
		// off-thread reload worker that writes tab attrs via
		// editorFlusherCloser.OnDidEdit races a concurrent
		// IsDirty read in handleFSChange. Mirror the production
		// caller's invariant here.
		var mu sync.Mutex
		ignores := vctrl.NopMatcher(false)

		x := newExForEventTesting(t)
		testURI, err := x.workspace.URI("a")
		require.NoError(t, err)

		createOpenWriteFile(t, x, testURI, "abc")

		fsev := testEventInfo{e: schemeapi.Write, u: testURI}

		n := 1000
		dispatchMu := &sync.Mutex{}
		var wg sync.WaitGroup
		wg.Add(n)
		for range n {
			go func() {
				defer wg.Done()
				dispatchMu.Lock()
				defer dispatchMu.Unlock()
				dispatchFilesystemEvent(x, &mu, ignores, fsev)
				// Reloads are async: the worker that writes
				// dirty tab attrs via OnDidEdit continues
				// after dispatchFilesystemEvent returns.
				// Drain it before another dispatch reads
				// IsDirty from the host goroutine.
				x.waitInflight()
			}()
		}
		for i := range n {
			_ = os.WriteFile(testURI.Path(), []byte(strconv.Itoa(i)), 0666)
			mu.Lock()
			_, ok := x.comp.Resource(testURI)
			assert.True(t, ok)
			mu.Unlock()
		}
		wg.Wait()
	})

	t.Run("write triggers reload file if open, pre-created, non-dirty", func(t *testing.T) {
		var mu sync.Mutex
		ignores := vctrl.NopMatcher(false)

		x := newExForEventTesting(t)
		testURI, err := x.workspace.URI("a")
		require.NoError(t, err)

		createOpenWriteFile(t, x, testURI, "abc")

		fsev := testEventInfo{e: schemeapi.Write, u: testURI}
		dispatchFilesystemEvent(x, &mu, ignores, fsev)

		assertBufferContent(t, x, testURI, "abc")
	})

	t.Run("write triggers reload file if open, uncreated, non-dirty", func(t *testing.T) {
		var mu sync.Mutex
		ignores := vctrl.NopMatcher(false)

		x := newExForEventTesting(t)
		testURI, err := x.workspace.URI("a")
		require.NoError(t, err)

		openWriteUncreatedFile(t, x, testURI, "abc")

		fsev := testEventInfo{e: schemeapi.Write, u: testURI}
		dispatchFilesystemEvent(x, &mu, ignores, fsev)

		assertBufferContent(t, x, testURI, "abc")
	})

	t.Run("write triggers prompt if open, dirty file changes, user discards", func(t *testing.T) {
		var mu sync.Mutex
		ignores := vctrl.NopMatcher(false)

		x := newExForEventTesting(t)
		testURI, err := x.workspace.URI("a")
		require.NoError(t, err)

		createOpenWriteFile(t, x, testURI, "abc")

		editBuffer(t, x, testURI, "ABC")

		fsev := testEventInfo{e: schemeapi.Write, u: testURI}
		dispatchFilesystemEvent(x, &mu, ignores, fsev)

		userPromptDiscards(t, x)

		assertBufferContent(t, x, testURI, "abc")

		assertNoPrompt(t, x)
	})

	t.Run("write triggers prompt if open, dirty file changes, user overwrites", func(t *testing.T) {
		var mu sync.Mutex
		ignores := vctrl.NopMatcher(false)

		x := newExForEventTesting(t)
		testURI, err := x.workspace.URI("a")
		require.NoError(t, err)

		createOpenWriteFile(t, x, testURI, "abc")
		editBuffer(t, x, testURI, "ABC")

		fsev := testEventInfo{e: schemeapi.Write, u: testURI}
		dispatchFilesystemEvent(x, &mu, ignores, fsev)

		userPromptOverwrites(t, x)

		assertFileContent(t, x, testURI, "ABC")
		assertBufferContent(t, x, testURI, "ABC")

		assertNoPrompt(t, x)
	})

	t.Run("create triggers reload file if open, uncreated, non-dirty", func(t *testing.T) {
		var mu sync.Mutex
		ignores := vctrl.NopMatcher(false)

		x := newExForEventTesting(t)
		testURI, err := x.workspace.URI("a")
		require.NoError(t, err)

		openWriteUncreatedFile(t, x, testURI, "abc")

		fsev := testEventInfo{e: schemeapi.Create, u: testURI}
		dispatchFilesystemEvent(x, &mu, ignores, fsev)

		assertBufferContent(t, x, testURI, "abc")
	})

	t.Run("create triggers reload file if open, pre-created, non-dirty", func(t *testing.T) {
		var mu sync.Mutex
		ignores := vctrl.NopMatcher(false)

		x := newExForEventTesting(t)
		testURI, err := x.workspace.URI("a")
		require.NoError(t, err)

		createOpenWriteFile(t, x, testURI, "abc")

		fsev := testEventInfo{e: schemeapi.Create, u: testURI}
		dispatchFilesystemEvent(x, &mu, ignores, fsev)

		assertBufferContent(t, x, testURI, "abc")
		assertNoPrompt(t, x)
	})

	t.Run("create triggers prompt if open, dirty file changes; user discards", func(t *testing.T) {
		var mu sync.Mutex
		ignores := vctrl.NopMatcher(false)

		x := newExForEventTesting(t)
		testURI, err := x.workspace.URI("a")
		require.NoError(t, err)

		createOpenWriteFile(t, x, testURI, "abc")

		editBuffer(t, x, testURI, "ABC")

		fsev := testEventInfo{e: schemeapi.Create, u: testURI}
		dispatchFilesystemEvent(x, &mu, ignores, fsev)

		userPromptDiscards(t, x)

		assertBufferContent(t, x, testURI, "abc")
		assertNoPrompt(t, x)
	})

	t.Run("create triggers prompt if open, dirty file changes; user overwrites", func(t *testing.T) {
		var mu sync.Mutex
		ignores := vctrl.NopMatcher(false)

		x := newExForEventTesting(t)
		testURI, err := x.workspace.URI("a")
		require.NoError(t, err)

		createOpenWriteFile(t, x, testURI, "abc")

		editBuffer(t, x, testURI, "ABC")

		fsev := testEventInfo{e: schemeapi.Create, u: testURI}
		dispatchFilesystemEvent(x, &mu, ignores, fsev)

		userPromptOverwrites(t, x)

		assertFileContent(t, x, testURI, "ABC")
		assertBufferContent(t, x, testURI, "ABC")
		assertNoPrompt(t, x)
	})

	t.Run("remove triggers nothing if open, non-dirty file", func(t *testing.T) {
		var mu sync.Mutex
		ignores := vctrl.NopMatcher(false)

		x := newExForEventTesting(t)
		testURI, err := x.workspace.URI("a")
		require.NoError(t, err)

		require.NoError(t, x.editFiles(bgctx, testURI.String()))

		fsev := testEventInfo{e: schemeapi.Remove, u: testURI}
		dispatchFilesystemEvent(x, &mu, ignores, fsev)

		assertNoPrompt(t, x)
		assertTabNotRemoved(t, x, testURI)
	})

	t.Run("remove triggers prompt if open, dirty file; user discards, removes tab", func(t *testing.T) {
		var mu sync.Mutex
		ignores := vctrl.NopMatcher(false)

		x := newExForEventTesting(t)
		testURI, err := x.workspace.URI("a")
		require.NoError(t, err)

		createOpenRemoveFile(t, x, testURI, "")

		editBuffer(t, x, testURI, "ABC")

		dirty, ok := x.comp.IsDirty(testURI)
		require.True(t, ok)
		require.True(t, dirty)

		fsev := testEventInfo{e: schemeapi.Remove, u: testURI}
		dispatchFilesystemEvent(x, &mu, ignores, fsev)

		userPromptDiscards(t, x)

		assertTabRemoved(t, x, testURI)
		assertNoPrompt(t, x)
	})

	t.Run("remove triggers prompt if open, dirty file; user overwrites", func(t *testing.T) {
		var mu sync.Mutex
		ignores := vctrl.NopMatcher(false)

		x := newExForEventTesting(t)
		testURI, err := x.workspace.URI("a")
		require.NoError(t, err)

		createOpenRemoveFile(t, x, testURI, "")

		editBuffer(t, x, testURI, "ABC")

		fsev := testEventInfo{e: schemeapi.Remove, u: testURI}
		dispatchFilesystemEvent(x, &mu, ignores, fsev)

		userPromptOverwrites(t, x)

		assertTabNotRemoved(t, x, testURI)
		assertBufferContent(t, x, testURI, "ABC")
		assertNoPrompt(t, x)
	})

	t.Run("rename original file triggers remove tab if open, non-dirty", func(t *testing.T) {
		var mu sync.Mutex
		ignores := vctrl.NopMatcher(false)

		x := newExForEventTesting(t)
		testURI, err := x.workspace.URI("a")
		require.NoError(t, err)

		createOpenRemoveFile(t, x, testURI, "")

		fsev := testEventInfo{e: schemeapi.Rename, u: testURI}
		dispatchFilesystemEvent(x, &mu, ignores, fsev)

		assertTabRemoved(t, x, testURI)
		assertNoPrompt(t, x)
	})

	t.Run("rename target file triggers reload tab if open, non-dirty", func(t *testing.T) {
		var mu sync.Mutex
		ignores := vctrl.NopMatcher(false)

		x := newExForEventTesting(t)
		testURI, err := x.workspace.URI("a")
		require.NoError(t, err)

		createOpenWriteFile(t, x, testURI, "abc")

		res, ok := x.comp.Resource(testURI)
		require.True(t, ok)

		flush, err := x.comp.LastFlush(res)
		require.NoError(t, err)

		fsev := testEventInfo{e: schemeapi.Rename, u: testURI}
		dispatchFilesystemEvent(x, &mu, ignores, fsev)

		x.waitInflight()

		lastFlush, err := x.comp.LastFlush(res)
		require.NoError(t, err)
		assert.NotEqual(t, flush, lastFlush)

		assertBufferContent(t, x, testURI, "abc")
		assertNoPrompt(t, x)
	})

	t.Run("rename target file opens prompt if open, dirty; user discards triggers reload", func(t *testing.T) {
		var mu sync.Mutex
		ignores := vctrl.NopMatcher(false)

		x := newExForEventTesting(t)
		testURI, err := x.workspace.URI("a")
		require.NoError(t, err)

		createOpenWriteFile(t, x, testURI, "abc")

		editBuffer(t, x, testURI, "ABC")

		fsev := testEventInfo{e: schemeapi.Rename, u: testURI}
		dispatchFilesystemEvent(x, &mu, ignores, fsev)

		userPromptDiscards(t, x)

		assertBufferContent(t, x, testURI, "abc")
		assertNoPrompt(t, x)
	})

	t.Run("rename original file opens prompt if open, dirty; user discards triggers remove tab", func(t *testing.T) {
		var mu sync.Mutex
		ignores := vctrl.NopMatcher(false)

		x := newExForEventTesting(t)
		testURI, err := x.workspace.URI("a")
		require.NoError(t, err)

		createOpenRemoveFile(t, x, testURI, "abc")

		editBuffer(t, x, testURI, "ABC")

		fsev := testEventInfo{e: schemeapi.Rename, u: testURI}
		dispatchFilesystemEvent(x, &mu, ignores, fsev)

		userPromptDiscards(t, x)

		assertTabRemoved(t, x, testURI)
		assertNoPrompt(t, x)
	})

	t.Run("rename target file opens prompt if open, dirty; user overwrites", func(t *testing.T) {
		var mu sync.Mutex
		ignores := vctrl.NopMatcher(false)

		x := newExForEventTesting(t)
		testURI, err := x.workspace.URI("a")
		require.NoError(t, err)

		createOpenWriteFile(t, x, testURI, "abc")

		editBuffer(t, x, testURI, "ABC")

		fsev := testEventInfo{e: schemeapi.Rename, u: testURI}
		dispatchFilesystemEvent(x, &mu, ignores, fsev)

		userPromptOverwrites(t, x)

		assertBufferContent(t, x, testURI, "ABC")
		assertNoPrompt(t, x)
	})
}

// TestHandleFSChange_NoPromptForSecondWriteDuringReload guards
// against a spurious "Discard your changes" prompt that fired when
// an external tool (e.g. `git rebase`) wrote to a clean, open file
// multiple times in quick succession.
//
// The first Write kicks off an async reload. The reload's
// buffer-mutation tick (workspace/file.go scheduleNextTick) raises
// OnDidEdit on text.editorFlusherCloser, which used to call
// setDirtyFileAttr unconditionally — flipping the tab to "dirty"
// even though the buffer was being rewritten from disk, not edited
// by the user. A second Write arriving before
// editorFlusherCloser.dispatchFlush ran (it's queued via the host
// scheduler) saw IsDirty=true in handleFSChange and routed to
// openFileChangedPrompt instead of just triggering another reload.
//
// With the fix, OnDidEdit short-circuits while the reload is in
// flight, so the second Write observes IsDirty=false and no prompt
// is opened.
func TestHandleFSChange_NoPromptForSecondWriteDuringReload(t *testing.T) {
	var mu sync.Mutex
	ignores := vctrl.NopMatcher(false)

	x := newExForEventTesting(t)
	testURI, err := x.workspace.URI("a")
	require.NoError(t, err)

	// Open a clean file. createOpenWriteFile writes empty content,
	// opens the buffer, then writes "abc" on disk — leaving the
	// buffer empty and disk modified, which mirrors the
	// "external tool just touched a clean file" precondition.
	createOpenWriteFile(t, x, testURI, "abc")

	dirty, ok := x.comp.IsDirty(testURI)
	require.True(t, ok)
	require.False(t, dirty, "precondition: buffer must be clean before first reload")

	// First Write kicks off the reload. The async worker reloads
	// the file from disk and schedules the buffer mutation onto
	// the workspace scheduler (inline in tests). OnDidEdit fires
	// for each cell-edit and previously raised the dirty
	// attribute mid-reload.
	fsev := testEventInfo{e: schemeapi.Write, u: testURI}
	dispatchFilesystemEvent(x, &mu, ignores, fsev)

	// Drain the async worker so the buffer mutation has run.
	// dispatchFlush (which clears efc.lastFlush) is still queued
	// on the host scheduler and intentionally not drained — this
	// is the precise window the production race opens.
	x.waitInflight()

	// Second Write arrives while the first reload's dispatchFlush
	// is still pending. handleFSChange must observe IsDirty=false
	// and route to startReloadAndNotify, not openFileChangedPrompt.
	require.NoError(t, os.WriteFile(testURI.Path(), []byte("xyz"), 0o666))
	fsev2 := testEventInfo{e: schemeapi.Write, u: testURI}
	dispatchFilesystemEvent(x, &mu, ignores, fsev2)

	// If openFileChangedPrompt opened, the next keypress would be
	// handled by the prompt (returning handled=true). assertNoPrompt
	// verifies it wasn't.
	assertNoPrompt(t, x)

	// Final sanity: drain everything and confirm the buffer
	// reflects the latest disk content.
	x.waitInflight()
	assertBufferContent(t, x, testURI, "xyz")
}

func assertFileContent(t *testing.T, x *ex, file workspaceapi.URI, content string) {
	t.Helper()
	// Reloads, flushes and overwrites are asynchronous; wait for
	// any in-flight ops to settle so the assertion reflects the
	// post-completion state instead of a snapshot mid-flight.
	x.waitInflight()
	data, err := os.ReadFile(file.Path())
	require.NoError(t, err)
	assert.Equal(t, content+"\n", string(data))
}

func assertBufferContent(t *testing.T, x *ex, file workspaceapi.URI, content string) {
	t.Helper()
	x.waitInflight()
	ed, err := x.comp.Editor(file)
	require.NoError(t, err)
	cells := ed.CellView().RawCells()
	assert.Equal(t, content, term.CellsToString(cells))
}

func assertTabRemoved(t *testing.T, x *ex, file workspaceapi.URI) {
	t.Helper()
	_, err := x.comp.Editor(file)
	require.Error(t, err, "tab was not removed")
}

func assertTabNotRemoved(t *testing.T, x *ex, file workspaceapi.URI) {
	_, err := x.comp.Editor(file)
	require.NoError(t, err, "tab was removed")
}

func assertNoPrompt(t *testing.T, x *ex) {
	exit, handled := x.Handle(term.Event{Type: term.EventKey, Ch: 'o'})
	assert.False(t, exit)
	require.False(t, handled)
}

func newExForEventTesting(t *testing.T) *ex {
	ctx := context.Background()
	opts := []text.Option{
		text.WithCommandKey(testCommandKey),
	}

	tempDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(tempDir)
	})
	uri, err := workspaceapi.ParseURI(filepath.Join("file://", tempDir))
	require.NoError(t, err)

	fileScheme, err := workspace.NewFileScheme(ctx, config.NopConfig(), uri)
	require.NoError(t, err)

	workspace := workspace.NewSchemeWorkspace(uri, fileScheme, inlineSchedule)

	e := newExForTestingTerminal(t, workspace, texttest.NopEditor(),
		vte.DefaultConfig(), nopPublishEvent, opts...)

	t.Cleanup(func() {
		require.NoError(t, e.Close())
		require.NoError(t, fileScheme.Close())
	})

	return e.ex
}

type testEventInfo struct {
	e schemeapi.Event
	u workspaceapi.URI
	d bool
}

func (t testEventInfo) Event() schemeapi.Event {
	return t.e
}

func (t testEventInfo) URI() workspaceapi.URI {
	return t.u
}

func (t testEventInfo) IsDir() (bool, error) {
	return t.d, nil
}

// createOpenWriteFile touches a file on disk, opens it for editing (empty) then
// changes the contents on disk to content.
func createOpenWriteFile(t *testing.T, x *ex, file workspaceapi.URI, content string) {
	require.NoError(t, os.WriteFile(file.Path(), []byte(""), 0666))
	require.NoError(t, x.editFiles(bgctx, file.String()))
	require.NoError(t, os.WriteFile(file.Path(), []byte(content), 0666))
}

// createOpenRemoveFile writes content to a file on disk, then opens it for editing
// (non empty, with content), then removes it from disk.
func createOpenRemoveFile(t *testing.T, x *ex, file workspaceapi.URI, content string) {
	require.NoError(t, os.WriteFile(file.Path(), []byte(content), 0666))
	require.NoError(t, x.editFiles(bgctx, file.String()))
	require.NoError(t, os.Remove(file.Path()))
}

// openWriteUncreatedFile open an empty, uncreated file for editing, then changes
// the contents on disk to content.
func openWriteUncreatedFile(t *testing.T, x *ex, file workspaceapi.URI, content string) {
	require.NoError(t, x.editFiles(bgctx, file.String()))
	require.NoError(t, os.WriteFile(file.Path(), []byte("abc"), 0666))
}

func userPromptDiscards(t *testing.T, x *ex) {
	keyCombs := []rune{'d', 'y'}
	for _, key := range keyCombs {
		exit, handled := x.Handle(term.Event{Type: term.EventKey, Ch: key})
		assert.False(t, exit)
		assert.True(t, handled)
	}
}

func userPromptOverwrites(t *testing.T, x *ex) {
	keyCombs := []rune{'o'}
	for _, key := range keyCombs {
		exit, handled := x.Handle(term.Event{Type: term.EventKey, Ch: key})
		assert.False(t, exit)
		assert.True(t, handled)
	}
}

func editBuffer(t *testing.T, x *ex, file workspaceapi.URI, content string) {
	h, err := x.comp.Editor(file)
	require.NoError(t, err)

	ctx := context.Background()
	_, _, _ = h.CellEditor().
		Edit(ctx, term.Coordinates{}, term.Coordinates{}, "ABC")

	dirty, ok := x.comp.IsDirty(file)
	require.True(t, ok)
	require.True(t, dirty)
}
