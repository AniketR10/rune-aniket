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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	blueauth "github.com/unstablebuild/blue/auth"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"google.golang.org/grpc/peer"
	"unstable.build/go-tui/extension/extensionv2/peerprocess"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/text/texttest"
)

const testWindowManagerResource = "/browser.WindowManager/NewWindow"

type stubPermissionPrompter struct {
	decision PermissionDecision
	err      error
	mutate   bool
	calls    int
	requests []PermissionRequest
}

func (s *stubPermissionPrompter) PromptPermission(
	ctx context.Context, req PermissionRequest,
) (PermissionDecision, error) {
	s.calls++
	if s.mutate && len(req.Args) > 0 {
		req.Args[0] = "mutated"
	}
	s.requests = append(s.requests, req)
	return s.decision, s.err
}

type errorStorage struct {
	storageapi.Service
	getErr error
	setErr error
}

func (s errorStorage) Get(ctx context.Context, ID string, doc any) error {
	if s.getErr != nil {
		return s.getErr
	}
	return s.Service.Get(ctx, ID, doc)
}

func (s errorStorage) Set(ctx context.Context, ID string, doc any) error {
	if s.setErr != nil {
		return s.setErr
	}
	return s.Service.Set(ctx, ID, doc)
}

func TestAuthorizerRegularExtensionPromptsForClaimedPermission(t *testing.T) {
	t.Parallel()

	prompter := &stubPermissionPrompter{decision: PermissionAllowOnce}
	a := mustNewAuthorizer(t, prompter, storagestub.NewInMemoryService(), texttest.NopEditor())
	ext := testRegularExtension(extensionapi.NewPermissions(
		extensionapi.PermissionBrowserWindowManager,
	))

	err := a.Authorize(context.Background(), blueauth.UserClaims[Extension]{Extra: ext},
		testWindowManagerResource)
	require.NoError(t, err)
	require.Equal(t, 1, prompter.calls)
	assert.Equal(t, "test-extension", prompter.requests[0].ExtensionID)
	assert.Equal(t, "Test Extension", prompter.requests[0].ExtensionName)
	assert.Equal(t, "dev-id", prompter.requests[0].DeveloperID)
	assert.Equal(t, extensionapi.PermissionBrowserWindowManager,
		prompter.requests[0].Permission)
	assert.Equal(t, "test-extension", prompter.requests[0].Path)

	err = a.Authorize(context.Background(), blueauth.UserClaims[Extension]{Extra: ext},
		testWindowManagerResource)
	require.NoError(t, err)
	assert.Equal(t, 1, prompter.calls)
}

func TestAuthorizerRegularExtensionMissingClaimForbiddenWithoutPrompt(t *testing.T) {
	t.Parallel()

	prompter := &stubPermissionPrompter{decision: PermissionAllowOnce}
	a := mustNewAuthorizer(t, prompter, storagestub.NewInMemoryService(), texttest.NopEditor())
	ext := testRegularExtension(extensionapi.NewPermissions(extensionapi.PermissionStorage))

	err := a.Authorize(context.Background(), blueauth.UserClaims[Extension]{Extra: ext},
		testWindowManagerResource)
	assert.ErrorIs(t, err, blueauth.ErrForbidden)
	assert.Zero(t, prompter.calls)
}

func TestAuthorizerUnknownResourceForbidden(t *testing.T) {
	t.Parallel()

	a := mustNewAuthorizer(t, nil, nil, texttest.NopEditor())
	err := a.Authorize(context.Background(), blueauth.UserClaims[Extension]{},
		"/unknown.Service/Method")
	assert.ErrorIs(t, err, blueauth.ErrForbidden)
}

