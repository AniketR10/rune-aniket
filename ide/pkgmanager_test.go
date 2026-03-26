// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/blue/release"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/handler/handlertest"
	"unstable.build/go-tui/ide/idepkg/idepkgtest"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/workspace"
)

func TestPackageManagerConcurrent(t *testing.T) {
	t.Parallel()
	pkgs := idepkgtest.MakePackages(
		release.Package{Name: "go"},
		release.Package{Name: "six", Latest: "2"},
	)
	bundles := idepkgtest.MakeBundles(
		[]release.Bundle{
			{Package: "go", Version: "2"},
			{Package: "go", Version: "3"},
			{Package: "go", Version: "1", CreatedAt: time.Now()},
		},
		[]release.Bundle{
			{Package: "six", Version: "1"},
			{Package: "six", Version: "2"},
		},
	)
	rm := idepkgtest.NewReleaseManager(pkgs, bundles)
	rm.SetMissProgressComplete(true)
	cfg := defaultCfg()
	var m *testWorkspaceManagerHandler
	var mu sync.Mutex
	cfg.scheduleNextTick = func(fn func()) bool {
		mu.Lock()
		defer mu.Unlock()
		fn()
		return true
	}
	m = newTestWorkspaceManagerHandlerForPkgManagerWithInterrupterCfg(t, rm, true,
		term.NopInterrupter(), cfg, &mu)
	require.NoError(t, m.pkgmanager.setAutoInstall())

	const n = 1000
	var errs [n]error
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			it, err := m.pkgmanager.LibDir(context.Background(), "go")
			if err != nil {
				errs[i] = err
				return
			}
			defer it.Close()
			for {
				_, ok := it.Next(context.Background())
				if !ok {
					break
				}
			}
			if err := it.Err(); err != nil {
				errs[i] = err
			}
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("LibDir %d returned error: %v", i, err)
		}
	}
}

