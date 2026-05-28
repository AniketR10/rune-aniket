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

package text

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
)

// loopRunner serializes scheduled callbacks onto a single goroutine,
// mimicking the host event loop where workspaceManagerHandler.focus
// and .workspaces are mutated and read.
type loopRunner struct {
	ch   chan func()
	done chan struct{}
	wg   sync.WaitGroup
}

func newLoopRunner() *loopRunner {
	l := &loopRunner{
		ch:   make(chan func(), 64),
		done: make(chan struct{}),
	}
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		for {
			select {
			case fn := <-l.ch:
				fn()
			case <-l.done:
				return
			}
		}
	}()
	return l
}

func (l *loopRunner) schedule(fn func()) bool {
	l.ch <- fn
	return true
}

func (l *loopRunner) stop() {
	close(l.done)
	l.wg.Wait()
}

// focusNotifications models notis: every Notify and
// UpdateNotificationProgress reads a focus value that the loop runner
// mutates. Calls off the loop race that read.
type focusNotifications struct {
	focus int
	calls int
}

func (f *focusNotifications) Notify(
	_ browserapi.NotificationLevel, _ string, _ ...any,
) (string, error) {
	_ = f.focus
	f.calls++
	return "id", nil
}

func (f *focusNotifications) NotifyOnce(
	level browserapi.NotificationLevel, msg string, args ...any,
) (string, error) {
	return f.Notify(level, msg, args...)
}

func (f *focusNotifications) UpdateNotificationProgress(
	_, _ string, _, _ int64,
) error {
	_ = f.focus
	f.calls++
	return nil
}

// TestSchedNotifyProgressWriterAvoidsRace exercises the cluster-1
// race: progress samples must hop onto the event-loop goroutine that
// owns the focus field. Without the scheduling, -race flags the
// concurrent access between Notify (called from the writer goroutine)
// and the loop runner's mutation of focus.
func TestSchedNotifyProgressWriterAvoidsRace(t *testing.T) {
	t.Parallel()

	loop := newLoopRunner()
	defer loop.stop()

	fn := &focusNotifications{}
	pw := NewNotifyProgressWriter(fn, nil, "install pkg@1.0", loop.schedule)

	var producerWG sync.WaitGroup
	producerWG.Add(1)
	go func() {
		defer producerWG.Done()
		for i := int64(1); i <= 50; i++ {
			pw.Progress(i, 100, "B")
		}
		pw.Progress(100, 100, "B")
	}()

	// Concurrent mutator on the loop, simulating switchToWorkspace.
	var mutWG sync.WaitGroup
	mutWG.Add(1)
	go func() {
		defer mutWG.Done()
		for i := 0; i < 50; i++ {
			loop.schedule(func() { fn.focus++ })
		}
	}()

	producerWG.Wait()
	mutWG.Wait()

	done := make(chan struct{})
	loop.schedule(func() { close(done) })
	<-done

	require.Greater(t, fn.calls, 0)
}
