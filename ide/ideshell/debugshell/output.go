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


package debugshell

import (
	"fmt"
	"os"
	"sync"

	"github.com/google/go-dap"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
)

// outputSink is a synchronised file-backed sink for DAP
// OutputEvent bodies. The same file collects every category
// (stdout, stderr, console, telemetry) so the user has a single
// log to inspect after the debuggee exits.
type outputSink struct {
	mu   sync.Mutex
	path string
	f    *os.File
}

// newOutputSink creates a fresh temp file for the active
// debug session and returns a sink that writes to it. dir is
// the directory the file is created in; an empty dir falls
// back to os.TempDir(). Errors are returned to the caller; a
// nil sink must not be used.
//
// The caller should pass the workspace root so the IDE's
// existing file-system watcher fires Write events as the
// session appends output, which causes the open editor tab to
// auto-reload. Files in os.TempDir() are not watched and the
// editor buffer would otherwise stay empty.
func newOutputSink(prefix, dir string) (*outputSink, error) {
	if prefix == "" {
		prefix = "rune-debugger-"
	}
	f, err := os.CreateTemp(dir, prefix+"*.log")
	if err != nil {
		return nil, fmt.Errorf("create debugger output file: %w", err)
	}
	return &outputSink{path: f.Name(), f: f}, nil
}

// Path returns the absolute path of the underlying log file.
func (s *outputSink) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// Append writes ev's body to the sink, prefixing each line
// with the event's category for context. Best-effort: write
// errors are returned but the sink remains usable.
func (s *outputSink) Append(ev *dap.OutputEvent) error {
	if s == nil || ev == nil {
		return nil
	}
	body := ev.Body
	if body.Output == "" {
		return nil
	}
	cat := body.Category
	if cat == "" {
		cat = "output"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f == nil {
		return nil
	}
	if _, err := fmt.Fprintf(s.f, "[%s] %s", cat, body.Output); err != nil {
		return err
	}
	// fsync after each event so the IDE's file-system watcher
	// observes a write event and reloads the open tab. Without
	// the explicit Sync the kernel may delay the on-disk update
	// long enough that the user sees the output in the REPL but
	// an empty file in the editor pane.
	return s.f.Sync()
}

// Close flushes and closes the underlying file. Safe to call
// multiple times. The file itself is left on disk so the user
// can inspect it after the session ends.
func (s *outputSink) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f == nil {
		return nil
	}
	err := s.f.Close()
	s.f = nil
	return err
}

// startOutputCapture creates a fresh output sink for the
// active debug session inside the workspace root. Returns the
// sink path so it can be reported to the user (typically as a
// workspace-relative path). The file is *not* opened in a
// browser tab: it is left on disk for the user to open, tail,
// or post-mortem-inspect at their own discretion. Splitting
// the shell window automatically was confusing UX and is now
// the user's choice.
func (h *Handler) startOutputCapture() (string, error) {
	if h.output != nil {
		// A previous launch left a sink open; reuse it. The
		// adapter cannot have started a new session without
		// resetSession having cleared output first, so this
		// branch should not normally fire — but is safe.
		return h.output.Path(), nil
	}
	sink, err := newOutputSink("rune-debugger-", h.outputDir())
	if err != nil {
		return "", err
	}
	h.mu.Lock()
	h.output = sink
	h.mu.Unlock()
	return sink.Path(), nil
}

// stopOutputCapture closes any active output sink. Idempotent.
func (h *Handler) stopOutputCapture() {
	h.mu.Lock()
	sink := h.output
	h.output = nil
	h.mu.Unlock()
	if sink != nil {
		_ = sink.Close()
	}
}

// outputDir returns the directory the per-session output sink
// file should live in. The IDE's file-system watcher only
// watches the workspace root, so placing the sink there is
// what causes Write events to fire as the debuggee runs and
// the open editor tab to auto-reload. Falls back to os.TempDir
// when no workspace URI is configured (tests that pass a
// zero-valued Config).
func (h *Handler) outputDir() string {
	if path := h.cfg.WorkspaceURI.Path(); path != "" {
		return path
	}
	return ""
}

// appendOutput writes ev to the active sink, if any. Best-
// effort: write errors are surfaced through the notify hook.
func (h *Handler) appendOutput(ev *dap.OutputEvent) {
	h.mu.Lock()
	sink := h.output
	h.mu.Unlock()
	if sink == nil {
		return
	}
	if err := sink.Append(ev); err != nil && h.notify != nil {
		h.notify(browserapi.LevelWarn,
			"debugger: write output: %v", err)
	}
}
