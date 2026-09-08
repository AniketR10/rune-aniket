// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package extensionv2

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtensionReadinessSetUnblocksWait(t *testing.T) {
	t.Parallel()

	r := newExtensionReadiness()
	done := make(chan error, 1)
	go func() {
		done <- r.Wait(context.Background())
	}()

	select {
	case err := <-done:
		t.Fatalf("Wait returned before Set: %v", err)
	case <-time.After(25 * time.Millisecond):
	}

	want := errors.New("boom")
	r.Set(want)

	select {
	case err := <-done:
		require.ErrorIs(t, err, want)
	case <-time.After(time.Second):
		t.Fatal("Wait did not return after Set")
	}
}

func TestExtensionReadinessSetIsOneShot(t *testing.T) {
	t.Parallel()

	r := newExtensionReadiness()
	r.Set(nil)
	// Subsequent Set calls must not overwrite the resolved outcome so a late
	// stop cannot clobber a successful handshake.
	r.Set(errors.New("ignored"))

	err := r.Wait(context.Background())
	assert.NoError(t, err)
}

func TestExtensionReadinessWaitRespectsContextCancel(t *testing.T) {
	t.Parallel()

	r := newExtensionReadiness()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- r.Wait(ctx)
	}()

	cancel()

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("Wait did not return after context cancellation")
	}
}

func TestExtensionReadinessWaitReturnsResolvedErrorAfterCancel(t *testing.T) {
	t.Parallel()

	// If Set and context cancellation race, a resolved terminal error should
	// win over ctx.Err() so callers always observe the protocol outcome when
	// one is available.
	r := newExtensionReadiness()
	want := errors.New("boom")
	r.Set(want)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := r.Wait(ctx)
	require.ErrorIs(t, err, want)
}

func TestExtensionReadinessConcurrentWaitersAllWake(t *testing.T) {
	t.Parallel()

	r := newExtensionReadiness()

	const waiters = 4
	var wg sync.WaitGroup
	wg.Add(waiters)
	errs := make([]error, waiters)
	for i := range waiters {
		go func() {
			defer wg.Done()
			errs[i] = r.Wait(context.Background())
		}()
	}

	// Small sleep to ensure goroutines are parked in Wait before Set.
	time.Sleep(25 * time.Millisecond)
	r.Set(nil)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("not all waiters woke after Set")
	}
	for _, err := range errs {
		assert.NoError(t, err)
	}
}
