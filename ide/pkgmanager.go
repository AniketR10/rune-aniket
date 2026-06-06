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
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ernestrc/go-multierror"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/blue/release"
	"github.com/unstablebuild/ox-api/auth"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler"
	sdkiterator "github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/ide/idepkg"
	"unstable.build/go-tui/text"
)

const installStorageKey = "autoInstallPrompt"

type pkgManager struct {
	pkg              *idepkg.Manager
	n                browserapi.Notifications
	wh               *workspaceManagerHandler
	storage          storageapi.Service
	scheduleNextTick func(func()) bool
	interrupter      term.Interrupter
	pending          sync.Map // map[string]*sync.Mutex
	uc               *idepkg.UpdateChecker
}

type installStorageValue struct {
	Value bool // true => always, false => never
}

func (m *pkgManager) init(
	n browserapi.Notifications, rm release.Manager,
	wm browserapi.WindowManager,
	rootStorage storageapi.Service, scheme schemeapi.Scheme,
	dataDir, configPath string,
	fcs component.FrameCharSet,
	interrupter term.Interrupter, wh *workspaceManagerHandler,
	scheduleNextTick func(func()) bool,
	parser syntaxapi.Parser,
	editorMode string,
) {
	storage := storageapi.WithPartition(rootStorage, "idepkg")
	m.pkg = idepkg.NewManager(n, rm, storage, scheme, dataDir,
		configPath, wm, scheduleNextTick, interrupter,
		idepkg.WithFrameCharSet(fcs),
		idepkg.WithSyntaxParser(parser),
		idepkg.WithEditorMode(editorMode),
	)
	m.uc = idepkg.NewUpdateChecker(m.pkg)
	m.scheduleNextTick = scheduleNextTick
	m.n = n
	m.interrupter = interrupter
	m.wh = wh
	m.storage = storage
	m.uc.Start(context.Background())
}

// LibDir installs package via prompt if not installed yet
func (m *pkgManager) LibDir(ctx context.Context, pkgID string) (
	sdkiterator.Iterator[string], error,
) {
	it, err := m.pkg.LibDir(ctx, pkgID)
	if err == nil || !errors.Is(err, idepkg.ErrNotInstalled) {
		return it, err
	}

	version, err := m.getLatestVersion(ctx, pkgID)
	if err != nil {
		if errors.Is(err, auth.ErrNotAuthenticated) {
			_, _ = m.n.NotifyOnce(browserapi.LevelWarn,
				"Some language packages require authentication. "+
					"Run the `login` command to enable them.")
			return nil, storageapi.ErrNotFound
		}
		if errors.Is(err, storageapi.ErrNotFound) || errors.Is(err, document.ErrNotFound) {
			return nil, storageapi.ErrNotFound
		}
		return nil, fmt.Errorf("get latest version: %w", err)
	}

	ready, ok := m.pending.Load(pkgID)
	if ok {
		return newPendingIterator(m.pkg, pkgID, ready.(*sync.Mutex)), nil
	}

	var val installStorageValue
	if err := m.storage.Get(ctx, installStorageKey, &val); err != nil {
		return m.openInstallPrompt(pkgID, version)
	}
	if !val.Value {
		// signals that package does not exist, which
		// should prevent further attempts or errors being logged.
		return nil, storageapi.ErrNotFound
	}
	pw := text.NewNotifyProgressWriter(m.n, m.interrupter,
		fmt.Sprintf("install %s@%s", pkgID, version), m.scheduleNextTick)
	if err := m.pkg.InstallPackageVersion(ctx, pkgID, version, pw); err != nil {
		return nil, fmt.Errorf("install latest version: %w", err)
	}
	return m.pkg.LibDir(ctx, pkgID)
}

func (m *pkgManager) getLatestVersion(
	ctx context.Context, pack string,
) (release.Version, error) {
	p, err := m.pkg.DescribePackage(ctx, pack)
	if err != nil {
		if err.Error() == "not found" {
			return "", storageapi.ErrNotFound
		}
		return "", err
	}
	if p.Latest == "" {
		iter, err := m.pkg.ListPackageVersions(ctx, pack, nil)
		if err != nil {
			return "", fmt.Errorf("list %q versions: %w", pack, err)
		}
		// this could be pretty slow, so hopefully either there's not many package
		// versions or Latest is always populated
		defer iter.Close()
		var latest time.Time
		var version release.Version
		for {
			bundle, ok := iter.Next(ctx)
			if !ok {
				break
			}
			if bundle.CreatedAt.After(latest) {
				latest = bundle.CreatedAt
				version = bundle.Version
			}
		}
		if err := iter.Err(); err != nil {
			return "", fmt.Errorf("iterate over versions of %q: %w", pack, err)
		}
		if version == "" {
			return "", fmt.Errorf("package %q has no releases", pack)
		}
		return version, nil
	}
	return p.Latest, nil
}

