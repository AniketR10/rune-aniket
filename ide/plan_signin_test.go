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
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"github.com/unstablebuild/rune-go-sdk/handler/handlertest"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/ide/idelockdown"
	"unstable.build/go-tui/ide/ideplan"
)

// stubSignIn is a PlanSourceConfig.SignIn hook that hands the test
// control over the URL/Done channels of each sign-in attempt.
type stubSignIn struct {
	mu     sync.Mutex
	calls  int
	ctx    context.Context
	urlCh  chan *url.URL
	doneCh chan error
}

func (s *stubSignIn) start(ctx context.Context) SignInSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.ctx = ctx
	s.urlCh = make(chan *url.URL, 1)
	s.doneCh = make(chan error, 1)
	return SignInSession{URL: s.urlCh, Done: s.doneCh}
}

func (s *stubSignIn) started() bool { return s.callCount() > 0 }

func (s *stubSignIn) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func (s *stubSignIn) ctxErr() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx == nil {
		return nil
	}
	return s.ctx.Err()
}

func (s *stubSignIn) sendURL(u *url.URL) {
	s.mu.Lock()
	ch := s.urlCh
	s.mu.Unlock()
	ch <- u
	close(ch)
}

func (s *stubSignIn) finish(err error) {
	s.mu.Lock()
	ch := s.doneCh
	s.mu.Unlock()
	// apiclient.Login always sends the result (nil on success)
	// before closing Done.
	ch <- err
	close(ch)
}

// newSignInTestIDE builds a full IDE wired with the stub sign-in hook
// and an expired plan source, returning the drain function of the
// test scheduler and the root handler wrapped for safe Draw/Handle.
func newSignInTestIDE(t *testing.T) (
	i *IDE, stub *stubSignIn, cfg PlanSourceConfig,
	wrapped *lockedHandler, drain func(),
) {
	return newSignInTestIDEUsage(t, false)
}

// newSignInTestIDEUsage builds the sign-in test IDE, optionally
// seeding a qualifying usage history so the live usage planner locks
// the overlay (expired + qualifying). Tests that exercise the
// unlocked path pass seedLocked=false so the planner keeps the IDE
// unlocked.
func newSignInTestIDEUsage(t *testing.T, seedLocked bool) (
	i *IDE, stub *stubSignIn, cfg PlanSourceConfig,
	wrapped *lockedHandler, drain func(),
) {
	t.Helper()
	configFile, _ := makeTestFiles(t)
	dataDir := t.TempDir()
	mu := new(sync.Mutex)
	sched, drain := newTestScheduler(mu)
	stub = &stubSignIn{}
	cfg = PlanSourceConfig{
		Source: staticPlanSource{dec: ideplan.Decision{Status: ideplan.StatusExpired}},
		SignIn: stub.start,
	}
	storage := newTestStorage(t, dataDir)
	if seedLocked {
		// A qualifying usage history makes the live usage planner
		// evaluate to ActionLockdown; without it the planner
		// intercepts the monitor's expired-lock and unlocks, since
		// expired alone no longer locks under the usage-based paywall.
		require.NoError(t,
			idelockdown.SeedQualifyingUsage(context.Background(), storage, time.Now()))
	}
	i, err := New(t.TempDir(), configFile.Name(), dataDir,
		storage,
		WithLocker(mu),
		WithScheduleNextTick(sched),
		WithPlanSource(cfg),
	)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, i.Close()) })
	wrapped = &lockedHandler{Handler: i.Ready(), mu: mu}
	wrapped.Resize(100, 30)
	return i, stub, cfg, wrapped, drain
}

func frameContains(wrapped *lockedHandler, drain func(), substr string) func() bool {
	return func() bool {
		drain()
		return strings.Contains(handlertest.DrawHandler(wrapped, 100, 30), substr)
	}
}

// lockIDEForTest drives the IDE into the locked state through the
// real monitor/planLocker path and waits for the seeded qualifying
// usage to make the lock stick.
func lockIDEForTest(t *testing.T, i *IDE, wrapped *lockedHandler, drain func()) {
	t.Helper()
	require.Eventually(t, func() bool {
		i.TickPlan(context.Background())
		return frameContains(wrapped, drain, "license has lapsed")()
	}, 5*time.Second, 10*time.Millisecond,
		"seeded qualifying usage while expired must lock with the default prompt")
}

