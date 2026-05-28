// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2024 Unstable Build, All Rights Reserved.
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

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/cmd/rune/ide/apiclient"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/ide"
	"unstable.build/go-tui/ide/ideupgrade"
	"unstable.build/go-tui/term/gui"
)

// bootstrapHandler is the root tui.Handler given to gui.New. It either
// fronts a pre-config IDE (until the bootstrap prompt completes) or the
// fully-configured IDE. On first launch, the prompt(s) collect the user's
// preferred editor mode and config format, performSwap writes the override
// config file, then a fresh configured IDE takes over.
type bootstrapHandler struct {
	inner             tui.Handler
	dataDir           string
	configPath        string
	workspace         string
	zdotDir           string
	filenames         []string
	launchCmd         []string
	runner            ide.ExtensionsRunner
	mu                *sync.Mutex
	publishEvent      func(term.Event) bool
	preIDE            *ide.IDE
	realIDE           *ide.IDE
	g                 *gui.GUI
	transparentWindow bool
	initialThemeAttr  term.Attributes
	client            *apiclient.Client
	upgradeMgr        *ideupgrade.Manager
	upgradeCancel     context.CancelFunc
	bootstrapClient   *apiclient.Client
	lastTabsClick     time.Time
	clickCount        int
	lastResizeW       int
	lastResizeH       int
	chosenEditor      string
	chosenFormat      string
	chosenByoePreset  string
}

func newRoot(
	dataDir, configPath, workspace, zdotDir string,
	filenames, launchCmd []string,
	runner ide.ExtensionsRunner,
	mu *sync.Mutex,
	publishEvent func(term.Event) bool,
) (*bootstrapHandler, error) {
	bh := &bootstrapHandler{
		dataDir:      dataDir,
		configPath:   configPath,
		workspace:    workspace,
		zdotDir:      zdotDir,
		filenames:    filenames,
		launchCmd:    launchCmd,
		runner:       runner,
		mu:           mu,
		publishEvent: publishEvent,
	}

	if isBootstrapped(dataDir) {
		realIDE, err := bh.buildConfiguredIDE()
		if err != nil {
			return nil, fmt.Errorf("build configured ide: %w", err)
		}
		bh.realIDE = realIDE
		bh.inner = realIDE.Ready()
		return bh, nil
	}

	preIDE, err := bh.buildPreIDE()
	if err != nil {
		return nil, fmt.Errorf("new pre-config ide: %w", err)
	}
	bh.preIDE = preIDE
	bh.inner = preIDE.Ready()
	bh.openBootstrapFlow()
	return bh, nil
}

func isBootstrapped(dataDir string) bool {
	if _, err := os.Stat(filepath.Join(dataDir, configFilename)); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(dataDir, configStarFilename)); err == nil {
		return true
	}
	return false
}

func (b *bootstrapHandler) buildPreIDE() (*ide.IDE, error) {
	opts := []ide.Option{
		ide.WithExtensionsRunner(b.runner),
		ide.WithLocker(b.mu),
		ide.WithDefaultConfigStarlark(defaultStarlarkConfig, true, false),
		ide.WithDefaultWallpaper(makeWallpaper()),
		ide.WithTabBarOffset(13),
		ide.WithTabBarHeight(2),
		ide.WithWorkspacesBarHeight(2),
		ide.WithWorkspacesBarOffset(2),
		ide.WithWorkspacesIcon('1'),
		ide.WithWorkspacesBarFrame(false),
		ide.WithStreamingOpen(true),
		ide.WithBell(func() {}),
		ide.WithPublishEvent(b.publishEvent),
		ide.WithScheduleNextTick(b.scheduleNextTick),
		ide.WithZdotDir(b.zdotDir),
	}
	preIDE, err := ide.New("", b.configPath, b.dataDir, opts...)
	if err != nil {
		return nil, err
	}
	b.applyInitialThemeAttr(preIDE)
	return preIDE, nil
}

