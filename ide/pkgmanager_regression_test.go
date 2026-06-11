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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/blue/release"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/ide/idepkg/idepkgtest"
)

func TestPackageManagerLibDirMissingPackageReturnsStorageNotFound(t *testing.T) {
	t.Parallel()

	rm := idepkgtest.NewReleaseManager(
		idepkgtest.MakePackages(release.Package{Name: "six", Latest: "1"}),
		idepkgtest.MakeBundles([]release.Bundle{{Package: "six", Version: "1"}}),
	)
	m := newTestWorkspaceManagerHandlerForPkgManager(t, rm, false, 0)
	defer func() {
		require.NoError(t, m.Close())
	}()

	it, err := m.pkgmanager.LibDir(context.Background(), "go")
	require.Nil(t, it)
	require.ErrorIs(t, err, storageapi.ErrNotFound)
}

// TestPackageManagerInstallPromptDoesNotBlockEventLoop is a regression
// test: selecting "Yes" on the auto-install prompt used to run the
// synchronous InstallPackageVersion download directly inside the prompt
// handler, which executes on the event loop and froze the UI until the
// package finished installing. The install must run off the event loop
// so dispatching the selection key returns promptly while the download
// is still in flight.
func TestPackageManagerInstallPromptDoesNotBlockEventLoop(t *testing.T) {
	t.Parallel()

	rm := idepkgtest.NewReleaseManager(
		idepkgtest.MakePackages(release.Package{Name: "go", Latest: "1"}),
		idepkgtest.MakeBundles([]release.Bundle{{Package: "go", Version: "1"}}),
	)
	m := newTestWorkspaceManagerHandlerForPkgManager(t, rm, false, 0)

	it, err := m.pkgmanager.LibDir(context.Background(), "go")
	require.NoError(t, err)

	m.Resize(40, 15)

	// Block the download so it cannot complete while the prompt
	// selection is dispatched. The hook is installed only after
	// LibDir's latest-version lookup, so it gates the install download
	// rather than the version lookup.
	releaseDownload := make(chan struct{})
	rm.SetHook(func() { <-releaseDownload })

	// Dispatching the "Yes" selection must return even though the
	// download is blocked; otherwise the install ran on the event loop.
	dispatched := make(chan struct{})
	go func() {
		m.Handle(term.Event{Ch: 'Y', Type: term.EventKey})
		close(dispatched)
	}()

	select {
	case <-dispatched:
	case <-time.After(5 * time.Second):
		close(releaseDownload)
		t.Fatal("dispatching the install prompt selection blocked on the install download")
	}

	close(releaseDownload)
	slice, err := iterator.ToSlice(context.Background(), it)
	require.NoError(t, err)
	require.NotEmpty(t, slice)

	require.NoError(t, m.Close())
}