// TestPlanSignInLockdownFlowShowsURLCopiesAndReevaluates drives the
// lockdown Sign in button end-to-end: the flow must surface immediate
// feedback above the gray fade (notifications render beneath it),
// expose the OAuth URL with a copy fallback for when the browser does
// not open, confirm the copy inside the prompt itself, and
// re-evaluate plan gating once the flow completes.
func TestPlanSignInLockdownFlowShowsURLCopiesAndReevaluates(t *testing.T) {
	i, stub, _, wrapped, drain := newSignInTestIDEUsage(t, true)

	lockIDEForTest(t, i, wrapped, drain)

	wrapped.Handle(term.Event{Type: term.EventKey, Ch: 's'})
	require.Eventually(t, frameContains(wrapped, drain, "Signing you in"),
		5*time.Second, 10*time.Millisecond,
		"Sign in must swap the overlay prompt to a waiting state")
	require.Eventually(t, stub.started, 5*time.Second, 10*time.Millisecond)

	u, err := url.Parse("https://auth.example/x")
	require.NoError(t, err)
	stub.sendURL(u)
	require.Eventually(t, frameContains(wrapped, drain, "copy this link"),
		5*time.Second, 10*time.Millisecond,
		"the OAuth URL must be offered for copying above the overlay")
	assert.Contains(t, handlertest.DrawHandler(wrapped, 100, 30), "auth.example")

	wrapped.Handle(term.Event{Type: term.EventKey, Ch: 'p'})
	require.Eventually(t, frameContains(wrapped, drain, "URL copied to clipboard"),
		5*time.Second, 10*time.Millisecond,
		"copy confirmation must render in the prompt, not as a notification")
	data, err := i.workspaceHandler.clip.Paste(clipboard.DefaultRegisterID)
	require.NoError(t, err)
	assert.Equal(t, "https://auth.example/x", data.Text)

	// Success re-evaluates gating via TickPlan and dismisses the wait
	// prompt. The source is still expired with qualifying usage, so
	// the default lockdown prompt returns rather than the overlay
	// tearing down.
	stub.finish(nil)
	require.Eventually(t, frameContains(wrapped, drain, "license has lapsed"),
		5*time.Second, 10*time.Millisecond,
		"successful sign-in must dismiss the wait prompt and re-evaluate")
	assert.NotContains(t, handlertest.DrawHandler(wrapped, 100, 30), "auth.example",
		"the sign-in wait prompt must be gone after the flow completes")
}

// TestPlanSignInLockdownCancelRestoresPromptAndRetries proves Cancel
// aborts the OAuth flow, restores the default lockdown prompt without
// unlocking, and releases the flow guard so Sign in can be retried.
func TestPlanSignInLockdownCancelRestoresPromptAndRetries(t *testing.T) {
	i, stub, _, wrapped, drain := newSignInTestIDEUsage(t, true)

	lockIDEForTest(t, i, wrapped, drain)
	wrapped.Handle(term.Event{Type: term.EventKey, Ch: 's'})
	require.Eventually(t, frameContains(wrapped, drain, "Signing you in"),
		5*time.Second, 10*time.Millisecond)
	require.Eventually(t, stub.started, 5*time.Second, 10*time.Millisecond)

	wrapped.Handle(term.Event{Type: term.EventKey, Ch: 'c'})
	require.Eventually(t, func() bool { return stub.ctxErr() != nil },
		5*time.Second, 10*time.Millisecond,
		"Cancel must cancel the sign-in context")
	// Mimic the login flow resolving with the cancellation.
	stub.finish(stub.ctxErr())

	require.Eventually(t, frameContains(wrapped, drain, "license has lapsed"),
		5*time.Second, 10*time.Millisecond,
		"cancel must restore the default lockdown prompt")
	assert.True(t, i.planLockdown.Locked(), "cancel must not unlock the IDE")

	wrapped.Handle(term.Event{Type: term.EventKey, Ch: 's'})
	require.Eventually(t, func() bool { return stub.callCount() == 2 },
		5*time.Second, 10*time.Millisecond,
		"a cancelled flow must release the guard so sign-in can be retried")
}

// TestPlanSignInUnlockedUsesWorkspacePrompt covers the nag-prompt
// path: when the IDE is not locked the wait prompt opens as a
// floating workspace prompt and closes when the flow finishes. Also
// pins that a second sign-in is ignored while one is in flight.
func TestPlanSignInUnlockedUsesWorkspacePrompt(t *testing.T) {
	i, stub, cfg, wrapped, drain := newSignInTestIDE(t)

	i.startPlanSignIn(cfg)
	require.Eventually(t, frameContains(wrapped, drain, "Signing you in"),
		5*time.Second, 10*time.Millisecond,
		"unlocked sign-in must open a workspace wait prompt")

	i.startPlanSignIn(cfg)
	assert.Equal(t, 1, stub.callCount(),
		"a second sign-in while one is in flight must be ignored")

	u, err := url.Parse("https://auth.example/x")
	require.NoError(t, err)
	stub.sendURL(u)
	require.Eventually(t, frameContains(wrapped, drain, "auth.example"),
		5*time.Second, 10*time.Millisecond,
		"the OAuth URL must be offered in the workspace prompt")

	stub.finish(nil)
	require.Eventually(t, func() bool {
		drain()
		return !strings.Contains(
			handlertest.DrawHandler(wrapped, 100, 30), "auth.example")
	}, 5*time.Second, 10*time.Millisecond,
		"the wait prompt must close when the flow finishes")
}
