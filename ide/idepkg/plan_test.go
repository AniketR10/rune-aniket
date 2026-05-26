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

package idepkg

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/release"
	"github.com/unstablebuild/blue/release/cdnrelease"
	"unstable.build/go-tui/ide/idepkg/idepkgtest"
	"unstable.build/go-tui/ide/ideplan"
)

type stubPlan struct {
	d ideplan.Decision
}

func (s stubPlan) PlanDecision(context.Context) ideplan.Decision { return s.d }

func TestInstallPackageVersionPlanDeniedSubscriptionRequired(t *testing.T) {
	t.Parallel()

	pkgs := idepkgtest.MakePackages()
	versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "go", Version: "1"}})
	m, _, _, _ := newTestManager(t, pkgs, versions)
	m.planSource = stubPlan{d: ideplan.Decision{Kind: ideplan.Denied}}

	err := m.InstallPackageVersion(context.Background(), "go", "1")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSubscriptionRequired)
	assert.ErrorIs(t, err, ErrForbidden)
}

func TestInstallPackageVersionPlanAllowedNotBlocked(t *testing.T) {
	t.Parallel()

	pkgs := idepkgtest.MakePackages()
	versions := idepkgtest.MakeBundles([]release.Bundle{{Package: "go", Version: "1"}})
	m, n, _, _ := newTestManager(t, pkgs, versions)
	m.planSource = stubPlan{d: ideplan.Decision{Kind: ideplan.Allowed}}

	n.SetWg(1)
	err := m.InstallPackageVersion(context.Background(), "go", "1")
	require.NoError(t, err)
	n.Wait()
}

func TestForbiddenErrReleaseSurfacesSubscriptionRequired(t *testing.T) {
	se := &cdnrelease.StatusError{
		URL:    "https://example.test/api/releases/linux-amd64/foo",
		Status: http.StatusForbidden,
	}
	err := forbiddenErr(se)
	assert.ErrorIs(t, err, ErrSubscriptionRequired)
	assert.ErrorIs(t, err, ErrForbidden)
}

func TestForbiddenErrNonReleaseSurfacesPlainForbidden(t *testing.T) {
	se := &cdnrelease.StatusError{
		URL:    "https://example.test/api/other/foo",
		Status: http.StatusForbidden,
	}
	err := forbiddenErr(se)
	assert.ErrorIs(t, err, ErrForbidden)
	assert.False(t, errors.Is(err, ErrSubscriptionRequired),
		"non-release 403 should not be ErrSubscriptionRequired: %v", err)
}
