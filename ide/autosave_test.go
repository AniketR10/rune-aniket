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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
)

// recordingFlusher implements autoSaverFlusher and records every URI it is
// asked to flush. All calls happen on the test goroutine because tests
// drain the scheduler queue inline (matching the editor's single-threaded
// event dispatch).
type recordingFlusher struct {
	calls   []workspaceapi.URI
	err     error
	missing map[string]bool
}

func (r *recordingFlusher) Resource(uri workspaceapi.URI) (browserapi.Handler, bool) {
	if r.missing[uri.String()] {
		return nil, false
	}
	return recordingHandler{uri: uri}, true
}

func (r *recordingFlusher) FlushTab(h browserapi.Handler) error {
	rh := h.(recordingHandler)
	r.calls = append(r.calls, rh.uri)
	return r.err
}

// recordingHandler is a minimal browserapi.Handler used to round-trip a
// URI from Resource into FlushTab.
type recordingHandler struct{ uri workspaceapi.URI }

func (recordingHandler) Resize(_, _ int)                          {}
func (recordingHandler) Draw(_ term.Writer)                       {}
func (recordingHandler) Handle(_ term.Event) (exit, handled bool) { return false, false }
func (recordingHandler) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return term.Coordinates{}, 0, false
}
func (recordingHandler) Selection() (string, bool) { return "", false }
func (recordingHandler) Close() error              { return nil }

var _ tui.Handler = recordingHandler{}

// fakeNotifications captures calls into browserapi.Notifications.
type fakeNotifications struct {
	notes []notifRecord
}

type notifRecord struct {
	level browserapi.NotificationLevel
	msg   string
}

func (f *fakeNotifications) Notify(level browserapi.NotificationLevel,
	msg string, _ ...any) (string, error) {
	f.notes = append(f.notes, notifRecord{level: level, msg: msg})
	return "", nil
}

func (f *fakeNotifications) NotifyOnce(level browserapi.NotificationLevel,
	msg string, args ...any) (string, error) {
	return f.Notify(level, msg, args...)
}

func (f *fakeNotifications) UpdateNotificationProgress(_, _ string, _, _ int64) error {
	return nil
}

// queueSched mimics scheduleNextTick by enqueuing scheduled callbacks onto
// a buffered channel. Tests drain the queue inline so all flushURI calls
// run on the test goroutine, matching the editor's single-threaded event
// dispatch.
type queueSched struct{ q chan func() }

func newQueueSched() *queueSched { return &queueSched{q: make(chan func(), 16)} }

func (s *queueSched) sched(fn func()) bool {
	s.q <- fn
	return true
}

// drain pulls the next scheduled callback and runs it. It fails the test
// if no callback shows up within the timeout.
func (s *queueSched) drain(t *testing.T) {
	t.Helper()
	select {
	case fn := <-s.q:
		fn()
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for scheduled flush")
	}
}

// drainAll runs everything currently queued.
func (s *queueSched) drainAll() {
	for {
		select {
		case fn := <-s.q:
			fn()
		default:
			return
		}
	}
}

func mustURI(t *testing.T, raw string) workspaceapi.URI {
	t.Helper()
	u, err := workspaceapi.ParseURI(raw)
	require.NoError(t, err)
	return u
}

func TestAutoSaver_FlushesAfterIdleDelay(t *testing.T) {
	t.Parallel()
	flusher := &recordingFlusher{}
	sched := newQueueSched()
	notif := &fakeNotifications{}
	saver := newAutoSaver(flusher, notif, sched.sched, 10*time.Millisecond)
	uri := mustURI(t, "memory:///tmp/a")

	saver.Handle(context.Background(),
		textapi.Event{Type: textapi.EventTypeEdit, URI: uri})

	sched.drain(t)
	assert.Equal(t, []workspaceapi.URI{uri}, flusher.calls)
	assert.Empty(t, notif.notes)
}

func TestAutoSaver_DebouncesEdits(t *testing.T) {
	t.Parallel()
	flusher := &recordingFlusher{}
	sched := newQueueSched()
	saver := newAutoSaver(flusher, &fakeNotifications{}, sched.sched, 50*time.Millisecond)
	uri := mustURI(t, "memory:///tmp/a")

	for range 5 {
		saver.Handle(context.Background(),
			textapi.Event{Type: textapi.EventTypeEdit, URI: uri})
		time.Sleep(5 * time.Millisecond)
	}

	sched.drain(t)
	// give the goroutine a chance to spuriously fire again.
	time.Sleep(80 * time.Millisecond)
	sched.drainAll()
	assert.Len(t, flusher.calls, 1)
}