func TestAuthorizerPluginPromptsAndIgnoresClaimsPermissions(t *testing.T) {
	t.Parallel()

	prompter := &stubPermissionPrompter{
		decision: PermissionAllowOnce,
	}
	a := mustNewAuthorizer(t, prompter, storagestub.NewInMemoryService(), texttest.NopEditor())
	ext := testPluginExtension(extensionapi.NewPermissions(extensionapi.PermissionStorage))

	err := a.Authorize(context.Background(), blueauth.UserClaims[Extension]{Extra: ext},
		testWindowManagerResource)
	require.NoError(t, err)
	require.Equal(t, 1, prompter.calls)
	require.Len(t, prompter.requests, 1)
	assert.Equal(t, "/bin/test", prompter.requests[0].Path)
	assert.Equal(t, []string{"--flag"}, prompter.requests[0].Args)
	assert.Equal(t, extensionapi.PermissionBrowserWindowManager,
		prompter.requests[0].Permission)
	assert.Equal(t, testWindowManagerResource, prompter.requests[0].Resource)
	assert.Equal(t, []string{"--flag"}, ext.Args)
}

func TestAuthorizerPluginUsesPeerProcessForPromptAndStorageKey(t *testing.T) {
	t.Parallel()

	storage := storagestub.NewInMemoryService()
	prompter := &stubPermissionPrompter{decision: PermissionAllowAlways}
	a := mustNewAuthorizer(t, prompter, storage, texttest.NopEditor())
	ext := Extension{
		Metadata: extensionapi.Metadata{Permissions: extensionapi.AllPermissions()},
		Plugin:   true,
		Path:     "/bin/zsh",
		Args:     []string{"--login"},
	}
	ctx := contextWithPeerProcess(context.Background(), peerprocess.Process{
		PID:  123,
		UID:  501,
		Exe:  "/usr/local/bin/trusted-cli",
		Argv: []string{"trusted-cli", "run", "--verbose"},
	})

	err := a.Authorize(ctx, blueauth.UserClaims[Extension]{Extra: ext},
		testWindowManagerResource)
	require.NoError(t, err)
	require.Equal(t, 1, prompter.calls)
	require.Len(t, prompter.requests, 1)
	assert.Equal(t, "/usr/local/bin/trusted-cli", prompter.requests[0].Path)
	assert.Equal(t, []string{"run", "--verbose"}, prompter.requests[0].Args)
	assert.Equal(t, "/bin/zsh", prompter.requests[0].LauncherPath)
	assert.Equal(t, []string{"--login"}, prompter.requests[0].LauncherArgs)

	peerKey := pluginPermissionStorageKey("/usr/local/bin/trusted-cli",
		[]string{"run", "--verbose"}, extensionapi.PermissionBrowserWindowManager)
	var stored storedPermissionDecision
	require.NoError(t, storage.Get(context.Background(), peerKey, &stored))
	assert.Equal(t, pluginPermissionDecisionAllow, stored.Decision)

	launcherKey := pluginPermissionStorageKey("/bin/zsh", []string{"--login"},
		extensionapi.PermissionBrowserWindowManager)
	err = storage.Get(context.Background(), launcherKey, &stored)
	assert.ErrorIs(t, err, storageapi.ErrNotFound)
}

func TestAuthorizerPluginOmitsLauncherWhenPeerMatchesClaim(t *testing.T) {
	t.Parallel()

	prompter := &stubPermissionPrompter{decision: PermissionAllowOnce}
	a := mustNewAuthorizer(t, prompter, storagestub.NewInMemoryService(), texttest.NopEditor())
	ext := Extension{
		Plugin: true,
		Path:   "/usr/local/bin/runectl",
		Args:   []string{"wm", "focus"},
	}
	ctx := contextWithPeerProcess(context.Background(), peerprocess.Process{
		Exe:  "/usr/local/bin/runectl",
		Argv: []string{"runectl", "wm", "focus"},
	})

	err := a.Authorize(ctx, blueauth.UserClaims[Extension]{Extra: ext},
		testWindowManagerResource)
	require.NoError(t, err)
	require.Equal(t, 1, prompter.calls)
	assert.Equal(t, "/usr/local/bin/runectl", prompter.requests[0].Path)
	assert.Equal(t, []string{"wm", "focus"}, prompter.requests[0].Args)
	assert.Empty(t, prompter.requests[0].LauncherPath)
	assert.Empty(t, prompter.requests[0].LauncherArgs)
}