func (m *pkgManager) setAutoInstall() error {
	return m.storage.Set(context.Background(),
		installStorageKey, installStorageValue{Value: true})
}

func (m *pkgManager) openInstallPrompt(pkgID string, version release.Version) (
	iterator.Iterator[string], error,
) {
	const (
		yes       = "   Yes   "
		yesAlways = "   Yes, Always   "
		no        = "   No   "
		noNever   = "   No, Never   "
	)

	msg := fmt.Sprintf("Do you want to install package **%q**?", pkgID)

	ready := new(sync.Mutex)
	it := newPendingIterator(m.pkg, pkgID, ready)

	ctx := context.Background()
	ready.Lock()
	unlocked := false
	m.scheduleNextTick(func() {
		m.wh.focusEx().comp.Prompt(msg, []string{yes, yesAlways, no, noNever},
			[]term.KeyComb{{Ch: 'Y'}, {Ch: 'A'}, {Ch: 'N'}, {Ch: 'V'}},
			handler.FuncPromptHandler(
				func(i int, opt string) {
					defer func() { unlocked = true; ready.Unlock() }()
					var err error
					switch opt {
					case yesAlways:
						_ = m.storage.Set(ctx, installStorageKey, installStorageValue{Value: true})
						fallthrough
					case yes:
						pw := text.NewNotifyProgressWriter(m.n, m.interrupter,
							fmt.Sprintf("install %s@%s", pkgID, version), m.scheduleNextTick)
						err = m.pkg.InstallPackageVersion(ctx, pkgID, version, pw)
						if err == nil {
							it.it, err = m.pkg.LibDir(ctx, pkgID)
						}
					case noNever:
						_ = m.storage.Set(ctx, installStorageKey, installStorageValue{Value: false})
						fallthrough
					case no:
						err = storageapi.ErrNotFound
					}
					if err != nil {
						it.err = err
					}
				},
				func() error {
					if it.it == nil && it.err == nil {
						it.err = storageapi.ErrNotFound
					}
					if !unlocked {
						unlocked = true
						ready.Unlock()
					}
					m.pending.Delete(pkgID)
					return nil
				}))
	})

	m.pending.Store(pkgID, ready)
	return it, nil
}

func (m *pkgManager) Close() error {
	ret := m.uc.Close()
	if m.storage != nil {
		if err := m.storage.Close(); err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	return ret
}

type pkgManagerIterator struct {
	pkgID string
	ready *sync.Mutex
	pkg   *idepkg.Manager

	err error
	it  iterator.Iterator[string]
}

func newPendingIterator(
	pkg *idepkg.Manager, pkgID string, ready *sync.Mutex,
) *pkgManagerIterator {
	return &pkgManagerIterator{
		pkgID: pkgID,
		ready: ready,
		pkg:   pkg,
	}
}

func (l *pkgManagerIterator) Next(ctx context.Context) (string, bool) {
	l.ready.Lock()
	defer l.ready.Unlock()
	if l.err != nil {
		return "", false
	}
	if l.it == nil {
		l.it, l.err = l.pkg.LibDir(context.Background(), l.pkgID)
	}
	if l.err != nil {
		return "", false
	}
	return l.it.Next(ctx)
}

func (l *pkgManagerIterator) Err() error {
	l.ready.Lock()
	defer l.ready.Unlock()
	if l.err != nil {
		return l.err
	}
	if l.it == nil {
		l.it, l.err = l.pkg.LibDir(context.Background(), l.pkgID)
	}
	if l.err != nil {
		return l.err
	}
	return l.it.Err()
}

func (l *pkgManagerIterator) Close() error {
	l.ready.Lock()
	defer l.ready.Unlock()
	if l.it == nil {
		return nil
	}
	return l.it.Close()
}
