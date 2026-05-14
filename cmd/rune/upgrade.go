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
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/ide"
	"unstable.build/go-tui/ide/ideupgrade"
	"unstable.build/go-tui/text"
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
// still constructed so the `:upgrade` command continues to work.
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

// subscribeUpgradeCommands registers `:upgrade` against the given
// IDE. The command checks for a new release and, if one is
// available, prompts the user to install it. The user can decline
// at the prompt, so a single command serves both "check" and
// "upgrade" intents.
func subscribeUpgradeCommands(i *ide.IDE, mgr *ideupgrade.Manager) error {
	if mgr == nil {
		return nil
	}
	man := textapi.CommandManual{
		Name: "upgrade",
		Summary: "Check for a new Rune release and prompt to upgrade in place. " +
			"If no new release is available, a notification is shown instead.",
	}
	h := text.FuncCommandHandler(
		upgradeCommandHandler(mgr, i.Notifications()), nil)
	if err := i.SubscribeCommand(man, h); err != nil {
		return fmt.Errorf("subscribe '%s': %w", man.Name, err)
	}
	return nil
}

// upgradeCommandHandler returns the func body installed into the
// `:upgrade` command handler. It dispatches the manifest check to a
// background goroutine so the editor's event loop stays responsive:
// the manifest fetch performs network I/O, and showing the upgrade
// prompt requires posting work *back* to the event loop via
// ScheduleNextTick — work that can never run if the very same
// goroutine that handles input is parked here waiting on a socket.
//
// The handler returns nil immediately after posting an "in
// progress" notification so the user has visual feedback that the
// check is running. When CheckNow completes:
//   - on success, the progress notification is closed (`2/2`) and
//     CheckNow itself posts the "Rune is up to date" / upgrade
//     prompt notification.
//   - on failure, the progress notification is closed and a
//     **separate** error notification is posted. Closing the
//     progress notification dismisses it from the progress UI, so
//     the error message would otherwise vanish with it and the
//     user would see no resolution at all.
func upgradeCommandHandler(
	mgr *ideupgrade.Manager, n browserapi.Notifications,
) func(context.Context, textapi.Command) error {
	return func(_ context.Context, _ textapi.Command) error {
		// Post the "running" notification before we spawn the
		// goroutine so the user has feedback the moment the command
		// returns. Progress 1/2 anchors the notification in the
		// progress UI; closing it with 2/2 below dismisses it.
		notifID, nerr := n.Notify(browserapi.LevelInfo,
			"Checking for updates...")
		if nerr != nil {
			log.WithError(nerr).Warn("ideupgrade: notify check start")
		} else if perr := n.UpdateNotificationProgress(
			notifID, "", 1, 2); perr != nil {
			log.WithError(perr).Warn("ideupgrade: progress check start")
		}

		go debug.CapturePanicReport(func() {
			err := mgr.CheckNow(context.Background())
			if err != nil {
				log.WithError(err).Warn("ideupgrade: check now")
			}
			if notifID != "" {
				// Close the progress notification regardless of outcome.
				// CheckNow already posted any user-visible success
				// notification (up-to-date / upgrade prompt); on
				// failure we surface the error as a fresh notification
				// below so it isn't dismissed alongside the progress UI.
				if perr := n.UpdateNotificationProgress(
					notifID, "", 2, 2); perr != nil {
					log.WithError(perr).Warn(
						"ideupgrade: progress check end")
				}
			}
			if err != nil {
				if _, nerr := n.Notify(browserapi.LevelError,
					"Upgrade check failed: %v", err); nerr != nil {
					log.WithError(nerr).Warn(
						"ideupgrade: notify check failure")
				}
			}
		})
		return nil
	}
}