func TestAuthorizerPluginPeerDecisionIsIndependentFromLauncherDecision(t *testing.T) {
	t.Parallel()

	storage := storagestub.NewInMemoryService()
	ext := Extension{
		Plugin: true,
		Path:   "/bin/zsh",
		Args:   []string{"--login"},
	}
	launcherKey := pluginPermissionStorageKey(ext.Path, ext.Args,
		extensionapi.PermissionBrowserWindowManager)
	require.NoError(t, storage.Set(context.Background(), launcherKey,
		storedPermissionDecision{Decision: pluginPermissionDecisionAllow}))
	prompter := &stubPermissionPrompter{decision: PermissionDenyOnce}
	a := mustNewAuthorizer(t, prompter, storage, texttest.NopEditor())
	ctx := contextWithPeerProcess(context.Background(), peerprocess.Process{
		Exe:  "/usr/local/bin/untrusted-cli",
		Argv: []string{"untrusted-cli"},
	})

	err := a.Authorize(ctx, blueauth.UserClaims[Extension]{Extra: ext},
		testWindowManagerResource)
	assert.ErrorIs(t, err, blueauth.ErrForbidden)
	assert.Equal(t, 1, prompter.calls)
	assert.Equal(t, "/usr/local/bin/untrusted-cli", prompter.requests[0].Path)
}

func TestAuthorizerPluginFallsBackWhenPeerProcessUnavailable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		ctx  context.Context
	}{
		{name: "no peer", ctx: context.Background()},
		{name: "peer error", ctx: peer.NewContext(context.Background(), &peer.Peer{
			Addr: peerProcessAddr{
				process: peerprocess.Process{Exe: "/usr/local/bin/client", Argv: []string{"client"}},
				err:     errors.New("attribution failed"),
			},
		})},
		{name: "peer without program", ctx: contextWithPeerProcess(context.Background(),
			peerprocess.Process{PID: 123})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			prompter := &stubPermissionPrompter{
				decision: PermissionAllowOnce,
			}
			a := mustNewAuthorizer(t, prompter, storagestub.NewInMemoryService(), texttest.NopEditor())
			ext := testPluginExtension(nil)

			err := a.Authorize(tc.ctx, blueauth.UserClaims[Extension]{Extra: ext},
				testWindowManagerResource)
			require.NoError(t, err)
			require.Equal(t, 1, prompter.calls)
			assert.Equal(t, ext.Path, prompter.requests[0].Path)
			assert.Equal(t, ext.Args, prompter.requests[0].Args)
			assert.Empty(t, prompter.requests[0].LauncherPath)
		})
	}
}

func TestAuthorizerPluginPromptArgsAreCopied(t *testing.T) {
	t.Parallel()

	prompter := &stubPermissionPrompter{
		decision: PermissionAllowOnce,
		mutate:   true,
	}
	a := mustNewAuthorizer(t, prompter, storagestub.NewInMemoryService(), texttest.NopEditor())
	ext := testPluginExtension(nil)

	err := a.Authorize(context.Background(), blueauth.UserClaims[Extension]{Extra: ext},
		testWindowManagerResource)
	require.NoError(t, err)
	assert.Equal(t, []string{"--flag"}, ext.Args)
}

func TestAuthorizerPluginPromptDenialForbidden(t *testing.T) {
	t.Parallel()

	prompter := &stubPermissionPrompter{decision: PermissionDenyOnce}
	a := mustNewAuthorizer(t, prompter, storagestub.NewInMemoryService(), texttest.NopEditor())
	err := a.Authorize(context.Background(), blueauth.UserClaims[Extension]{
		Extra: testPluginExtension(extensionapi.AllPermissions()),
	}, testWindowManagerResource)
	assert.ErrorIs(t, err, blueauth.ErrForbidden)
}

