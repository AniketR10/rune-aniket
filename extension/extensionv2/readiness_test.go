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
