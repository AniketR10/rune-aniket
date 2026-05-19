// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2025 Unstable Build, All Rights Reserved.
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
	"fmt"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/cmd/rune/crashreport"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/ide"
)

const (
	sendOpt     = "    Send    "
	dontSendOpt = "  Don't Send  "
)

var sendDontSendKeyCombs = []term.KeyComb{{Ch: 's'}, {Ch: 'd'}}

// checkCrashReports looks for unsent crash reports and prompts the user
// to send them. It runs on a background goroutine and synchronizes
// back to the event loop only when a prompt needs to be opened.
func checkCrashReports(
	i *ide.IDE,
	uploader crashreport.Uploader,
	dataDir string,
	scheduleNextTick func(func()) bool,
) {
	reportsDir := filepath.Join(dataDir, "reports")
	debugLogPath := filepath.Join(dataDir, "debug.log")
	storage := storageapi.WithPartition(i.Storage(), "crash_reports")

	mgr := crashreport.NewManager(
		reportsDir,
		debugLogPath,
		storage,
		uploader,
	)

	ctx := context.Background()

	// Materialize crash reports for fatal errors (stack overflow,
	// runtime.throw, unrecovered panics) that the runtime printed to
	// stderr in a prior session. recover() can not catch those, so
	// without this pass they would otherwise be lost.
	if _, err := mgr.IngestLaunchLog(
		ctx, crashreport.DefaultLaunchLogPath(dataDir), debug.Package, debug.Tag,
	); err != nil {
		log.Warnf("ingest launch log: %v", err)
	}

	pending, err := mgr.PendingReports(ctx)
	if err != nil {
		log.Warnf("check crash reports: %v", err)
		return
	}
	if len(pending) == 0 {
		return
	}

	var message string
	if len(pending) == 1 {
		message = "Rune **crashed** during a previous session. " +
			"Send a crash report with details and recent logs?"
	} else {
		message = fmt.Sprintf(
			"Rune **crashed** during a previous session (%d reports). "+
				"Send crash reports with details and recent logs?",
			len(pending))
	}

	promptHandler := &crashReportPromptHandler{
		i:       i,
		mgr:     mgr,
		pending: pending,
	}

	scheduleNextTick(func() {
		promptWindow := i.Prompt(
			message,
			[]string{sendOpt, dontSendOpt},
			sendDontSendKeyCombs,
			promptHandler,
		)
		promptHandler.promptWindow = promptWindow
	})
}

type crashReportPromptHandler struct {
	i            *ide.IDE
	mgr          *crashreport.Manager
	pending      []crashreport.ReportState
	promptWindow browser.Window
}

func (h *crashReportPromptHandler) OnSelect(idx int, option string) {
	if h.promptWindow != nil {
		_ = h.promptWindow.Close()
	}

	switch option {
	case sendOpt:
		go debug.CapturePanicReport(func() {
			h.sendReports()
		})
	case dontSendOpt:
		ctx := context.Background()
		h.mgr.DeclineReports(ctx, h.pending)
	}
}

func (h *crashReportPromptHandler) OnClose() error {
	return nil
}

func (h *crashReportPromptHandler) sendReports() {
	ctx := context.Background()
	sent, err := h.mgr.SendReports(ctx, h.pending)
	if err != nil {
		_, _ = h.i.Notifications().Notify(browserapi.LevelError,
			"Failed to send crash report: %v", err)
		if sent > 0 {
			_, _ = h.i.Notifications().Notify(browserapi.LevelInfo,
				"Sent %d of %d crash reports", sent, len(h.pending))
		}
		return
	}
	_, _ = h.i.Notifications().Notify(browserapi.LevelInfo,
		"Crash report sent. Thank you!")
}

// scheduleCrashReportCheck starts a background goroutine that scans for
// unsent crash reports and prompts the user to send them.
func scheduleCrashReportCheck(
	i *ide.IDE,
	uploader crashreport.Uploader,
	dataDir string,
	scheduleNextTick func(func()) bool,
) {
	go debug.CapturePanicReport(func() {
		checkCrashReports(i, uploader, dataDir, scheduleNextTick)
	})
}

// noopPromptHandler satisfies handler.PromptHandler for compile reference.
var _ handler.PromptHandler = (*crashReportPromptHandler)(nil)