func TestAuthorizerPluginCachesOnceDecisionsForPeerProcess(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		decision  PermissionDecision
		wantError error
	}{
		{name: "allow once", decision: PermissionAllowOnce},
		{name: "deny once", decision: PermissionDenyOnce,
			wantError: blueauth.ErrForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			prompter := &stubPermissionPrompter{decision: tc.decision}
			a := mustNewAuthorizer(t, prompter, storagestub.NewInMemoryService(), texttest.NopEditor())
			ext := testPluginExtension(nil)
			ctx := contextWithPeerProcess(context.Background(), peerprocess.Process{
				PID:  123,
				UID:  501,
				Exe:  "/usr/local/bin/plugin-cli",
				Argv: []string{"plugin-cli", "wm", "focus"},
			})

			for range 2 {
				err := a.Authorize(ctx, blueauth.UserClaims[Extension]{Extra: ext},
					testWindowManagerResource)
				if tc.wantError != nil {
					assert.ErrorIs(t, err, tc.wantError)
				} else {
					assert.NoError(t, err)
				}
			}
			assert.Equal(t, 1, prompter.calls)
		})
	}
}

func TestAuthorizerPluginOnceDecisionCacheIncludesPeerPID(t *testing.T) {
	t.Parallel()

	prompter := &stubPermissionPrompter{decision: PermissionAllowOnce}
	a := mustNewAuthorizer(t, prompter, storagestub.NewInMemoryService(), texttest.NopEditor())
	ext := testPluginExtension(nil)
	ctxA := contextWithPeerProcess(context.Background(), peerprocess.Process{
		PID:  123,
		UID:  501,
		Exe:  "/usr/local/bin/plugin-cli",
		Argv: []string{"plugin-cli", "wm", "focus"},
	})
	ctxB := contextWithPeerProcess(context.Background(), peerprocess.Process{
		PID:  456,
		UID:  501,
		Exe:  "/usr/local/bin/plugin-cli",
		Argv: []string{"plugin-cli", "wm", "focus"},
	})

	require.NoError(t, a.Authorize(ctxA, blueauth.UserClaims[Extension]{Extra: ext},
		testWindowManagerResource))
	require.NoError(t, a.Authorize(ctxB, blueauth.UserClaims[Extension]{Extra: ext},
		testWindowManagerResource))
	assert.Equal(t, 2, prompter.calls)
}

func TestAuthorizerPluginOnceDecisionCacheRequiresPeerPID(t *testing.T) {
	t.Parallel()

	prompter := &stubPermissionPrompter{decision: PermissionAllowOnce}
	a := mustNewAuthorizer(t, prompter, storagestub.NewInMemoryService(), texttest.NopEditor())
	ext := testPluginExtension(nil)

	for range 2 {
		require.NoError(t, a.Authorize(context.Background(),
			blueauth.UserClaims[Extension]{Extra: ext}, testWindowManagerResource))
	}
	assert.Equal(t, 2, prompter.calls)
}

func TestPluginPermissionOnceDecisionExpires(t *testing.T) {
	t.Parallel()

	a := newTestAuthorizerCore(nil, nil)
	now := time.Now()
	key := "once-key"
	a.setOnceDecision(key, pluginPermissionIdentity{Path: "/bin/test"},
		extensionapi.PermissionLSP, pluginPermissionDecisionAllow,
		now.Add(-pluginPermissionOnceTTL-time.Second))

	_, ok := a.getOnceDecision(key, now)
	assert.False(t, ok)
}

