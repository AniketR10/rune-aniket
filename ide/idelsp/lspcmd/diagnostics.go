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

package lspcmd

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/handler/locationpicker"
)

// DiagnosticsSource exposes a snapshot of the latest LSP diagnostics
// indexed by document URI. It is intentionally narrow so the
// `lsp diagnostics` subcommand does not depend on idelsp directly.
type DiagnosticsSource interface {
	Diagnostics() map[string][]semanticapi.Diagnostic
}

// DiagnosticsConfig configures the "diagnostics" subcommand.
type DiagnosticsConfig struct {
	ListConfig locationpicker.Config
}

// DefaultDiagnosticsConfig returns a DiagnosticsConfig with sensible
// defaults.
func DefaultDiagnosticsConfig() DiagnosticsConfig {
	return DiagnosticsConfig{
		ListConfig: locationpicker.DefaultConfig(),
	}
}

// DiagnosticsHandler creates a textapi.CommandHandler that lists all
// LSP diagnostics currently known to the workspace and lets the user
// jump to any of them via a floating location picker.
func DiagnosticsHandler(
	editor textapi.Editor,
	wm browserapi.WindowManager, opener browserapi.ResourceOpener,
	notify browserapi.Notifications, fs workspaceapi.FileSystem,
	rootURI workspaceapi.URI, scheduleNextTick func(func()) bool,
	parser syntaxapi.Parser, source DiagnosticsSource,
	cfg DiagnosticsConfig, log *slog.Logger,
) textapi.CommandHandler {
	return &diagnosticsHandler{
		editor: editor, wm: wm, opener: opener, notify: notify,
		fs: fs, rootURI: rootURI, scheduleNextTick: scheduleNextTick,
		parser: parser, source: source, cfg: cfg, log: log,
	}
}

type diagnosticsHandler struct {
	editor           textapi.Editor
	wm               browserapi.WindowManager
	opener           browserapi.ResourceOpener
	notify           browserapi.Notifications
	fs               workspaceapi.FileSystem
	rootURI          workspaceapi.URI
	scheduleNextTick func(func()) bool
	parser           syntaxapi.Parser
	source           DiagnosticsSource
	cfg              DiagnosticsConfig
	log              *slog.Logger
}

func (h *diagnosticsHandler) HandleCommand(_ context.Context, _ textapi.Command) error {
	if h.source == nil {
		_, _ = h.notify.Notify(browserapi.LevelInfo, "no diagnostics")
		return nil
	}
	diags := h.source.Diagnostics()
	entries := flattenDiagnostics(diags)
	if len(entries) == 0 {
		_, _ = h.notify.Notify(browserapi.LevelInfo, "no diagnostics")
		return nil
	}
	sortDiagnosticEntries(entries)
	locEntries := make([]locationEntry, len(entries))
	for i, e := range entries {
		locEntries[i] = e.entry
	}
	locEntries = enrichEntries(locEntries, h.rootURI)
	for i, e := range entries {
		locEntries[i].display = formatDiagnosticDisplay(locEntries[i].display, e.diag)
	}
	picker := locationpicker.New(
		pickerEntries(locEntries), h.wm, h.fs,
		h.scheduleNextTick, h.parser,
		h.cfg.ListConfig, h.log,
	)
	picker.SetOnSelect(func(idx int) {
		navigateTo(locEntries[idx], h.opener, h.wm, h.editor, h.notify, h.scheduleNextTick)
	})
	win, err := h.wm.Floating(picker, browserapi.FloatingConfig{Alignment: component.AlignmentCentered})
	if err != nil {
		return err
	}
	picker.SetWindow(win)
	return nil
}

func (h *diagnosticsHandler) Complete(
	_ context.Context, _ string, _ []string,
) (iterator.Iterator[string], error) {
	return iterator.Empty[string](), nil
}

// diagnosticEntry pairs a locationEntry with the originating
// diagnostic so we can sort by severity and decorate the display.
type diagnosticEntry struct {
	entry locationEntry
	diag  semanticapi.Diagnostic
}

func flattenDiagnostics(
	diags map[string][]semanticapi.Diagnostic,
) []diagnosticEntry {
	count := 0
	for _, ds := range diags {
		count += len(ds)
	}
	out := make([]diagnosticEntry, 0, count)
	for uri, ds := range diags {
		for _, d := range ds {
			out = append(out, diagnosticEntry{
				entry: locationEntry{
					uri: uri,
					rng: d.Range,
					display: fmt.Sprintf("%s:%d",
						trimFilePrefix(uri), d.Range.Start.Line+1),
				},
				diag: d,
			})
		}
	}
	return out
}

func sortDiagnosticEntries(entries []diagnosticEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		si := diagnosticSeverityRank(entries[i].diag.Severity)
		sj := diagnosticSeverityRank(entries[j].diag.Severity)
		if si != sj {
			return si < sj
		}
		if entries[i].entry.uri != entries[j].entry.uri {
			return entries[i].entry.uri < entries[j].entry.uri
		}
		ai := entries[i].diag.Range.Start
		bj := entries[j].diag.Range.Start
		if ai.Line != bj.Line {
			return ai.Line < bj.Line
		}
		return ai.Character < bj.Character
	})
}

// diagnosticSeverityRank assigns a sortable rank with errors first
// and unset severities last.
func diagnosticSeverityRank(s semanticapi.DiagnosticSeverity) int {
	switch s {
	case semanticapi.DiagnosticSeverityError:
		return 0
	case semanticapi.DiagnosticSeverityWarning:
		return 1
	case semanticapi.DiagnosticSeverityInformation:
		return 2
	case semanticapi.DiagnosticSeverityHint:
		return 3
	default:
		return 4
	}
}

func diagnosticSeverityTag(s semanticapi.DiagnosticSeverity) string {
	switch s {
	case semanticapi.DiagnosticSeverityError:
		return "E"
	case semanticapi.DiagnosticSeverityWarning:
		return "W"
	case semanticapi.DiagnosticSeverityInformation:
		return "I"
	case semanticapi.DiagnosticSeverityHint:
		return "H"
	default:
		return "?"
	}
}

func formatDiagnosticDisplay(loc string, d semanticapi.Diagnostic) string {
	tag := diagnosticSeverityTag(d.Severity)
	if d.Message == "" {
		return fmt.Sprintf("%s [%s]", loc, tag)
	}
	return fmt.Sprintf("%s [%s] %s", loc, tag, d.Message)
}
