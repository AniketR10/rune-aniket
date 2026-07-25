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
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/ide/pkgtrust"
)

func TestPromptChoiceCompletion(t *testing.T) {
	tests := []struct {
		name     string
		complete func(*promptChoiceCompletion)
		cancel   bool
		want     int
		wantErr  error
	}{
		{
			name: "selection before close",
			complete: func(completion *promptChoiceCompletion) {
				completion.selectIndex(2)
				completion.dismiss()
			},
			want: 2,
		},
		{
			name: "close without selection",
			complete: func(completion *promptChoiceCompletion) {
				completion.dismiss()
			},
			want:    -1,
			wantErr: context.Canceled,
		},
		{
			name: "first terminal outcome wins",
			complete: func(completion *promptChoiceCompletion) {
				completion.selectIndex(1)
				completion.dismiss()
				completion.selectIndex(0)
				completion.dismiss()
			},
			want: 1,
		},
		{
			name:    "caller cancellation",
			cancel:  true,
			want:    -1,
			wantErr: context.Canceled,
		},
		{
			name: "completed outcome beats observed cancellation",
			complete: func(completion *promptChoiceCompletion) {
				completion.selectIndex(3)
			},
			cancel: true,
			want:   3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			completion := newPromptChoiceCompletion()
			if tt.complete != nil {
				tt.complete(completion)
			}

			ctx, cancel := context.WithCancel(context.Background())
			if tt.cancel {
				cancel()
			} else {
				defer cancel()
			}

			got, err := completion.wait(ctx)
			assert.Equal(t, tt.want, got)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}

	t.Run("caller cancellation remains observable", func(t *testing.T) {
		completion := newPromptChoiceCompletion()
		ctx, cancel := context.WithTimeout(context.Background(), 0)
		defer cancel()

		got, err := completion.wait(ctx)
		assert.Equal(t, -1, got)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})
}

func TestPromptChoiceBrowserLifecycle(t *testing.T) {
	dataDir := t.TempDir()
	workspaceDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspaceDir, "seed.txt"), nil, 0o666))

	configFile, _ := makeTestFiles(t)
	mu := new(sync.Mutex)
	scheduleNextTick, drain := newTestScheduler(mu)
	i, err := New(
		workspaceDir,
		configFile.Name(),
		dataDir,
		pkgtrust.NewStore(dataDir, nil), newTestStorage(t, dataDir),
		WithPublishEvent(nopPublishEvent),
		WithExtensionsRunner(FuncExtensionsRunner(testRunnerFn)),
		WithLocker(mu),
		WithScheduleNextTick(scheduleNextTick),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, i.Close()) })

	root := i.Ready()
	mu.Lock()
	root.Resize(80, 24)
	mu.Unlock()
	i.WaitWorkspaces()
	drain()

	type promptResult struct {
		index int
		err   error
	}
	openerScheduled := make(chan struct{})
	var openerScheduledOnce sync.Once
	i.scheduleFn = func(fn func()) bool {
		scheduled := scheduleNextTick(fn)
		openerScheduledOnce.Do(func() { close(openerScheduled) })
		return scheduled
	}
	result := make(chan promptResult, 1)
	go debug.CapturePanicReport(func() {
		index, promptErr := newWorkspaceWindowManagerUI(i).PromptChoice(
			context.Background(), "Trust this host key?", []string{"Trust once"},
		)
		result <- promptResult{index: index, err: promptErr}
	})

	<-openerScheduled
	drain()
	mu.Lock()
	floatingBefore := i.workspaceHandler.focusEx().comp.Browser().FloatingWindows()
	mu.Unlock()
	require.Equal(t, 1, floatingBefore)

	mu.Lock()
	root.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	floatingAfter := i.workspaceHandler.focusEx().comp.Browser().FloatingWindows()
	mu.Unlock()
	assert.Zero(t, floatingAfter)

	got := <-result
	assert.Equal(t, 0, got.index)
	assert.NoError(t, got.err)
}
