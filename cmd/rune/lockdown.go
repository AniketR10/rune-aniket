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

package main

import (
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"unstable.build/go-tui/cmd/rune/ide/apiclient"
	"unstable.build/go-tui/ide"
)

// lockdownReSignIn implements the Re-signin button on the IDE
// lockdown overlay. It purges the cached token so a stale paid
// session cannot resurrect, drives a fresh OAuth flow through the
// apiclient, and asks the IDE to re-evaluate gating once the new
// token lands so the overlay tears down without waiting for the
// daily monitor tick.
//
// The caller is expected to invoke this on a background goroutine;
// the OAuth Login call blocks until the user finishes the browser
// flow (or cancels), which is well beyond a single event-loop tick.
func lockdownReSignIn(
	client *apiclient.Client,
	i *ide.IDE,
	scheduleNextTick func(func()) bool,
) {
	notifs := i.Notifications()
	if notifs != nil {
		_, _ = notifs.Notify(browserapi.LevelInfo,
			"Re-authenticating. Follow the prompts in your browser.")
	}
	if ts := client.CachedTokenSource(); ts != nil {
		if err := ts.Purge(); err != nil {
			log.WithError(err).Warn("lockdown re-signin: purge cached token")
		}
	}
	session := client.Login(context.Background())
	err, ok := <-session.Done
	if !ok || err != nil {
		if notifs != nil {
			_, _ = notifs.Notify(browserapi.LevelWarn,
				"Re-sign in did not complete. Try again or check your browser.")
		}
		return
	}
	scheduleNextTick(func() { i.TickPlan(context.Background()) })
}