func (b *bootstrapHandler) buildConfiguredIDE() (*ide.IDE, error) {
	opts := []ide.Option{
		ide.WithExtensionsRunner(b.runner),
		ide.WithInitShader(initShader, initShaderFPS, initShaderDuration),
		ide.WithShutdownShader(shutdownShader, 30, shutdownShaderDuration),
		ide.WithLoadingShader(loadingShader, loadingShaderFPS, loadingShaderDuration),
		ide.WithOpenShader(openShader, openShaderFPS, openShaderDuration),
		ide.WithStreamingOpen(true),
		ide.WithLocker(b.mu),
		ide.WithConfigFilename(workspaceConfigFilename),
		ide.WithDefaultWallpaper(makeWallpaper()),
		ide.WithTabBarOffset(13),
		ide.WithTabBarHeight(2),
		ide.WithWorkspacesBarHeight(2),
		ide.WithWorkspacesBarOffset(2),
		ide.WithWorkspacesIcon('1'),
		ide.WithWorkspacesBarFrame(false),
		ide.WithDefaultConfigStarlark(defaultStarlarkConfig, true, false),
		ide.WithBell(func() {}),
		ide.WithPublishEvent(b.publishEvent),
		ide.WithScheduleNextTick(b.scheduleNextTick),
		ide.WithZdotDir(b.zdotDir),
		ide.WithScheme(docsScheme, newDocsSchemeFunc(b.configPath)),
		// maximize window on double click
		ide.WithTabsClickCallback(func(_ int) bool {
			if b.clickCount == 0 || time.Since(b.lastTabsClick) < doubleClickTimeout {
				b.clickCount++
			} else {
				b.clickCount = 1
			}
			b.lastTabsClick = time.Now()
			if b.clickCount == 2 && b.g != nil {
				b.g.MaximizeWindow()
				return true
			}
			return false
		}),
		ide.WithDispatchOnPreview(cmdSetTheme,
			func(cmd string, args ...string) (component.Responsive, func(), bool) {
				if cmd != cmdSetTheme || b.g == nil {
					return nil, nil, false
				}
				if len(args) == 0 {
					return nil, nil, false
				}
				theme := b.g.Theme()
				_, err := b.g.SetTheme(args[0])
				if err != nil {
					return nil, nil, false
				}
				return nil, func() {
					theme, err := b.g.SetTheme(theme)
					if err == nil && b.realIDE != nil {
						b.realIDE.SetDefaultAttributes(term.Attributes{
							Fg: term.FromTcellColor(theme.Foreground),
							Bg: term.FromTcellColor(theme.Background),
						})
					}
				}, true
			}),
	}
	opts = append(opts, embeddedTutorialOptions()...)
	if debug.DebugBuild == "true" {
		opts = append(opts, ide.WithDebugCommands(true))
	}
	realIDE, err := ide.New(b.workspace, b.configPath, b.dataDir, opts...)
	if err != nil {
		return nil, err
	}
	b.applyInitialThemeAttr(realIDE)
	return realIDE, nil
}

func (b *bootstrapHandler) attachGUI(g *gui.GUI, transparentWindow bool) {
	b.g = g
	b.transparentWindow = transparentWindow
}

// applyInitialThemeAttr seeds the IDE with the configured GUI theme
// background before any caller invokes Ready(). Ready() materializes
// the init shader and captures defAttr at that moment, so a later
// SetDefaultAttributes would not propagate into the running shader
// (RUNE-203). Keeping both the pre-bootstrap and configured IDEs on
// the same attrs also avoids a color jump across performSwap.
func (b *bootstrapHandler) applyInitialThemeAttr(i *ide.IDE) {
	b.initialThemeAttr = resolveInitialThemeAttr(i.Browser(), i.Config())
	i.SetDefaultAttributes(b.initialThemeAttr)
}

func (b *bootstrapHandler) browser() browser.Browser {
	if b.realIDE != nil {
		return b.realIDE.Browser()
	}
	return b.preIDE.Browser()
}

func (b *bootstrapHandler) notifications() browserapi.Notifications {
	return bootstrapNotifications{b: b}
}

func (b *bootstrapHandler) alreadyBootstrapped() bool {
	return b.realIDE != nil
}

func (b *bootstrapHandler) config() config.Config {
	if b.realIDE != nil {
		return b.realIDE.Config()
	}
	return b.preIDE.Config()
}

func (b *bootstrapHandler) storage() storageapi.Service {
	if b.realIDE != nil {
		return b.realIDE.Storage()
	}
	return b.preIDE.Storage()
}

