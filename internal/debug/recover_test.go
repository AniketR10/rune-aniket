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

package debug

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	sdkdebug "github.com/unstablebuild/rune-go-sdk/debug"
)

func TestCapturePanicReportWith_WritesReportUnderSuppliedDir(t *testing.T) {
	dir := t.TempDir()
	pkg := "testpkg"
	version := "v1.2.3"

	panicValue, err, ok := CapturePanicReportWith(dir, pkg, version, func() {
		panic("test crash")
	})

	if ok {
		t.Fatal("expected ok=false when panic occurs")
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if panicValue == nil {
		t.Fatal("expected non-nil panicValue")
	}
	if panicValue.(string) != "test crash" {
		t.Fatalf("unexpected panicValue: %v", panicValue)
	}

	// Verify report file was created in the supplied directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 report file, got %d", len(entries))
	}

	entry := entries[0]
	if !strings.HasPrefix(entry.Name(), pkg+"_crash_report_") {
		t.Errorf("report filename %q does not have expected prefix %q",
			entry.Name(), pkg+"_crash_report_")
	}

	// Verify report is valid non-empty YAML
	data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
	if err != nil {
		t.Fatalf("reading report: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("report file is empty")
	}

	var report sdkdebug.Report
	if err := yaml.Unmarshal(data, &report); err != nil {
		t.Fatalf("report is not valid YAML: %v", err)
	}
	if report.Package != pkg {
		t.Errorf("report.Package = %q, want %q", report.Package, pkg)
	}
	if report.Version != version {
		t.Errorf("report.Version = %q, want %q", report.Version, version)
	}
}

func TestCapturePanicReportWith_NoPanic_NoReportCreated(t *testing.T) {
	dir := t.TempDir()

	panicValue, err, ok := CapturePanicReportWith(dir, "testpkg", "v1.0.0", func() {
		// no panic
	})

	if !ok {
		t.Fatal("expected ok=true when no panic occurs")
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if panicValue != nil {
		t.Fatalf("expected nil panicValue, got %v", panicValue)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no report files when no panic, got %d", len(entries))
	}
}

func TestCapturePanicReport_WritesReportAndRepanics(t *testing.T) {
	dir := t.TempDir()
	origReportsDir := ReportsDir
	origPackage := Package
	origTag := Tag
	defer func() {
		ReportsDir = origReportsDir
		Package = origPackage
		Tag = origTag
	}()

	ReportsDir = dir
	Package = "capturepkg"
	Tag = "v9.9.9"

	var recovered any
	func() {
		defer func() {
			recovered = recover()
		}()
		CapturePanicReport(func() {
			panic("re-panic test")
		})
	}()

	if recovered == nil {
		t.Fatal("expected CapturePanicReport to re-panic")
	}
	if recovered.(string) != "re-panic test" {
		t.Fatalf("unexpected re-panic value: %v", recovered)
	}

	// Verify report was written
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 report file, got %d", len(entries))
	}
	if !strings.HasPrefix(entries[0].Name(), "capturepkg_crash_report_") {
		t.Errorf("report filename %q does not have expected prefix", entries[0].Name())
	}
}