func TestPluginPermissionOnceKeyIncludesProcessIdentity(t *testing.T) {
	t.Parallel()

	base := pluginPermissionIdentity{
		PID:  123,
		UID:  501,
		Path: "/usr/local/bin/plugin-cli",
		Args: []string{"wm", "focus"},
	}
	baseKey := pluginPermissionOnceKey(base, extensionapi.PermissionBrowserWindowManager)
	assert.NotEmpty(t, baseKey)

	changedPID := base
	changedPID.PID = 456
	assert.NotEqual(t, baseKey,
		pluginPermissionOnceKey(changedPID, extensionapi.PermissionBrowserWindowManager))

	changedUID := base
	changedUID.UID = 502
	assert.NotEqual(t, baseKey,
		pluginPermissionOnceKey(changedUID, extensionapi.PermissionBrowserWindowManager))

	changedPermission := base
	assert.NotEqual(t, baseKey,
		pluginPermissionOnceKey(changedPermission, extensionapi.PermissionStorage))
}

func TestAuthorizerPluginPersistedDecisionsSkipPrompt(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		stored     string
		wantErrIs  error
		wantPrompt int
	}{
		{name: "stored allow", stored: pluginPermissionDecisionAllow},
		{name: "stored deny", stored: pluginPermissionDecisionDeny,
			wantErrIs: blueauth.ErrForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			storage := storagestub.NewInMemoryService()
			ext := testPluginExtension(nil)
			key := pluginPermissionStorageKey(ext.Path, ext.Args,
				extensionapi.PermissionBrowserWindowManager)
			require.NoError(t, storage.Set(context.Background(), key,
				storedPermissionDecision{Decision: tc.stored}))
			prompter := &stubPermissionPrompter{
				decision: PermissionDenyOnce,
			}
			a := mustNewAuthorizer(t, prompter, storage, texttest.NopEditor())

			err := a.Authorize(context.Background(), blueauth.UserClaims[Extension]{
				Extra: ext,
			}, testWindowManagerResource)
			if tc.wantErrIs != nil {
				assert.ErrorIs(t, err, tc.wantErrIs)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.wantPrompt, prompter.calls)
		})
	}
}

func TestAuthorizerPluginStoredUnknownDecisionReturnsError(t *testing.T) {
	t.Parallel()

	storage := storagestub.NewInMemoryService()
	ext := testPluginExtension(nil)
	key := pluginPermissionStorageKey(ext.Path, ext.Args,
		extensionapi.PermissionBrowserWindowManager)
	require.NoError(t, storage.Set(context.Background(), key,
		storedPermissionDecision{Decision: "maybe"}))
	prompter := &stubPermissionPrompter{decision: PermissionAllowOnce}
	a := mustNewAuthorizer(t, prompter, storage, texttest.NopEditor())

	err := a.Authorize(context.Background(), blueauth.UserClaims[Extension]{Extra: ext},
		testWindowManagerResource)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown stored plugin permission decision")
	assert.Zero(t, prompter.calls)
}

func TestAuthorizerPluginStorageGetErrorReturnsError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("get failed")
	prompter := &stubPermissionPrompter{decision: PermissionAllowOnce}
	a := mustNewAuthorizer(t, prompter, errorStorage{
		Service: storagestub.NewInMemoryService(),
		getErr:  wantErr,
	}, texttest.NopEditor())

	err := a.Authorize(context.Background(), blueauth.UserClaims[Extension]{
		Extra: testPluginExtension(nil),
	}, testWindowManagerResource)
	assert.ErrorIs(t, err, wantErr)
	assert.Zero(t, prompter.calls)
}

