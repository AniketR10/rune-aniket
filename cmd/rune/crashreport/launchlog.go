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

package crashreport

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	sdkdebug "github.com/unstablebuild/rune-go-sdk/debug"
	"gopkg.in/yaml.v3"
)

// launchLogOffsetID is the storage key under which the manager persists the
// byte offset of the launch log it has already ingested.
const launchLogOffsetID = "launchlog_offset"

// maxFatalEventBytes caps a single parsed fatal block so a runaway log can
// not blow up memory. Real go runtime dumps are well under this size even
// with thousands of goroutines once "frames elided" collapses are applied.
const maxFatalEventBytes = 1 << 20 // 1 MiB

// FatalEvent is a single fatal-error block parsed out of a launch log.
//
// The block spans from the line that triggered the fatal (a "fatal error:",
// "panic:" or "runtime: goroutine stack exceeds" marker) through the end of
// the runtime's goroutine dump. The Header is the first marker line, useful
// as a crash subject. Body is the full block text.
type FatalEvent struct {
	Header string
	Body   string
}

// launchLogOffsetState is what we persist to storage to dedupe across runs.
type launchLogOffsetState struct {
	Offset int64 `json:"Offset"`
}

// fatalMarkers lists the prefixes that begin a Go runtime fatal block.
//
// "runtime: goroutine stack exceeds" precedes a stack-overflow "fatal
// error: stack overflow", but it is the first user-visible line so we
// anchor on it when present; otherwise we anchor on the next marker.
var fatalMarkers = []string{
	"runtime: goroutine stack exceeds",
	"fatal error:",
	"panic:",
}

// ScanFatal reads r and returns each fatal-error block it contains.
//
// Multiple back-to-back fatals are returned in order. Lines that do not
// belong to any fatal block are discarded. The returned events do not
// retain references to r; callers may close it as soon as ScanFatal returns.
func ScanFatal(r io.Reader) ([]FatalEvent, error) {
	scanner := bufio.NewScanner(r)
	// Stack dumps include long lines (full goroutine stacks); 1 MiB token
	// is plenty without inviting pathological growth.
	scanner.Buffer(make([]byte, 64*1024), 1<<20)

	var (
		events       []FatalEvent
		current      *FatalEvent
		size         int
		sawGoroutine bool
	)
	flush := func() {
		if current == nil {
			return
		}
		events = append(events, *current)
		current = nil
		size = 0
		sawGoroutine = false
	}
	for scanner.Scan() {
		line := scanner.Text()
		if isFatalMarker(line) {
			// A second marker that appears before the goroutine-stack
			// section of the current block is part of the same fatal
			// (e.g. "runtime: goroutine stack exceeds" is immediately
			// followed by "fatal error: stack overflow"). Only treat
			// it as a new block once we've already entered the
			// goroutine dump of the prior one.
			if current == nil || sawGoroutine {
				flush()
				current = &FatalEvent{Header: line, Body: line + "\n"}
				size = len(line) + 1
				continue
			}
		}
		if current == nil {
			continue
		}
		if strings.HasPrefix(line, "goroutine ") &&
			strings.Contains(line, "[") {
			sawGoroutine = true
		}
		add := len(line) + 1
		if size+add > maxFatalEventBytes {
			// Cap the block but keep what we have; the next marker
			// starts a fresh event.
			continue
		}
		current.Body += line + "\n"
		size += add
	}
	if err := scanner.Err(); err != nil {
		return events, fmt.Errorf("scan launch log: %w", err)
	}
	flush()
	return events, nil
}

func isFatalMarker(line string) bool {
	for _, m := range fatalMarkers {
		if strings.HasPrefix(line, m) {
			return true
		}
	}
	return false
}

// BuildLaunchLogReport renders a FatalEvent as a crash-report YAML that
// matches the layout produced by debug.CapturePanicReport, so the rest of
// the pipeline (PendingReports, BuildPayload, SendReports) treats it
// identically to a recover()-captured panic.
func BuildLaunchLogReport(pkg, version string, ev FatalEvent) ([]byte, error) {
	report := sdkdebug.BuildCrashReport(pkg, version, ev.Header)
	// Replace the debug.Stack() snapshot (which is just this process'
	// current stack) with the actual fatal block from the prior run.
	report.Metadata["stack"] = ev.Body
	report.Metadata["source"] = "launchlog"

	data, err := yaml.Marshal(report)
	if err != nil {
		return nil, fmt.Errorf("marshal launchlog report: %w", err)
	}
	return data, nil
}

