// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2026 Unstable Build, All Rights Reserved.
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
	"runtime"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/ide"
	"unstable.build/go-tui/ide/ideupgrade"
	"unstable.build/go-tui/ide/upgradeshell"
)

// upgradeConfig captures the values read from the "upgrade" stanza of
// the user's starlark configuration.
type upgradeConfig struct {
	autoCheckEnabled bool
	checkPeriod      time.Duration
}

func defaultUpgradeConfig() upgradeConfig {
	return upgradeConfig{
		autoCheckEnabled: true,
		checkPeriod:      ideupgrade.DefaultCheckPeriod,
	}
}

// loadUpgradeConfig reads the upgrade stanza from the IDE configuration.
// It is forgiving: missing values fall back to defaults.
func loadUpgradeConfig(i *ide.IDE) upgradeConfig {
	out := defaultUpgradeConfig()
	cfg, err := i.Config().GetConfig("upgrade")
	if err != nil {
		if !errors.Is(err, config.ErrNotFound) {
			log.Warnf("load upgrade config: %v", err)
		}
		return out
	}
	if v, err := cfg.GetBool("auto_check_enabled"); err == nil {
		out.autoCheckEnabled = v
	} else if !errors.Is(err, config.ErrNotFound) {
		log.Warnf("load upgrade.auto_check_enabled: %v", err)
	}
	if v, err := cfg.GetString("check_period"); err == nil {
		if d, perr := time.ParseDuration(v); perr == nil {
			out.checkPeriod = d
		} else {
			log.Warnf("parse upgrade.check_period %q: %v", v, perr)
		}
	} else if !errors.Is(err, config.ErrNotFound) {
		log.Warnf("load upgrade.check_period: %v", err)
	}
	return out
}

// scheduleUpgradeCheck wires up an ideupgrade.Manager and starts its
// background check loop. When auto-check is disabled the manager is
// still constructed so the `upgrade` command continues to work.
//
// Returns the Manager (which may be nil if construction failed or the
// manifest URL is unavailable) so the caller can register commands
// that operate on it.
func scheduleUpgradeCheck(
	ctx context.Context, i *ide.IDE, manifestBaseURL string,
	scheduleNextTick func(func()) bool,
) *ideupgrade.Manager {
	if manifestBaseURL == "" {
		return nil
	}
	upCfg := loadUpgradeConfig(i)

	mgr, err := ideupgrade.New(ideupgrade.Config{
		CurrentVersion:   debug.Tag,
		Arch:             fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH),
		ManifestURL:      manifestBaseURL,
		Storage:          i.Storage(),
		Notifications:    i.Notifications(),
		WindowManager:    i.WindowManager(),
		ScheduleNextTick: scheduleNextTick,
		CheckPeriod:      upCfg.checkPeriod,
	})
	if err != nil {
		_, _ = i.Notifications().Notify(browserapi.LevelError,
			"Could not initialize upgrade checker: %v", err)
		return nil
	}

	if upCfg.autoCheckEnabled {
		mgr.Start(ctx)
	}
	return mgr
}

// registerUpgradeCommand registers the `upgrade` console command
// against the given IDE. The command checks for a new release and, if
// one is available, prompts the user to install it. The user can
// decline at the prompt, so a single command serves both "check" and
// "upgrade" intents.
//
// It is a console command rather than an ex-command so the manifest
// fetch, the confirmation prompt and the install all run off the
// editor's event loop, and so the install can stream its progress into
// the console.
func registerUpgradeCommand(i *ide.IDE, mgr *ideupgrade.Manager) error {
	if mgr == nil {
		return nil
	}
	h := upgradeshell.New(upgradeshell.Config{Manager: mgr})
	if err := i.RegisterREPLCommand(upgradeshell.Manual(), h); err != nil {
		return fmt.Errorf("register '%s': %w", upgradeshell.CommandName, err)
	}
	return nil
}