func TestAutoSaver_FlushEventCancelsPending(t *testing.T) {
	t.Parallel()
	flusher := &recordingFlusher{}
	sched := newQueueSched()
	saver := newAutoSaver(flusher, &fakeNotifications{}, sched.sched, 50*time.Millisecond)
	uri := mustURI(t, "memory:///tmp/a")

	saver.Handle(context.Background(),
		textapi.Event{Type: textapi.EventTypeEdit, URI: uri})
	saver.Handle(context.Background(),
		textapi.Event{Type: textapi.EventTypeFlush, URI: uri})

	time.Sleep(80 * time.Millisecond)
	sched.drainAll()
	assert.Empty(t, flusher.calls, "manual flush must cancel auto-save")
}

func TestAutoSaver_CloseEventCancelsPending(t *testing.T) {
	t.Parallel()
	flusher := &recordingFlusher{}
	sched := newQueueSched()
	saver := newAutoSaver(flusher, &fakeNotifications{}, sched.sched, 50*time.Millisecond)
	uri := mustURI(t, "memory:///tmp/a")

	saver.Handle(context.Background(),
		textapi.Event{Type: textapi.EventTypeEdit, URI: uri})
	saver.Handle(context.Background(),
		textapi.Event{Type: textapi.EventTypeClose, URI: uri})

	time.Sleep(80 * time.Millisecond)
	sched.drainAll()
	assert.Empty(t, flusher.calls)
}

func TestAutoSaver_PerURIIsolation(t *testing.T) {
	t.Parallel()
	flusher := &recordingFlusher{}
	sched := newQueueSched()
	saver := newAutoSaver(flusher, &fakeNotifications{}, sched.sched, 30*time.Millisecond)
	a := mustURI(t, "memory:///tmp/a")
	b := mustURI(t, "memory:///tmp/b")

	saver.Handle(context.Background(),
		textapi.Event{Type: textapi.EventTypeEdit, URI: a})
	saver.Handle(context.Background(),
		textapi.Event{Type: textapi.EventTypeEdit, URI: b})
	// Cancel only A; B's timer should still fire.
	saver.Handle(context.Background(),
		textapi.Event{Type: textapi.EventTypeFlush, URI: a})

	sched.drain(t)
	time.Sleep(60 * time.Millisecond)
	sched.drainAll()
	assert.Equal(t, []workspaceapi.URI{b}, flusher.calls)
}

func TestAutoSaver_SkipsClosedTab(t *testing.T) {
	t.Parallel()
	flusher := &recordingFlusher{
		missing: map[string]bool{"memory:///tmp/a": true},
	}
	sched := newQueueSched()
	saver := newAutoSaver(flusher, &fakeNotifications{}, sched.sched, 10*time.Millisecond)
	uri := mustURI(t, "memory:///tmp/a")

	saver.Handle(context.Background(),
		textapi.Event{Type: textapi.EventTypeEdit, URI: uri})

	sched.drain(t)
	assert.Empty(t, flusher.calls)
}

func TestAutoSaver_StaleDataNotifies(t *testing.T) {
	t.Parallel()
	flusher := &recordingFlusher{err: workspaceapi.ErrStaleData}
	notif := &fakeNotifications{}
	sched := newQueueSched()
	saver := newAutoSaver(flusher, notif, sched.sched, 10*time.Millisecond)
	uri := mustURI(t, "memory:///tmp/a")

	saver.Handle(context.Background(),
		textapi.Event{Type: textapi.EventTypeEdit, URI: uri})

	sched.drain(t)
	require.Len(t, notif.notes, 1)
	assert.Equal(t, browserapi.LevelWarn, notif.notes[0].level)
}

func TestAutoSaver_NotifiesOnReadOnly(t *testing.T) {
	t.Parallel()
	flusher := &recordingFlusher{err: workspaceapi.ErrFileIsNotWritable}
	notif := &fakeNotifications{}
	sched := newQueueSched()
	saver := newAutoSaver(flusher, notif, sched.sched, 10*time.Millisecond)
	uri := mustURI(t, "memory:///tmp/a")

	saver.Handle(context.Background(),
		textapi.Event{Type: textapi.EventTypeEdit, URI: uri})

	sched.drain(t)
	require.Len(t, notif.notes, 1)
	assert.Equal(t, browserapi.LevelWarn, notif.notes[0].level)
}

func TestAutoSaver_SwallowsInvalidSaveSilently(t *testing.T) {
	t.Parallel()
	flusher := &recordingFlusher{err: textapi.ErrInvalidSave}
	notif := &fakeNotifications{}
	sched := newQueueSched()
	saver := newAutoSaver(flusher, notif, sched.sched, 10*time.Millisecond)
	uri := mustURI(t, "memory:///tmp/a")

	saver.Handle(context.Background(),
		textapi.Event{Type: textapi.EventTypeEdit, URI: uri})

	sched.drain(t)
	assert.Empty(t, notif.notes,
		"non-file tabs (e.g. terminals) must not produce a notification")
}