func TestPackageManagerIntegration(t *testing.T) {
	t.Parallel()
	pkgs := idepkgtest.MakePackages(
		release.Package{Name: "go"},
		release.Package{Name: "six", Latest: "2"},
	)
	bundles := idepkgtest.MakeBundles(
		[]release.Bundle{
			{Package: "go", Version: "2"},
			{Package: "go", Version: "3"},
			{Package: "go", Version: "1", CreatedAt: time.Now()},
		},
		[]release.Bundle{
			{Package: "six", Version: "1"},
			{Package: "six", Version: "2"},
		},
	)
	rm := idepkgtest.NewReleaseManager(pkgs, bundles)
	rm.SetMissProgressComplete(true)
	m := newTestWorkspaceManagerHandlerForPkgManager(t, rm, true, 15)
	m.forceSyncCommandPrompt = true

	cases := []handlertest.SequenceTestCase{
		{":pkginstall ",
			`┌──────────────────────────────────────┐
│                                      │
├──────────────────────────────────────┤
│                                      │
│                                      │
│                                      │
┌──────────────────────────────────────┐
│pkginstall ▐                          │
│go                                    │
│six                                   │
│──────────────────────────────────────│
│                                      │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
		{"six ",
			`┌──────────────────────────────────────┐
│                                      │
├──────────────────────────────────────┤
│                                      │
│                                      │
│                                      │
┌──────────────────────────────────────┐
│pkginstall six ▐                      │
│1                                     │
│2                                     │
│──────────────────────────────────────│
│                                      │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
		{"1>",
			`┌────────────────────────┌─────────────┐
│                        │ downloading │
├────────────────────────│  version 1  │
│                        │ of package  │
│                        │ six         │
│                        └─────────────┘
│                                      │
│          workspaceWallpaper          │
│                                      │
│                                      │
│                                      │
│                                      │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
		{":noticlose>:pkgwait six>:pkgcurrent ",
			`┌────────────────────────┌─────────────┐
│                        │ downloaded  │
├────────────────────────│ version 1   │
│                        │ of package  │
│                        │ six         │
│                        └─────────────┘
┌──────────────────────────────────────┐
│pkgcurrent ▐                          │
│six                                   │
│                                      │
│──────────────────────────────────────│
│                                      │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
		{"six>",
			`┌────────────────────────┌─────────────┐
│                        │ version 1   │
├────────────────────────│ of package  │
│                        │ six is in   │
│                        │ use         │
│                        └─────────────┘
│                        ┌─────────────┐
│          workspaceWallp│ downloaded  │
│                        │ version 1   │
│                        │ of package  │
│                        │ six         │
│                        └─────────────┘
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
		{":noticlose>:pkginstall six 1>",
			`┌────────────────────────┌─────────────┐
│                        │ version 1   │
├────────────────────────│ of package  │
│                        │ six has     │
│                        │ already     │
│                        │ been        │
│                        │ installed   │
│          workspaceWallp└─────────────┘
│                                      │
│                                      │
│                                      │
│                                      │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
		{":noticlose>:pkginstall go>:pkgwait go>",
			`┌────────────────────────┌─────────────┐
│                        │ downloaded  │
├────────────────────────│ version 1   │
│                        │ of package  │
│                        │ go          │
│                        └─────────────┘
│                                      │
│          workspaceWallpaper          │
│                                      │
│                                      │
│                                      │
│                                      │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
		{":noticlose>:pkgupdateall>:pkgwait six>",
			`┌────────────────────────┌─────────────┐
│                        │ downloaded  │
├────────────────────────│ version 2   │
│                        │ of package  │
│                        │ six         │
│                        └─────────────┘
│                        ┌─────────────┐
│          workspaceWallp│ package go  │
│                        │ already     │
│                        │ updated to  │
│                        │ the latest  │
│                        │ version (1) │
├────────────────────────└─────────────┤
│1                                     │
└──────────────────────────────────────┘`},
		{":noticlose>:pkguse six 1>",
			`┌────────────────────────┌─────────────┐
│                        │ version 1   │
├────────────────────────│ of package  │
│                        │ six is now  │
│                        │ in use      │
│                        └─────────────┘
│                                      │
│          workspaceWallpaper          │
│                                      │
│                                      │
│                                      │
│                                      │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
		{":noticlose>:pkginstall six>",
			`┌────────────────────────┌─────────────┐
│                        │ version 2   │
├────────────────────────│ of package  │
│                        │ six has     │
│                        │ already     │
│                        │ been        │
│                        │ installed   │
│          workspaceWallp└─────────────┘
│                                      │
│                                      │
│                                      │
│                                      │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
		{":noticlose>:pkginstall go>",
			`┌────────────────────────┌─────────────┐
│                        │ version 1   │
├────────────────────────│ of package  │
│                        │ go has      │
│                        │ already     │
│                        │ been        │
│                        │ installed   │
│          workspaceWallp└─────────────┘
│                                      │
│                                      │
│                                      │
│                                      │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
		{":noticlose>:pkgremove six 1>",
			`┌────────────────────────┌─────────────┐
│                        │ version 1   │
├────────────────────────│ of package  │
│                        │ six is      │
│                        │ currently   │
│                        │ in use,     │
│                        │ run         │
│          workspaceWallp│ 'pkguse'    │
│                        │ with some   │
│                        │ other       │
│                        │ version     │
│                        │ first       │
├────────────────────────│ before      ┤
│1                                     │
└──────────────────────────────────────┘`},
		{":noticlose>:pkgremove six 2>",
			`┌────────────────────────┌─────────────┐
│                        │ version 2   │
├────────────────────────│ of package  │
│                        │ six has     │
│                        │ been        │
│                        │ removed     │
│                        └─────────────┘
│          workspaceWallpaper          │
│                                      │
│                                      │
│                                      │
│                                      │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
		{":noticlose>:pkgremove go>",
			`┌────────────────────────┌─────────────┐
│                        │ package go  │
├────────────────────────│ has been    │
│                        │ removed     │
│                        └─────────────┘
│                                      │
│                                      │
│          workspaceWallpaper          │
│                                      │
│                                      │
│                                      │
│                                      │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
		{":noticlose>:pkginstall ox>",
			`┌────────────────────────┌─────────────┐
│                        │ install     │
├────────────────────────│ package:    │
│                        │ not found   │
│                        └─────────────┘
│                                      │
│                                      │
│          workspaceWallpaper          │
│                                      │
│                                      │
│                                      │
│                                      │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
		{":noticlose>:pkgremove ox>",
			`┌────────────────────────┌─────────────┐
│                        │ package ox  │
├────────────────────────│ is not      │
│                        │ installed   │
│                        └─────────────┘
│                                      │
│                                      │
│          workspaceWallpaper          │
│                                      │
│                                      │
│                                      │
│                                      │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
		{":noticlose>:pkguse six 2>",
			`┌────────────────────────┌─────────────┐
│                        │ package     │
├────────────────────────│ version is  │
│                        │ not         │
│                        │ installed   │
│                        └─────────────┘
│                                      │
│          workspaceWallpaper          │
│                                      │
│                                      │
│                                      │
│                                      │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
		{":noticlose>:pkgremove six>",
			`┌────────────────────────┌─────────────┐
│                        │ package     │
├────────────────────────│ six has     │
│                        │ been        │
│                        │ removed     │
│                        └─────────────┘
│                                      │
│          workspaceWallpaper          │
│                                      │
│                                      │
│                                      │
│                                      │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
		{":noticlose>:pkguse six 1>",
			`┌────────────────────────┌─────────────┐
│                        │ package     │
├────────────────────────│ version is  │
│                        │ not         │
│                        │ installed   │
│                        └─────────────┘
│                                      │
│          workspaceWallpaper          │
│                                      │
│                                      │
│                                      │
│                                      │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
	}
	h := newSafeHandler(m)

	handlertest.TestHandlerSequence(t, h, 40, 15, cases)

	require.NoError(t, m.Close())
}

func TestPackageManagerPreviewIntegration(t *testing.T) {
	createdAt := time.Unix(6666666666, 0)
	t.Parallel()
	pkgs := idepkgtest.MakePackages(
		release.Package{Name: "go", CreatedAt: createdAt},
		release.Package{
			Name:      "six",
			Latest:    "2",
			Notes:     "blabla",
			Metadata:  map[string]string{},
			CreatedAt: createdAt,
		},
	)
	bundles := idepkgtest.MakeBundles(
		[]release.Bundle{
			{Package: "go", Version: "2"},
		},
		[]release.Bundle{
			{
				Package: "six",
				Version: "2",
				Notes:   "yikes",
				Metadata: map[string]string{
					"git-author-email": "clawdbot@clawd.bot",
					"git-log":          "Just messed up with the code a bit, you know\nthen something else\ndone",
				},
				CreatedAt: createdAt,
			},
		},
	)
	// used to ensure that animation is deterministically rendered:
	// Interrupt is blocked until wg.Wait returns, so after we have verified
	// Drawing animation.
	var pkgsema, interruptsema sync.Mutex
	rm := idepkgtest.NewReleaseManager(pkgs, bundles)
	rm.SetMissProgressComplete(true)

	rm.SetHook(func() {
		pkgsema.Lock()
		defer pkgsema.Unlock()
	})

	i := term.FuncInterrupter(func(ctx context.Context) error {
		if component.IsAsyncContext(ctx) {
			interruptsema.Unlock()
		}
		return nil
	})

	m := newTestWorkspaceManagerHandlerForPkgManagerWithInterrupter(t, rm, true, i)
	h := newSafeHandler(m)

	cases := []handlertest.SequenceTestCase{
		{"<c-\\\\>pkginstall<space>",
			`┌──────────────────────────────────────┐
│                                      │
├──────────────────────────────────────┤
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
┌──────────────────────────────────────┐
│pkginstall ▐                          │
│go                                    │
│six                                   │
│──────────────────────────────────────│
│                                      │
│ Usage                                │
│                                      │
│ pkginstall <package> [version]       │
│                                      │
│                                      │
│ Description                          │
│                                      │
│ Installs a package from the official │
│ distribution. If version is omitted, │
└──────────────────────────────────────┘
│                                      │
│                                      │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
		{"<down>",
			`┌──────────────────────────────────────┐
│                                      │
├──────────────────────────────────────┤
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
┌──────────────────────────────────────┐
│pkginstall ▐                          │
│go                                    │
│six                                   │
│──────────────────────────────────────│
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
│                  ⠃                   │
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
	}

	pkgsema.Lock()
	handlertest.RunHandlerSequence(t, h, 40, 30, cases)

	cases = []handlertest.SequenceTestCase{
		{"",
			`┌──────────────────────────────────────┐
│                                      │
├──────────────────────────────────────┤
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
┌──────────────────────────────────────┐
│pkginstall ▐                          │
│go                                    │
│six                                   │
│──────────────────────────────────────│
│go                                    │
│                                      │
│NOTES                                 │
│                                      │
│                                      │
│VERSION                               │
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
│CREATED AT                            │
│Apr 4, 2181 1:51 PM                   │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
	}

	interruptsema.Lock()
	pkgsema.Unlock()
	interruptsema.Lock()
	handlertest.RunHandlerSequence(t, h, 40, 30, cases)
	interruptsema.Unlock()

	cases = []handlertest.SequenceTestCase{
		{"<down>",
			`┌──────────────────────────────────────┐
│                                      │
├──────────────────────────────────────┤
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
┌──────────────────────────────────────┐
│pkginstall ▐                          │
│go                                    │
│six                                   │
│──────────────────────────────────────│
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
│                  ⠃                   │
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
	}
	pkgsema.Lock()
	handlertest.RunHandlerSequence(t, h, 40, 30, cases)

	cases = []handlertest.SequenceTestCase{
		{"",
			`┌──────────────────────────────────────┐
│                                      │
├──────────────────────────────────────┤
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
┌──────────────────────────────────────┐
│pkginstall ▐                          │
│go                                    │
│six                                   │
│──────────────────────────────────────│
│six                                   │
│                                      │
│NOTES                                 │
│blabla                                │
│                                      │
│VERSION                               │
│2                                     │
│                                      │
│                                      │
│                                      │
│                                      │
│CREATED AT                            │
│Apr 4, 2181 1:51 PM                   │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
	}
	interruptsema.Lock()
	pkgsema.Unlock()
	interruptsema.Lock()
	handlertest.RunHandlerSequence(t, h, 40, 30, cases)
	interruptsema.Unlock()

	cases = []handlertest.SequenceTestCase{
		{"<tab><down>",
			`┌──────────────────────────────────────┐
│                                      │
├──────────────────────────────────────┤
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
┌──────────────────────────────────────┐
│pkginstall six ▐                      │
│2                                     │
│                                      │
│──────────────────────────────────────│
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
│                  ⠃                   │
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
	}
	pkgsema.Lock()
	handlertest.RunHandlerSequence(t, h, 40, 30, cases)

	cases = []handlertest.SequenceTestCase{
		{"",
			`┌──────────────────────────────────────┐
│                                      │
├──────────────────────────────────────┤
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
│                                      │
┌──────────────────────────────────────┐
│pkginstall six ▐                      │
│2                                     │
│                                      │
│──────────────────────────────────────│
│six @ 2                               │
│                                      │
│CHANGE LOG                            │
│Just messed up with the code a bit,   │
│you know                              │
│then something else                   │
│done                                  │
│                                      │
│AUTHOR                                │
│clawdbot@clawd.bot                    │
│                                      │
│CREATED AT                            │
│Apr 4, 2181 1:51 PM                   │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
	}
	interruptsema.Lock()
	pkgsema.Unlock()
	interruptsema.Lock()
	handlertest.RunHandlerSequence(t, h, 40, 30, cases)
	interruptsema.Unlock()

	require.NoError(t, m.Close())
}

func TestPackageManagerLibDir(t *testing.T) {
	t.Parallel()
	pkgs := idepkgtest.MakePackages(
		release.Package{Name: "go", Latest: "3"},
		release.Package{Name: "six", Latest: "2"},
	)
	bundles := idepkgtest.MakeBundles(
		[]release.Bundle{
			{Package: "go", Version: "1"},
			{Package: "go", Version: "2"},
			{Package: "go", Version: "3"},
		},
		[]release.Bundle{
			{Package: "six", Version: "1"},
			{Package: "six", Version: "2"},
		},
	)
	t.Run("prompt, no install", func(t *testing.T) {
		t.Parallel()

		rm := idepkgtest.NewReleaseManager(pkgs, bundles)
		m := newTestWorkspaceManagerHandlerForPkgManager(t, rm, false, 0)

		it, err := m.pkgmanager.LibDir(context.Background(), "go")
		require.NoError(t, err)

		cases := []handlertest.SequenceTestCase{
			{"",
				`┌──────────────────────────────────────┐
│                                      │
├──────────────────────────────────────┤
┌──────────────────────────────────────┐
│                                      │
│  Do you want to install package      │
│  "go"?                               │
│                                      │
│                                      │
│                                      │
│     Yes, Always          No          │
└──────────────────────────────────────┘
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
			{"N",
				`┌──────────────────────────────────────┐
│                                      │
├──────────────────────────────────────┤
│                                      │
│                                      │
│                                      │
│                                      │
│          workspaceWallpaper          │
│                                      │
│                                      │
│                                      │
│                                      │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
		}

		handlertest.TestHandlerSequence(t, m, 40, 15, cases)

		_, err = iterator.ToSlice(context.Background(), it)
		require.Equal(t, document.ErrNotFound, err)

		require.NoError(t, m.Close())
	})
	t.Run("prompt, user key ESC", func(t *testing.T) {
		t.Parallel()

		rm := idepkgtest.NewReleaseManager(pkgs, bundles)
		m := newTestWorkspaceManagerHandlerForPkgManager(t, rm, false, 0)

		it, err := m.pkgmanager.LibDir(context.Background(), "go")
		require.NoError(t, err)

		cases := []handlertest.SequenceTestCase{
			{"<",
				`┌──────────────────────────────────────┐
│                                      │
├──────────────────────────────────────┤
│                                      │
│                                      │
│                                      │
│                                      │
│          workspaceWallpaper          │
│                                      │
│                                      │
│                                      │
│                                      │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
		}

		handlertest.TestHandlerSequence(t, m, 40, 15, cases)

		_, err = iterator.ToSlice(context.Background(), it)
		require.Equal(t, document.ErrNotFound, err)

		require.NoError(t, m.Close())
	})

	t.Run("prompt, yes install", func(t *testing.T) {
		t.Parallel()

		rm := idepkgtest.NewReleaseManager(pkgs, bundles)
		m := newTestWorkspaceManagerHandlerForPkgManager(t, rm, false, 0)

		it, err := m.pkgmanager.LibDir(context.Background(), "go")
		require.NoError(t, err)

		cases := []handlertest.SequenceTestCase{
			{"Y",
				`┌──────────────────────────────────────┐
│                                      │
├──────────────────────────────────────┤
│                                      │
│                                      │
│                                      │
│                                      │
│          workspaceWallpaper          │
│                                      │
│                                      │
│                                      │
│                                      │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
		}

		handlertest.TestHandlerSequence(t, m, 40, 15, cases)

		slice, err := iterator.ToSlice(context.Background(), it)
		require.NoError(t, err)
		assert.NotEmpty(t, slice)
		require.NoError(t, m.Close())
	})

	t.Run("prompt, yes, always install", func(t *testing.T) {
		t.Parallel()

		rm := idepkgtest.NewReleaseManager(pkgs, bundles)
		m := newTestWorkspaceManagerHandlerForPkgManager(t, rm, false, 0)

		it, err := m.pkgmanager.LibDir(context.Background(), "go")
		require.NoError(t, err)

		cases := []handlertest.SequenceTestCase{
			{"A",
				`┌──────────────────────────────────────┐
│                                      │
├──────────────────────────────────────┤
│                                      │
│                                      │
│                                      │
│                                      │
│          workspaceWallpaper          │
│                                      │
│                                      │
│                                      │
│                                      │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
		}

		handlertest.TestHandlerSequence(t, m, 40, 15, cases)

		slice, err := iterator.ToSlice(context.Background(), it)
		require.NoError(t, err)
		assert.NotEmpty(t, slice)

		it, err = m.pkgmanager.LibDir(context.Background(), "six")
		require.NoError(t, err)

		slice, err = iterator.ToSlice(context.Background(), it)
		require.NoError(t, err)
		assert.NotEmpty(t, slice)

		require.NoError(t, m.Close())
	})

	t.Run("prompt, no never install", func(t *testing.T) {
		t.Parallel()

		rm := idepkgtest.NewReleaseManager(pkgs, bundles)
		m := newTestWorkspaceManagerHandlerForPkgManager(t, rm, false, 0)

		it, err := m.pkgmanager.LibDir(context.Background(), "go")
		require.NoError(t, err)

		cases := []handlertest.SequenceTestCase{
			{"V",
				`┌──────────────────────────────────────┐
│                                      │
├──────────────────────────────────────┤
│                                      │
│                                      │
│                                      │
│                                      │
│          workspaceWallpaper          │
│                                      │
│                                      │
│                                      │
│                                      │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
		}

		handlertest.TestHandlerSequence(t, m, 40, 15, cases)

		_, err = iterator.ToSlice(context.Background(), it)
		require.Equal(t, document.ErrNotFound, err)

		_, err = m.pkgmanager.LibDir(context.Background(), "six")
		require.Equal(t, document.ErrNotFound, err)

		require.NoError(t, m.Close())
	})

	t.Run("prompt, yes install, simultaneous calls to LibDir", func(t *testing.T) {
		t.Parallel()

		rm := idepkgtest.NewReleaseManager(pkgs, bundles)
		m := newTestWorkspaceManagerHandlerForPkgManager(t, rm, false, 0)

		it1, err := m.pkgmanager.LibDir(context.Background(), "go")
		require.NoError(t, err)

		it2, err := m.pkgmanager.LibDir(context.Background(), "go")
		require.NoError(t, err)

		it3, err := m.pkgmanager.LibDir(context.Background(), "go")
		require.NoError(t, err)

		cases := []handlertest.SequenceTestCase{
			{"Y",
				`┌──────────────────────────────────────┐
│                                      │
├──────────────────────────────────────┤
│                                      │
│                                      │
│                                      │
│                                      │
│          workspaceWallpaper          │
│                                      │
│                                      │
│                                      │
│                                      │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
		}

		handlertest.TestHandlerSequence(t, m, 40, 15, cases)

		for _, it := range []iterator.Iterator[string]{it1, it2, it3} {
			slice, err := iterator.ToSlice(context.Background(), it)
			require.NoError(t, err)
			assert.NotEmpty(t, slice)
			require.NoError(t, m.Close())
		}
	})
}

func TestSetReleaseManager(t *testing.T) {
	t.Parallel()
	rm := idepkgtest.NewReleaseManager(idepkgtest.MakePackages(), idepkgtest.MakeBundles())
	m := newTestWorkspaceManagerHandlerForPkgManager(t, rm, false, 0)

	cases := []handlertest.SequenceTestCase{
		{":pkginstall ",
			`┌──────────────────────────────────────┐
│                                      │
├──────────────────────────────────────┤
│                                      │
│                                      │
│                                      │
┌──────────────────────────────────────┐
│pkginstall ▐                          │
│                                      │
│                                      │
└──────────────────────────────────────┘
│                                      │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
	}
	h := newSafeHandler(m)
	handlertest.TestHandlerSequence(t, h, 40, 15, cases)

	pkgs := idepkgtest.MakePackages(
		release.Package{Name: "go"},
	)
	bundles := idepkgtest.MakeBundles(
		[]release.Bundle{
			{Package: "go", Version: "3"},
		},
	)
	rm2 := idepkgtest.NewReleaseManager(pkgs, bundles)
	rm2.SetMissProgressComplete(true)
	m.setReleaseManager(rm2)

	cases = []handlertest.SequenceTestCase{
		{"<:pkginstall ",
			`┌──────────────────────────────────────┐
│                                      │
├──────────────────────────────────────┤
│                                      │
│                                      │
│                                      │
┌──────────────────────────────────────┐
│pkginstall ▐                          │
│go                                    │
│                                      │
└──────────────────────────────────────┘
│                                      │
├──────────────────────────────────────┤
│1                                     │
└──────────────────────────────────────┘`},
	}
	handlertest.TestHandlerSequence(t, h, 40, 15, cases)

	require.NoError(t, m.Close())
}

func newTestWorkspaceManagerHandlerForPkgManager(
	t *testing.T, releaseManager release.Manager,
	showManual bool, notificationsWidth int,
) *testWorkspaceManagerHandler {
	cfg := defaultCfg()
	homeURI, err := workspaceapi.ParseURI("file:///tmp")
	require.NoError(t, err)

	m := new(testWorkspaceManagerHandler)
	m.workspaceManagerHandler = new(workspaceManagerHandler)

	// ensure that command manual is always shown
	if showManual {
		updatedCfg := defaultCfg().cfg["command"].(map[string]any)
		updatedCfg["show_manual_after"] = "0ms"
		cfg.cfg["command"] = updatedCfg
	}
	runner := FuncExtensionsRunner(testRunnerFn)
	shutdownShaderCfg := nopShutdownShaderConfig()
	mu := new(sync.Mutex)
	interrupter := term.NopInterrupter()
	shRunner := new(shaderRunner)
	shRunner.init(handler.Nop(), interrupter, term.Attributes{},
		shutdownShaderCfg, component.FrameCharSetDefault())
	if cfg.scheduleNextTick == nil {
		cfg.scheduleNextTick = func(fn func()) bool {
			mu.Lock()
			defer mu.Unlock()
			fn()
			return true
		}
	}

	dir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})
	manager := workspace.NewManager(cfg.workspace())
	manager.RegisterScheme(workspace.FileScheme, workspace.NewFileScheme)

	notiCfg := notificationsConfig()
	notiCfg.Width = notificationsWidth
	err = m.workspaceManagerHandler.init(nil, homeURI, manager,
		notiCfg, cfg,
		dir, func(ev term.Event) bool {
			if ev.Type == term.EventInterrupt {
				interrupter.Interrupt(ev.Context)
			}
			return true
		}, runner, mu, nil,
		func() (ideConfig, error) { return cfg, nil },
		".sixrc", 0, 0, '1', 0, 0, true, nil, releaseManager,
		shRunner, 0, nil)
	require.NoError(t, err)
	m.subscribeCommand(textapi.CommandManual{Name: "pkgwait"}, text.FuncCommandHandler(
		func(ctx context.Context, cmd textapi.Command) error {
			require.Len(t, cmd.Args, 1)
			it, err := m.pkgmanager.pkg.LibDir(ctx, cmd.Args[0])
			require.NoError(t, err)
			it.Next(ctx)
			it.Close()
			return nil
		}, nil))
	return m
}

func newTestWorkspaceManagerHandlerForPkgManagerWithInterrupter(
	t *testing.T, releaseManager release.Manager, showManual bool,
	interrupter term.Interrupter,
) *testWorkspaceManagerHandler {
	cfg := defaultCfg()
	return newTestWorkspaceManagerHandlerForPkgManagerWithInterrupterCfg(
		t, releaseManager, showManual, interrupter, cfg, new(sync.Mutex),
	)
}
func newTestWorkspaceManagerHandlerForPkgManagerWithInterrupterCfg(
	t *testing.T, releaseManager release.Manager, showManual bool,
	interrupter term.Interrupter,
	cfg ideConfig, mu sync.Locker,
) *testWorkspaceManagerHandler {
	dir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})
	manager := workspace.NewManager(cfg.workspace())
	manager.RegisterScheme(workspace.FileScheme, workspace.NewFileScheme)
	ret := newTestWorkspaceManagerHandlerWithReleaseManager(t, manager,
		cfg, FuncExtensionsRunner(testRunnerFn), nil, nil, dir, nil,
		nopShutdownShaderConfig(), releaseManager, showManual, interrupter, mu)
	// only home workspace has a sync command prompt
	ret.empty.syncCommandPrompt = true
	// this allows blocking until packages are installed, for testing
	ret.subscribeCommand(textapi.CommandManual{Name: "pkgwait"}, text.FuncCommandHandler(
		func(ctx context.Context, cmd textapi.Command) error {
			require.Len(t, cmd.Args, 1)
			it, err := ret.pkgmanager.pkg.LibDir(ctx, cmd.Args[0])
			require.NoError(t, err)
			it.Next(ctx)
			it.Close()
			return nil
		}, nil))
	return ret
}

func newTestWorkspaceManagerHandlerWithReleaseManager(
	t *testing.T, manager *workspace.Manager,
	cfg ideConfig, runner ExtensionsRunner,
	extensions map[string]Extension, files []string, dir string,
	onTabsClick func(int) bool,
	shutdownShaderCfg shutdownShaderConfig,
	releaseManager release.Manager,
	showManual bool,
	interrupter term.Interrupter,
	mu sync.Locker,
) *testWorkspaceManagerHandler {
	homeURI, err := workspaceapi.ParseURI("file:///tmp")
	require.NoError(t, err)

	m := new(testWorkspaceManagerHandler)
	m.workspaceManagerHandler = new(workspaceManagerHandler)
	// ensure that command manual is always shown
	updatedCfg := defaultCfg().cfg["command"].(map[string]any)
	if showManual {
		updatedCfg["show_manual_after"] = "0ms"
	}
	cfg.cfg["command"] = updatedCfg

	shRunner := new(shaderRunner)
	shRunner.init(handler.Nop(), interrupter, term.Attributes{},
		shutdownShaderCfg, component.FrameCharSetDefault())
	if cfg.scheduleNextTick == nil {
		cfg.scheduleNextTick = func(fn func()) bool {
			mu.Lock()
			defer mu.Unlock()
			fn()
			return true
		}
	}

	err = m.workspaceManagerHandler.init(nil, homeURI, manager,
		notificationsConfig(), cfg,
		dir, func(ev term.Event) bool {
			if ev.Type == term.EventInterrupt {
				interrupter.Interrupt(ev.Context)
			}
			return true
		}, runner, mu, extensions,
		func() (ideConfig, error) { return cfg, nil },
		".sixrc", 0, 0, '1', 0, 0, true, onTabsClick, releaseManager,
		shRunner, 0, nil)
	require.NoError(t, err)
	for i, file := range files {
		require.NoError(t, m.openFile(file, i == 0))
	}
	return m
}
