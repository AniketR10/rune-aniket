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
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/debug"
)

// immediateFlusherTarget completes every async op from its own
// goroutine so the flusher's awaiter stays in flight long enough for a
// concurrent wait to block, which is the interleaving the workspace
// handler hits when the FS watcher starts a reload mid-drain.
type immediateFlusherTarget struct{}

func (immediateFlusherTarget) done() (<-chan error, error) {
	ch := make(chan error, 1)
	go debug.CapturePanicReport(func() {
		runtime.Gosched()
		ch <- nil
		close(ch)
	})
	return ch, nil
}

func (t immediateFlusherTarget) FlushTab(
	context.Context, browserapi.Handler,
) (<-chan error, error) {
	return t.done()
}

func (t immediateFlusherTarget) ForceFlushTab(
	context.Context, browserapi.Handler,
) (<-chan error, error) {
	return t.done()
}

func (t immediateFlusherTarget) OverwriteTab(
	context.Context, browserapi.Handler,
) (<-chan error, error) {
	return t.done()
}

func (t immediateFlusherTarget) ReloadTab(
	context.Context, browserapi.Handler,
) (<-chan error, error) {
	return t.done()
}

func (immediateFlusherTarget) Resource(
	workspaceapi.URI,
) (browserapi.Handler, bool) {
	return nil, false
}

// TestFlusherWaitConcurrentWithStart reproduces the workspace-handler
// race: the FS watcher goroutine starts a reload (flusher bookkeeping
// going 0 -> 1) while the drain helper waits for in-flight ops from
// another goroutine. Both must be safe to call concurrently.
func TestFlusherWaitConcurrentWithStart(t *testing.T) {
	f := newFlusher(immediateFlusherTarget{}, &fakeNotifications{},
		func(func()) bool { return true })
	uri, err := workspaceapi.ParseURI("memory:///race.go")
	require.NoError(t, err)

	const iterations = 500
	var done sync.WaitGroup
	done.Add(2)
	go debug.CapturePanicReport(func() {
		defer done.Done()
		for range iterations {
			_ = f.reloadAsync(uri, nil, nil)
		}
	})
	go debug.CapturePanicReport(func() {
		defer done.Done()
		for range iterations {
			f.wait()
		}
	})
	done.Wait()

	f.wait()
	assert.Zero(t, f.inFlightCount(),
		"every awaiter must be accounted for once wait returns")
}