// IngestLaunchLog scans launchLogPath for fatal blocks the runtime printed
// to stderr (stack overflow, runtime.throw, unrecovered panics that escaped
// CapturePanicReport) and materializes each new block as a YAML crash
// report under m.reportsDir, where PendingReports will pick it up.
//
// The byte offset of the last fully-processed log is persisted to
// m.storage so subsequent invocations only see new content. Returns the
// number of new reports written.
//
// Errors are best-effort: ingestion is a recovery aid, not a critical
// path. Callers should log but not fail on errors.
func (m *Manager) IngestLaunchLog(
	ctx context.Context, launchLogPath, pkg, version string,
) (int, error) {
	if launchLogPath == "" {
		return 0, nil
	}

	f, err := os.Open(launchLogPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("open launch log %q: %w", launchLogPath, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return 0, fmt.Errorf("stat launch log %q: %w", launchLogPath, err)
	}
	size := info.Size()

	var prev launchLogOffsetState
	if err := m.storage.Get(ctx, launchLogOffsetID, &prev); err != nil &&
		!errors.Is(err, storageapi.ErrNotFound) {
		log.Warnf("get launch log offset: %v", err)
	}

	// If the log was truncated or replaced, reset to the start.
	offset := prev.Offset
	if offset > size {
		offset = 0
	}
	if offset == size {
		return 0, nil
	}

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return 0, fmt.Errorf("seek launch log: %w", err)
	}

	events, scanErr := ScanFatal(f)
	// Even if scanning hit an error mid-file, flush whatever we got.
	written := 0
	for _, ev := range events {
		if err := m.writeLaunchLogReport(pkg, version, ev); err != nil {
			log.Warnf("write launchlog crash report: %v", err)
			continue
		}
		written++
	}

	// On a clean scan, truncate the launch log so the next session
	// starts from an empty file and the producer (which opens with
	// O_APPEND) keeps appending normally. Reset the persisted offset
	// to 0 to match. If scanning hit a transient error we leave the
	// log intact and just advance the offset, trading one missed
	// event for stability without losing the raw dump on disk.
	nextOffset := size
	if scanErr == nil {
		// Close before truncating so we do not race with our own
		// read fd on platforms with strict file-handle semantics.
		_ = f.Close()
		if err := os.Truncate(launchLogPath, 0); err != nil {
			log.Warnf("truncate launch log %q: %v", launchLogPath, err)
		} else {
			nextOffset = 0
		}
	}
	next := launchLogOffsetState{Offset: nextOffset}
	if err := m.storage.Set(ctx, launchLogOffsetID, &next); err != nil {
		log.Warnf("set launch log offset: %v", err)
	}

	if scanErr != nil {
		return written, scanErr
	}
	return written, nil
}

func (m *Manager) writeLaunchLogReport(pkg, version string, ev FatalEvent) error {
	data, err := BuildLaunchLogReport(pkg, version, ev)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(m.reportsDir, 0o777); err != nil {
		return fmt.Errorf("mkdir reports dir: %w", err)
	}
	f, err := os.CreateTemp(m.reportsDir,
		fmt.Sprintf("%s_crash_report_launchlog_*.yaml", pkg))
	if err != nil {
		return fmt.Errorf("create launchlog report: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("write launchlog report %q: %w", f.Name(), err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close launchlog report %q: %w", f.Name(), err)
	}
	log.Warnf("saved launchlog crash report file://%v", f.Name())
	return nil
}

// DefaultLaunchLogPath returns the path Rune writes runtime stderr to when
// launched as a desktop app. It lives under the Rune data directory so the
// crash dump survives /tmp cleanup and stays close to the other Rune state
// (debug.log, reports/, sockets/). Keep this in sync with cmd/rune/main.go.
func DefaultLaunchLogPath(dataDir string) string {
	return filepath.Join(dataDir, "launch.log")
}