func TestAuthorizerPluginPersistsAlwaysDecisions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		decision   PermissionDecision
		wantStored string
		wantErrIs  error
	}{
		{name: "allow always", decision: PermissionAllowAlways,
			wantStored: pluginPermissionDecisionAllow},
		{name: "deny always", decision: PermissionDenyAlways,
			wantStored: pluginPermissionDecisionDeny, wantErrIs: blueauth.ErrForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			storage := storagestub.NewInMemoryService()
			ext := testPluginExtension(nil)
			a := mustNewAuthorizer(t, &stubPermissionPrompter{decision: tc.decision}, storage, texttest.NopEditor())

			err := a.Authorize(context.Background(), blueauth.UserClaims[Extension]{Extra: ext},
				testWindowManagerResource)
			if tc.wantErrIs != nil {
				assert.ErrorIs(t, err, tc.wantErrIs)
			} else {
				assert.NoError(t, err)
			}

			key := pluginPermissionStorageKey(ext.Path, ext.Args,
				extensionapi.PermissionBrowserWindowManager)
			var stored storedPermissionDecision
			require.NoError(t, storage.Get(context.Background(), key, &stored))
			assert.Equal(t, tc.wantStored, stored.Decision)
		})
	}
}

func TestAuthorizerPluginPersistSetErrorReturnsError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("set failed")
	cases := []PermissionDecision{
		PermissionAllowAlways,
		PermissionDenyAlways,
	}
	for _, decision := range cases {
		t.Run(string(decision), func(t *testing.T) {
			t.Parallel()

			a := mustNewAuthorizer(t, &stubPermissionPrompter{decision: decision},
				errorStorage{
					Service: storagestub.NewInMemoryService(),
					setErr:  wantErr,
				}, texttest.NopEditor())
			err := a.Authorize(context.Background(), blueauth.UserClaims[Extension]{
				Extra: testPluginExtension(nil),
			}, testWindowManagerResource)
			assert.ErrorIs(t, err, wantErr)
		})
	}
}

func TestAuthorizerPluginDoesNotPersistOnceDecisions(t *testing.T) {
	t.Parallel()

	cases := []PermissionDecision{
		PermissionAllowOnce,
		PermissionDenyOnce,
	}
	for _, decision := range cases {
		t.Run(string(decision), func(t *testing.T) {
			t.Parallel()

			storage := storagestub.NewInMemoryService()
			ext := testPluginExtension(nil)
			a := mustNewAuthorizer(t, &stubPermissionPrompter{decision: decision}, storage, texttest.NopEditor())

			_ = a.Authorize(context.Background(), blueauth.UserClaims[Extension]{Extra: ext},
				testWindowManagerResource)
			key := pluginPermissionStorageKey(ext.Path, ext.Args,
				extensionapi.PermissionBrowserWindowManager)
			var stored storedPermissionDecision
			err := storage.Get(context.Background(), key, &stored)
			assert.ErrorIs(t, err, storageapi.ErrNotFound)
		})
	}
}

