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

package idenag

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
)

const testWait = 5 * time.Second

// seedLastNag writes a last-nag record dated at t.
func seedLastNag(t *testing.T, store storageapi.Service, at time.Time) {
	t.Helper()
	doc := nagDoc{Kind: docKind, LastNag: at.UTC().Format(time.RFC3339Nano)}
	require.NoError(t, store.Set(context.Background(), docID, &doc))
}

func storedLastNag(t *testing.T, store storageapi.Service) (time.Time, bool) {
	t.Helper()
	var doc nagDoc
	err := store.Get(context.Background(), docID, &doc)
	if err != nil {
		require.ErrorIs(t, err, storageapi.ErrNotFound)
		return time.Time{}, false
	}
	last, perr := time.Parse(time.RFC3339Nano, doc.LastNag)
	require.NoError(t, perr)
	return last, true
}

func waitShown(t *testing.T, shown <-chan State) State {
	t.Helper()
	select {
	case state := <-shown:
		return state
	case <-time.After(testWait):
		t.Fatal("prompt was not shown")
		return 0
	}
}

func TestStartFirstRunRecordsWithoutPrompting(t *testing.T) {
	store := storagestub.NewInMemoryService()
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	shown := make(chan State, 1)
	s := New(Config{
		Storage:   store,
		State:     func(context.Context) State { return StateSignedOut },
		Show:      func(state State) { shown <- state },
		Now:       func() time.Time { return now },
		Interval:  time.Hour,
		BootDelay: time.Millisecond,
	})
	defer s.Stop()
	require.NoError(t, s.Start(context.Background()))

	last, ok := storedLastNag(t, store)
	require.True(t, ok)
	assert.Equal(t, now, last)

	// The first prompt waits a full interval: nothing may fire at
	// BootDelay cadence.
	select {
	case <-shown:
		t.Fatal("first run must not prompt")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestStartOverduePromptsAfterBootDelay(t *testing.T) {
	tests := []struct {
		name  string
		state State
	}{
		{"signed out", StateSignedOut},
		{"no plan", StateNoPlan},
		{"expired", StateExpired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := storagestub.NewInMemoryService()
			now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
			seedLastNag(t, store, now.Add(-2*time.Hour))
			shown := make(chan State, 1)
			s := New(Config{
				Storage:   store,
				State:     func(context.Context) State { return tt.state },
				Show:      func(state State) { shown <- state },
				Now:       func() time.Time { return now },
				Interval:  time.Hour,
				BootDelay: time.Millisecond,
			})
			defer s.Stop()
			require.NoError(t, s.Start(context.Background()))

			assert.Equal(t, tt.state, waitShown(t, shown))
			last, ok := storedLastNag(t, store)
			require.True(t, ok)
			assert.Equal(t, now, last)
		})
	}
}

func TestStartFreshRecordWaitsRemainingInterval(t *testing.T) {
	store := storagestub.NewInMemoryService()
	now := time.Now()
	// 50ms interval with a nag 30ms ago: fires after ~20ms, well
	// before the 1h boot delay could.
	seedLastNag(t, store, now.Add(-30*time.Millisecond))
	shown := make(chan State, 1)
	s := New(Config{
		Storage:   store,
		State:     func(context.Context) State { return StateNoPlan },
		Show:      func(state State) { shown <- state },
		Interval:  50 * time.Millisecond,
		BootDelay: time.Hour,
	})
	defer s.Stop()
	require.NoError(t, s.Start(context.Background()))

	assert.Equal(t, StateNoPlan, waitShown(t, shown))
}

func TestFireSkipsPromptForActivePlan(t *testing.T) {
	store := storagestub.NewInMemoryService()
	start := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	seedLastNag(t, store, start.Add(-2*time.Hour))
	fired := make(chan struct{}, 1)
	now := start
	s := New(Config{
		Storage: store,
		State: func(context.Context) State {
			// Advance the clock inside the fire so the recorded
			// timestamp proves the fire path ran.
			now = start.Add(time.Minute)
			select {
			case fired <- struct{}{}:
			default:
			}
			return StateActive
		},
		Show:      func(State) { t.Error("active plan must not prompt") },
		Now:       func() time.Time { return now },
		Interval:  time.Hour,
		BootDelay: time.Millisecond,
	})
	defer s.Stop()
	require.NoError(t, s.Start(context.Background()))

	select {
	case <-fired:
	case <-time.After(testWait):
		t.Fatal("timer did not fire")
	}
	require.Eventually(t, func() bool {
		last, ok := storedLastNag(t, store)
		return ok && last.Equal(start.Add(time.Minute))
	}, testWait, time.Millisecond)
}

func TestFireReschedulesNextInterval(t *testing.T) {
	store := storagestub.NewInMemoryService()
	seedLastNag(t, store, time.Now().Add(-time.Hour))
	shown := make(chan State, 2)
	s := New(Config{
		Storage:   store,
		State:     func(context.Context) State { return StateSignedOut },
		Show:      func(state State) { shown <- state },
		Interval:  20 * time.Millisecond,
		BootDelay: time.Millisecond,
	})
	defer s.Stop()
	require.NoError(t, s.Start(context.Background()))

	waitShown(t, shown)
	waitShown(t, shown)
}

func TestStopCancelsPendingPrompt(t *testing.T) {
	store := storagestub.NewInMemoryService()
	seedLastNag(t, store, time.Now().Add(-time.Hour))
	shown := make(chan State, 1)
	s := New(Config{
		Storage:   store,
		State:     func(context.Context) State { return StateSignedOut },
		Show:      func(state State) { shown <- state },
		Interval:  time.Hour,
		BootDelay: 250 * time.Millisecond,
	})
	require.NoError(t, s.Start(context.Background()))
	s.Stop()

	select {
	case <-shown:
		t.Fatal("stopped scheduler must not prompt")
	case <-time.After(500 * time.Millisecond):
	}
}

func TestStartMalformedRecordTreatedAsFirstRun(t *testing.T) {
	store := storagestub.NewInMemoryService()
	doc := nagDoc{Kind: docKind, LastNag: "not-a-time"}
	require.NoError(t, store.Set(context.Background(), docID, &doc))
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	s := New(Config{
		Storage:   store,
		State:     func(context.Context) State { return StateSignedOut },
		Show:      func(State) { t.Error("first run must not prompt") },
		Now:       func() time.Time { return now },
		Interval:  time.Hour,
		BootDelay: time.Millisecond,
	})
	defer s.Stop()
	require.NoError(t, s.Start(context.Background()))

	last, ok := storedLastNag(t, store)
	require.True(t, ok)
	assert.Equal(t, now, last)
	time.Sleep(50 * time.Millisecond)
}
