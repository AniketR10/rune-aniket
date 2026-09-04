// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
	"unstable.build/rune/browser"
	"unstable.build/rune/cmd/rune/crashreport"
	"unstable.build/rune/debug"
	"unstable.build/rune/ide"
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