func TestAuthorizerPluginWithoutStorage(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		decision  PermissionDecision
		wantError error
	}{
		{name: "allow always without storage", decision: PermissionAllowAlways},
		{name: "deny always without storage", decision: PermissionDenyAlways,
			wantError: blueauth.ErrForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			a := mustNewAuthorizer(t, &stubPermissionPrompter{decision: tc.decision}, nil, texttest.NopEditor())
			err := a.Authorize(context.Background(), blueauth.UserClaims[Extension]{
				Extra: testPluginExtension(nil),
			}, testWindowManagerResource)
			if tc.wantError != nil {
				assert.ErrorIs(t, err, tc.wantError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAuthorizerPluginWithoutPrompterForbidden(t *testing.T) {
	t.Parallel()

	a := mustNewAuthorizer(t, nil, nil, texttest.NopEditor())
	err := a.Authorize(context.Background(), blueauth.UserClaims[Extension]{
		Extra: testPluginExtension(extensionapi.AllPermissions()),
	}, testWindowManagerResource)
	assert.ErrorIs(t, err, blueauth.ErrForbidden)
}

func TestAuthorizerPluginUnknownPromptDecisionForbidden(t *testing.T) {
	t.Parallel()

	storage := storagestub.NewInMemoryService()
	ext := testPluginExtension(nil)
	a := mustNewAuthorizer(t, &stubPermissionPrompter{decision: "unknown"}, storage, texttest.NopEditor())

	err := a.Authorize(context.Background(), blueauth.UserClaims[Extension]{Extra: ext},
		testWindowManagerResource)
	assert.ErrorIs(t, err, blueauth.ErrForbidden)

	key := pluginPermissionStorageKey(ext.Path, ext.Args,
		extensionapi.PermissionBrowserWindowManager)
	var stored storedPermissionDecision
	err = storage.Get(context.Background(), key, &stored)
	assert.ErrorIs(t, err, storageapi.ErrNotFound)
}

func TestPluginPermissionStorageKeyChangesWhenArgsChange(t *testing.T) {
	t.Parallel()

	keyA := pluginPermissionStorageKey("/bin/test", []string{"a"},
		extensionapi.PermissionBrowserWindowManager)
	keyB := pluginPermissionStorageKey("/bin/test", []string{"b"},
		extensionapi.PermissionBrowserWindowManager)
	assert.NotEqual(t, keyA, keyB)
}

func TestPluginPermissionStorageKeyUsesColonSeparator(t *testing.T) {
	t.Parallel()

	key := pluginPermissionStorageKey("/usr/local/bin/test", []string{"a/b", "c"},
		extensionapi.PermissionBrowserWindowManager)
	assert.NotContains(t, key, "/")
	assert.Contains(t, key, ":")
	assert.Contains(t, key, "extensionv2:plugin-permissions:")
}

func TestPluginPermissionStorageKeyChangesWhenPermissionChanges(t *testing.T) {
	t.Parallel()

	keyA := pluginPermissionStorageKey("/bin/test", []string{"a"},
		extensionapi.PermissionBrowserWindowManager)
	keyB := pluginPermissionStorageKey("/bin/test", []string{"a"},
		extensionapi.PermissionStorage)
	assert.NotEqual(t, keyA, keyB)
}

func TestAuthorizerPluginPromptError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("prompt failed")
	a := mustNewAuthorizer(t, &stubPermissionPrompter{err: wantErr},
		storagestub.NewInMemoryService(), texttest.NopEditor())
	err := a.Authorize(context.Background(), blueauth.UserClaims[Extension]{
		Extra: testPluginExtension(nil),
	}, testWindowManagerResource)
	assert.ErrorIs(t, err, wantErr)
}

func testPluginExtension(perms extensionapi.Permissions) Extension {
	return Extension{
		Metadata: extensionapi.Metadata{Permissions: perms},
		Plugin:   true,
		Path:     "/bin/test",
		Args:     []string{"--flag"},
	}
}

func testRegularExtension(perms extensionapi.Permissions) Extension {
	return Extension{Metadata: extensionapi.Metadata{
		DeveloperID:      "dev-id",
		DeveloperEmail:   "dev@example.com",
		DeveloperKey:     "dev-key",
		ExtensionID:      "test-extension",
		ExtensionName:    "Test Extension",
		ExtensionVersion: "v1.2.3",
		Permissions:      perms,
	}}
}

func contextWithPeerProcess(ctx context.Context, process peerprocess.Process) context.Context {
	return peer.NewContext(ctx, &peer.Peer{Addr: peerProcessAddr{process: process}})
}

func mustNewAuthorizer(
	t *testing.T, prompter PermissionPrompter,
	storage storageapi.Service, editor text.Editor,
) blueauth.Authorizer[Extension] {
	t.Helper()
	a, err := newAuthorizer(prompter, storage, editor)
	require.NoError(t, err)
	return a
}

func newTestAuthorizerCore(
	prompter PermissionPrompter, storage storageapi.Service,
) *authorizer {
	return &authorizer{
		prompter: prompter,
		storage:  storage,
		once:     make(map[string]pluginPermissionOnceDecision),
	}
}

var _ PermissionPrompter = (*stubPermissionPrompter)(nil)
