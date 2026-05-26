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

package ideauthorizer

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	blueauth "github.com/unstablebuild/blue/auth"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/ide/ideplan"
	"unstable.build/go-tui/text/texttest"
)

type stubPlanSource struct {
	decision ideplan.Decision
}

func (s stubPlanSource) PlanDecision(context.Context) ideplan.Decision { return s.decision }

func newAuthorizerWithPlan(
	t *testing.T, opener PromptOpener, plan ideplan.Source, noti browserapi.Notifications,
) *Authorizer {
	t.Helper()
	a, err := NewAuthorizer(
		texttest.NopEditor(), opener, storagestub.NewInMemoryService(),
		syncScheduleNextTick, noti, plan,
	)
	require.NoError(t, err)
	return a
}

func TestAuthorizerPlanDeniedBlocksExtension(t *testing.T) {
	t.Parallel()

	opener := &stubPromptOpener{decision: PermissionAllowOnce}
	plan := stubPlanSource{decision: ideplan.Decision{Kind: ideplan.Denied, Reason: "logged out"}}
	a := newAuthorizerWithPlan(t, opener, plan, nil)
	ext := testRegularExtension(extensionapi.NewPermissions(
		extensionapi.PermissionBrowserWindowManager,
	))

	err := a.Authorize(context.Background(),
		blueauth.UserClaims[Extension]{Extra: ext}, testWindowManagerResource)
	assert.ErrorIs(t, err, ideplan.ErrSubscriptionRequired)
	assert.ErrorIs(t, err, blueauth.ErrForbidden)
	assert.Zero(t, opener.calls)
}

func TestAuthorizerPlanDeniedBlocksPlugin(t *testing.T) {
	t.Parallel()

	opener := &stubPromptOpener{decision: PermissionAllowOnce}
	plan := stubPlanSource{decision: ideplan.Decision{Kind: ideplan.Denied, Reason: "not subscribed"}}
	a := newAuthorizerWithPlan(t, opener, plan, nil)
	ext := testPluginExtension(extensionapi.NewPermissions(extensionapi.PermissionStorage))

	err := a.Authorize(context.Background(),
		blueauth.UserClaims[Extension]{Extra: ext}, testWindowManagerResource)
	assert.ErrorIs(t, err, ideplan.ErrSubscriptionRequired)
	assert.Zero(t, opener.calls)
}

func TestAuthorizerPlanDeniedBlocksAuthorizeCommand(t *testing.T) {
	t.Parallel()

	opener := &stubPromptOpener{decision: PermissionAllowOnce}
	plan := stubPlanSource{decision: ideplan.Decision{Kind: ideplan.Denied, Reason: "logged out"}}
	a := newAuthorizerWithPlan(t, opener, plan, nil)
	ext := testRegularExtension(extensionapi.NewPermissions(extensionapi.PermissionExecute))

	ctx := blueauth.ContextWithClaims(context.Background(),
		blueauth.UserClaims[Extension]{Extra: ext})
	err := a.AuthorizeCommand(ctx, workspaceapi.Cmd{Path: "/bin/echo"})
	assert.ErrorIs(t, err, ideplan.ErrSubscriptionRequired)
}

func TestAuthorizerPlanAllowedDelegatesToPrompt(t *testing.T) {
	t.Parallel()

	opener := &stubPromptOpener{decision: PermissionAllowOnce}
	plan := stubPlanSource{decision: ideplan.Decision{Kind: ideplan.Allowed}}
	a := newAuthorizerWithPlan(t, opener, plan, nil)
	ext := testRegularExtension(extensionapi.NewPermissions(
		extensionapi.PermissionBrowserWindowManager,
	))

	err := a.Authorize(context.Background(),
		blueauth.UserClaims[Extension]{Extra: ext}, testWindowManagerResource)
	require.NoError(t, err)
	require.Equal(t, 1, opener.calls)
}

func TestAuthorizerPlanGracePeriodAllowsAndWarnsOnce(t *testing.T) {
	t.Parallel()

	opener := &stubPromptOpener{decision: PermissionAllowOnce}
	noti := &capturingNotifications{}
	plan := stubPlanSource{decision: ideplan.Decision{
		Kind:           ideplan.GracePeriod,
		Reason:         "verification overdue",
		GraceExpiresAt: time.Now().Add(48 * time.Hour),
	}}
	a := newAuthorizerWithPlan(t, opener, plan, noti)
	ext := testRegularExtension(extensionapi.NewPermissions(
		extensionapi.PermissionBrowserWindowManager,
	))

	for range 3 {
		err := a.Authorize(context.Background(),
			blueauth.UserClaims[Extension]{Extra: ext}, testWindowManagerResource)
		require.NoError(t, err)
	}
	captured := noti.captured()
	var graceWarnings int
	for _, c := range captured {
		if c.Level == browserapi.LevelWarn &&
			strings.Contains(c.Msg, "subscription verification is overdue") {
			graceWarnings++
		}
	}
	assert.Equal(t, 1, graceWarnings, "expected exactly one grace warning, got: %+v", captured)
}