func (b *bootstrapHandler) setupConfiguredIDE(i *ide.IDE) {
	i.SetDefaultAttributes(b.initialThemeAttr)

	client, cerr := setupReleaseManager(i, i.Storage())
	if cerr != nil {
		log.Warnf("could not setup release manager: %v", cerr)
		// continue with nil client; commands that need it surface cerr.
	} else {
		b.client = client
		scheduleCrashReportCheck(i, client, b.dataDir, b.scheduleNextTick)
	}

	if err := subscribeCommands(b.g, client, cerr, i,
		b.transparentWindow, b.configPath, b.launchCmd); err != nil {
		log.Errorf("subscribe to GUI commands: %v", err)
	}

	openFiles(i, b.filenames)

	upgradeCtx, upgradeCancel := context.WithCancel(context.Background())
	b.upgradeCancel = upgradeCancel
	b.upgradeMgr = scheduleUpgradeCheck(upgradeCtx, i,
		apiclient.DefaultDownloadsHost, b.scheduleNextTick)
	if err := subscribeUpgradeCommands(i, b.upgradeMgr); err != nil {
		log.Errorf("subscribe upgrade commands: %v", err)
	}
}

func (b *bootstrapHandler) scheduleNextTick(fn func()) bool {
	return b.publishEvent(term.Event{Type: term.EventInterrupt, UserFunc: fn})
}

func (b *bootstrapHandler) Resize(w, h int) {
	b.lastResizeW = w
	b.lastResizeH = h
	b.inner.Resize(w, h)
}

func (b *bootstrapHandler) Draw(w term.Writer) { b.inner.Draw(w) }

func (b *bootstrapHandler) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return b.inner.Cursor()
}

func (b *bootstrapHandler) Selection() (string, bool) {
	return b.inner.Selection()
}

func (b *bootstrapHandler) Handle(ev term.Event) (exit, handled bool) {
	// While the bootstrap flow is still owning the screen, swallow
	// events that would let the user open the command prompt or
	// quit/close the only thing on screen. Esc is intentionally not
	// swallowed here; the guardedPromptChain close callback re-opens
	// the prompt instead. See bootstrap.go shouldSwallowBootstrapEvent.
	if b.realIDE == nil && shouldSwallowBootstrapEvent(ev) {
		return false, true
	}
	return b.inner.Handle(ev)
}

func (b *bootstrapHandler) performSwap() {
	if err := b.writeOverrideConfig(); err != nil {
		log.Errorf("write bootstrap override: %v", err)
		return
	}

	b.configPath = filepath.Join(b.dataDir, configFilenameForFormat(b.chosenFormat))

	b.mu.Unlock()
	defer b.mu.Lock()

	realIDE, err := b.buildConfiguredIDE()
	if err != nil {
		log.Errorf("build configured ide: %v", err)
		return
	}
	b.realIDE = realIDE
	b.setupConfiguredIDE(realIDE)
	b.inner = realIDE.Ready()
	if b.lastResizeW > 0 && b.lastResizeH > 0 {
		b.inner.Resize(b.lastResizeW, b.lastResizeH)
	}

	if b.preIDE != nil {
		if cerr := b.preIDE.Close(); cerr != nil {
			log.Warnf("close pre-config ide: %v", cerr)
		}
		b.preIDE = nil
	}
}

func (b *bootstrapHandler) writeOverrideConfig() error {
	body, err := renderOverride(b.chosenEditor, b.chosenFormat, b.chosenByoePreset)
	if err != nil {
		return fmt.Errorf("render override: %w", err)
	}
	path := filepath.Join(b.dataDir, configFilenameForFormat(b.chosenFormat))
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write override config %q: %w", path, err)
	}
	return nil
}

func (b *bootstrapHandler) Close() error {
	var errs []error
	if b.upgradeMgr != nil {
		if err := b.upgradeMgr.Close(); err != nil {
			errs = append(errs, err)
		}
		b.upgradeMgr = nil
	}
	if b.upgradeCancel != nil {
		b.upgradeCancel()
		b.upgradeCancel = nil
	}
	if b.client != nil {
		if err := b.client.Close(); err != nil {
			errs = append(errs, err)
		}
		b.client = nil
	}
	if b.realIDE != nil {
		if err := b.realIDE.Close(); err != nil {
			errs = append(errs, err)
		}
		b.realIDE = nil
	}
	if b.preIDE != nil {
		if err := b.preIDE.Close(); err != nil {
			errs = append(errs, err)
		}
		b.preIDE = nil
	}
	return errors.Join(errs...)
}
