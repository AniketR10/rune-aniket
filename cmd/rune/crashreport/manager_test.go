// Copyright (C) 2017-2026 The Rune Authors
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

package crashreport

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"gopkg.in/yaml.v3"
)

func TestPendingReports(t *testing.T) {
	type pendingEnv struct {
		reportsDir string
		storage    storageapi.Service
		paths      map[string]string
	}

	tests := []struct {
		name        string
		setup       func(t *testing.T, env *pendingEnv)
		wantPending int
		wantErr     bool
		verify      func(t *testing.T, env *pendingEnv, pending []ReportState)
	}{
		{
			name: "finds new crash reports and ignores unrelated entries",
			setup: func(t *testing.T, env *pendingEnv) {
				env.paths["first"] = writeReport(t, env.reportsDir,
					"testpkg_crash_report_001", validReportYAML())
				env.paths["second"] = writeReport(t, env.reportsDir,
					"testpkg_crash_report_002", validReportYAML())
				writeReport(t, env.reportsDir, "unrelated.txt", "not a crash report")
				if err := os.Mkdir(filepath.Join(env.reportsDir, "nested_crash_report_dir"), 0755); err != nil {
					t.Fatal(err)
				}
			},
			wantPending: 2,
		},
		{
			name:        "empty directory returns no reports",
			wantPending: 0,
		},
		{
			name: "missing directory returns no reports",
			setup: func(t *testing.T, env *pendingEnv) {
				env.reportsDir = filepath.Join(t.TempDir(), "missing-reports")
			},
			wantPending: 0,
		},
		{
			name: "reports path that is a file returns an error",
			setup: func(t *testing.T, env *pendingEnv) {
				env.reportsDir = writeFile(t,
					filepath.Join(t.TempDir(), "reports-file"), "not a directory")
			},
			wantErr: true,
		},
		{
			name: "sent and declined reports are skipped",
			setup: func(t *testing.T, env *pendingEnv) {
				ctx := context.Background()
				sentPath := writeReport(t, env.reportsDir,
					"testpkg_crash_report_sent", validReportYAML())
				declinedPath := writeReport(t, env.reportsDir,
					"testpkg_crash_report_declined", validReportYAML())
				env.paths["pending"] = writeReport(t, env.reportsDir,
					"testpkg_crash_report_pending", validReportYAML())

				sentID := reportIDForPath(t, sentPath)
				if err := env.storage.Set(ctx, sentID, &ReportState{
					ReportID: sentID,
					Path:     sentPath,
					Sent:     true,
				}); err != nil {
					t.Fatal(err)
				}
				declinedID := reportIDForPath(t, declinedPath)
				if err := env.storage.Set(ctx, declinedID, &ReportState{
					ReportID: declinedID,
					Path:     declinedPath,
					Declined: true,
				}); err != nil {
					t.Fatal(err)
				}
			},
			wantPending: 1,
			verify: func(t *testing.T, env *pendingEnv, pending []ReportState) {
				if pending[0].Path != env.paths["pending"] {
					t.Fatalf("pending path = %q, want %q", pending[0].Path, env.paths["pending"])
				}
			},
		},
		{
			name: "unreadable crash report is skipped",
			setup: func(t *testing.T, env *pendingEnv) {
				writeReport(t, env.reportsDir, "testpkg_crash_report_valid", validReportYAML())
				linkPath := filepath.Join(env.reportsDir, "testpkg_crash_report_broken")
				if err := os.Symlink(filepath.Join(env.reportsDir, "missing-target"), linkPath); err != nil {
					t.Skipf("symlink not supported: %v", err)
				}
			},
			wantPending: 1,
		},
		{
			name: "storage get failure skips report without failing scan",
			setup: func(t *testing.T, env *pendingEnv) {
				writeReport(t, env.reportsDir, "testpkg_crash_report_001", validReportYAML())
				env.storage = &failingStorage{
					Service: env.storage,
					getErr:  errors.New("storage get down"),
				}
			},
			wantPending: 0,
		},
		{
			name: "storage create failure still returns current pending report",
			setup: func(t *testing.T, env *pendingEnv) {
				writeReport(t, env.reportsDir, "testpkg_crash_report_001", validReportYAML())
				env.storage = &failingStorage{
					Service:   env.storage,
					createErr: errors.New("storage create down"),
				}
			},
			wantPending: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := &pendingEnv{
				reportsDir: t.TempDir(),
				storage:    storagestub.NewInMemoryService(),
				paths:      make(map[string]string),
			}
			if test.setup != nil {
				test.setup(t, env)
			}

			m := newTestManager(env.reportsDir, "", env.storage, nil)
			pending, err := m.PendingReports(context.Background())
			if test.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("PendingReports: %v", err)
			}
			if len(pending) != test.wantPending {
				t.Fatalf("pending count = %d, want %d", len(pending), test.wantPending)
			}
			if test.verify != nil {
				test.verify(t, env, pending)
			}
		})
	}
}

func TestBuildPayload(t *testing.T) {
	type payloadEnv struct {
		reportsDir   string
		dataDir      string
		debugLogPath string
	}

	tests := []struct {
		name    string
		setup   func(t *testing.T, env *payloadEnv) ReportState
		wantErr string
		verify  func(t *testing.T, payload Payload)
	}{
		{
			name: "valid report without debug log returns original YAML",
			setup: func(t *testing.T, env *payloadEnv) ReportState {
				path := writeReport(t, env.reportsDir, "testpkg_crash_report_001", validReportYAML())
				return ReportState{ReportID: "report-1", Path: path}
			},
			verify: func(t *testing.T, payload Payload) {
				report := payloadYAMLMap(t, payload)
				if _, ok := report["logs"]; ok {
					t.Fatalf("logs field should be absent when no logs are available")
				}
				if report["package"] != "testpkg" {
					t.Fatalf("package = %q, want testpkg", report["package"])
				}
			},
		},
		{
			name: "valid report includes last 50 debug log lines",
			setup: func(t *testing.T, env *payloadEnv) ReportState {
				env.debugLogPath = writeNumberedDebugLog(t, env.dataDir, 60)
				path := writeReport(t, env.reportsDir, "testpkg_crash_report_001", validReportYAML())
				return ReportState{ReportID: "report-1", Path: path}
			},
			verify: func(t *testing.T, payload Payload) {
				report := payloadYAMLMap(t, payload)
				logs, _ := report["logs"].(string)
				if !strings.Contains(logs, "log line 11\n") {
					t.Errorf("logs should contain line 11, got:\n%s", logs)
				}
				if strings.Contains(logs, "log line 10\n") {
					t.Errorf("logs should not contain line 10, got:\n%s", logs)
				}
				if !strings.Contains(logs, "log line 60\n") {
					t.Errorf("logs should contain line 60, got:\n%s", logs)
				}
			},
		},
		{
			name: "missing debug log is ignored",
			setup: func(t *testing.T, env *payloadEnv) ReportState {
				env.debugLogPath = filepath.Join(env.dataDir, "missing-debug.log")
				path := writeReport(t, env.reportsDir, "testpkg_crash_report_001", validReportYAML())
				return ReportState{ReportID: "report-1", Path: path}
			},
			verify: func(t *testing.T, payload Payload) {
				report := payloadYAMLMap(t, payload)
				if _, ok := report["logs"]; ok {
					t.Fatalf("logs field should be absent when debug log is missing")
				}
			},
		},
		{
			name: "debug log directory is ignored",
			setup: func(t *testing.T, env *payloadEnv) ReportState {
				env.debugLogPath = env.dataDir
				path := writeReport(t, env.reportsDir, "testpkg_crash_report_001", validReportYAML())
				return ReportState{ReportID: "report-1", Path: path}
			},
			verify: func(t *testing.T, payload Payload) {
				report := payloadYAMLMap(t, payload)
				if _, ok := report["logs"]; ok {
					t.Fatalf("logs field should be absent when debug log cannot be scanned")
				}
			},
		},
		{
			name: "missing report returns an error",
			setup: func(t *testing.T, env *payloadEnv) ReportState {
				return ReportState{ReportID: "missing", Path: filepath.Join(env.reportsDir, "missing")}
			},
			wantErr: "read report",
		},
		{
			name: "invalid YAML report returns an error when logs must be added",
			setup: func(t *testing.T, env *payloadEnv) ReportState {
				env.debugLogPath = writeDebugLog(t, env.dataDir, []string{"debug line"})
				path := writeReport(t, env.reportsDir, "testpkg_crash_report_001", "not: [yaml")
				return ReportState{ReportID: "invalid", Path: path}
			},
			wantErr: "yaml decode",
		},
		{
			name: "scalar YAML report returns an error when logs must be added",
			setup: func(t *testing.T, env *payloadEnv) ReportState {
				env.debugLogPath = writeDebugLog(t, env.dataDir, []string{"debug line"})
				path := writeReport(t, env.reportsDir, "testpkg_crash_report_001", "just a scalar")
				return ReportState{ReportID: "scalar", Path: path}
			},
			wantErr: "report root must be a YAML mapping",
		},
		{
			name: "existing logs field is replaced",
			setup: func(t *testing.T, env *payloadEnv) ReportState {
				env.debugLogPath = writeDebugLog(t, env.dataDir, []string{"new log line"})
				path := writeReport(t, env.reportsDir,
					"testpkg_crash_report_001", validReportYAMLWithLogs("old log line"))
				return ReportState{ReportID: "replace-logs", Path: path}
			},
			verify: func(t *testing.T, payload Payload) {
				report := payloadYAMLMap(t, payload)
				logs, _ := report["logs"].(string)
				if logs != "new log line\n" {
					t.Fatalf("logs = %q, want %q", logs, "new log line\n")
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := &payloadEnv{
				reportsDir: t.TempDir(),
				dataDir:    t.TempDir(),
			}
			state := test.setup(t, env)
			m := newTestManager(env.reportsDir, env.debugLogPath,
				storagestub.NewInMemoryService(), nil)

			payload, err := m.BuildPayload(context.Background(), state)
			if test.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", test.wantErr)
				}
				if !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("error = %q, want substring %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("BuildPayload: %v", err)
			}
			if test.verify != nil {
				test.verify(t, payload)
			}
		})
	}
}

func TestSendReports(t *testing.T) {
	type sendEnv struct {
		reportsDir   string
		dataDir      string
		debugLogPath string
		storage      storageapi.Service
		uploader     *fakeUploader
		states       map[string]ReportState
	}

	collectPending := func(t *testing.T, env *sendEnv) []ReportState {
		t.Helper()
		m := newTestManager(env.reportsDir, env.debugLogPath, env.storage, env.uploader)
		pending, err := m.PendingReports(context.Background())
		if err != nil {
			t.Fatalf("PendingReports: %v", err)
		}
		for _, state := range pending {
			_, name := filepath.Split(state.Path)
			env.states[name] = state
		}
		return pending
	}

	tests := []struct {
		name     string
		setup    func(t *testing.T, env *sendEnv) (*Manager, []ReportState)
		wantSent int
		wantErr  string
		verify   func(t *testing.T, env *sendEnv, m *Manager)
	}{
		{
			name: "uploads reports and marks them sent",
			setup: func(t *testing.T, env *sendEnv) (*Manager, []ReportState) {
				writeReport(t, env.reportsDir, "testpkg_crash_report_001", validReportYAML())
				pending := collectPending(t, env)
				return newTestManager(env.reportsDir, env.debugLogPath, env.storage, env.uploader), pending
			},
			wantSent: 1,
			verify: func(t *testing.T, env *sendEnv, m *Manager) {
				if env.uploader.calls != 1 || len(env.uploader.payloads) != 1 {
					t.Fatalf("uploader calls=%d payloads=%d, want calls=1 payloads=1",
						env.uploader.calls, len(env.uploader.payloads))
				}
				requirePendingCount(t, m, 0)
			},
		},
		{
			name: "upload failure leaves report pending and records last error",
			setup: func(t *testing.T, env *sendEnv) (*Manager, []ReportState) {
				writeReport(t, env.reportsDir, "testpkg_crash_report_001", validReportYAML())
				env.uploader.err = errors.New("network error")
				pending := collectPending(t, env)
				return newTestManager(env.reportsDir, env.debugLogPath, env.storage, env.uploader), pending
			},
			wantSent: 0,
			wantErr:  "network error",
			verify: func(t *testing.T, env *sendEnv, m *Manager) {
				pending := requirePendingCount(t, m, 1)
				state := storedReportState(t, env.storage, pending[0].ReportID)
				if !strings.Contains(state.LastError, "network error") {
					t.Fatalf("LastError = %q, want network error", state.LastError)
				}
			},
		},
		{
			name: "second upload failure returns partial sent count",
			setup: func(t *testing.T, env *sendEnv) (*Manager, []ReportState) {
				writeReport(t, env.reportsDir, "testpkg_crash_report_001", validReportYAML())
				writeReport(t, env.reportsDir, "testpkg_crash_report_002", validReportYAML())
				env.uploader.err = errors.New("second upload failed")
				env.uploader.failOnCall = 2
				pending := collectPending(t, env)
				return newTestManager(env.reportsDir, env.debugLogPath, env.storage, env.uploader), pending
			},
			wantSent: 1,
			wantErr:  "second upload failed",
			verify: func(t *testing.T, env *sendEnv, m *Manager) {
				if env.uploader.calls != 2 || len(env.uploader.payloads) != 1 {
					t.Fatalf("uploader calls=%d payloads=%d, want calls=2 payloads=1",
						env.uploader.calls, len(env.uploader.payloads))
				}
				requirePendingCount(t, m, 1)
			},
		},
		{
			name: "build payload failure skips bad report and uploads next report",
			setup: func(t *testing.T, env *sendEnv) (*Manager, []ReportState) {
				env.debugLogPath = writeDebugLog(t, env.dataDir, []string{"debug line"})
				writeReport(t, env.reportsDir, "a_crash_report_invalid", "not: [yaml")
				writeReport(t, env.reportsDir, "b_crash_report_valid", validReportYAML())
				pending := collectPending(t, env)
				return newTestManager(env.reportsDir, env.debugLogPath, env.storage, env.uploader), pending
			},
			wantSent: 1,
			verify: func(t *testing.T, env *sendEnv, m *Manager) {
				if env.uploader.calls != 1 || len(env.uploader.payloads) != 1 {
					t.Fatalf("uploader calls=%d payloads=%d, want calls=1 payloads=1",
						env.uploader.calls, len(env.uploader.payloads))
				}
				pending := requirePendingCount(t, m, 1)
				state := storedReportState(t, env.storage, pending[0].ReportID)
				if !strings.Contains(state.LastError, "yaml decode") {
					t.Fatalf("LastError = %q, want yaml decode", state.LastError)
				}
			},
		},
		{
			name: "mark sent failure does not fail an already uploaded report",
			setup: func(t *testing.T, env *sendEnv) (*Manager, []ReportState) {
				writeReport(t, env.reportsDir, "testpkg_crash_report_001", validReportYAML())
				env.storage = &failingStorage{
					Service: env.storage,
					setErr:  errors.New("set failed"),
				}
				pending := collectPending(t, env)
				return newTestManager(env.reportsDir, env.debugLogPath, env.storage, env.uploader), pending
			},
			wantSent: 1,
			verify: func(t *testing.T, env *sendEnv, m *Manager) {
				if env.uploader.calls != 1 || len(env.uploader.payloads) != 1 {
					t.Fatalf("uploader calls=%d payloads=%d, want calls=1 payloads=1",
						env.uploader.calls, len(env.uploader.payloads))
				}
				requirePendingCount(t, m, 1)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := &sendEnv{
				reportsDir: t.TempDir(),
				dataDir:    t.TempDir(),
				storage:    storagestub.NewInMemoryService(),
				uploader:   &fakeUploader{},
				states:     make(map[string]ReportState),
			}
			m, reports := test.setup(t, env)

			sent, err := m.SendReports(context.Background(), reports)
			if test.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", test.wantErr)
				}
				if !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("error = %q, want substring %q", err, test.wantErr)
				}
			} else if err != nil {
				t.Fatalf("SendReports: %v", err)
			}
			if sent != test.wantSent {
				t.Fatalf("sent = %d, want %d", sent, test.wantSent)
			}
			if test.verify != nil {
				test.verify(t, env, m)
			}
		})
	}
}

func TestDeclineReports(t *testing.T) {
	tests := []struct {
		name        string
		wrapStorage func(storageapi.Service) storageapi.Service
		wantPending int
	}{
		{
			name:        "marks all reports declined",
			wantPending: 0,
		},
		{
			name: "storage failure leaves reports pending without panicking",
			wrapStorage: func(storage storageapi.Service) storageapi.Service {
				return &failingStorage{
					Service: storage,
					setErr:  errors.New("set failed"),
				}
			},
			wantPending: 2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reportsDir := t.TempDir()
			writeReport(t, reportsDir, "testpkg_crash_report_001", validReportYAML())
			writeReport(t, reportsDir, "testpkg_crash_report_002", validReportYAML())

			storage := storageapi.Service(storagestub.NewInMemoryService())
			if test.wrapStorage != nil {
				storage = test.wrapStorage(storage)
			}
			m := newTestManager(reportsDir, "", storage, nil)
			pending := requirePendingCount(t, m, 2)

			m.DeclineReports(context.Background(), pending)
			requirePendingCount(t, m, test.wantPending)
		})
	}
}

type fakeUploader struct {
	payloads   []Payload
	err        error
	failOnCall int
	calls      int
}

func (f *fakeUploader) PostReport(_ context.Context, p Payload) error {
	f.calls++
	if f.err != nil && (f.failOnCall == 0 || f.calls == f.failOnCall) {
		return f.err
	}
	f.payloads = append(f.payloads, p)
	return nil
}

type failingStorage struct {
	storageapi.Service
	getErr    error
	createErr error
	setErr    error
	updateErr error
}

func (s *failingStorage) Get(ctx context.Context, id string, doc any) error {
	if s.getErr != nil {
		return s.getErr
	}
	return s.Service.Get(ctx, id, doc)
}

func (s *failingStorage) Create(ctx context.Context, id string, doc any) error {
	if s.createErr != nil {
		return s.createErr
	}
	return s.Service.Create(ctx, id, doc)
}

func (s *failingStorage) Set(ctx context.Context, id string, doc any) error {
	if s.setErr != nil {
		return s.setErr
	}
	return s.Service.Set(ctx, id, doc)
}

func (s *failingStorage) Update(
	ctx context.Context,
	id string,
	updates []storageapi.Update,
	precond ...storageapi.Precondition,
) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	return s.Service.Update(ctx, id, updates, precond...)
}

func writeFile(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeReport(t *testing.T, dir, name, content string) string {
	t.Helper()
	return writeFile(t, filepath.Join(dir, name), content)
}

func validReportYAML() string {
	return `package: testpkg
version: v1.0.0
author: debug.CapturePanic
subject: crash
metadata:
    error: crash
    stack: |
        goroutine 1 [running]:
        runtime/debug.Stack()
`
}

func validReportYAMLWithLogs(logs string) string {
	return validReportYAML() + "logs: " + fmt.Sprintf("%q", logs) + "\n"
}

func newTestManager(
	reportsDir string,
	debugLogPath string,
	storage storageapi.Service,
	uploader Uploader,
) *Manager {
	return NewManager(reportsDir, debugLogPath, storage, uploader)
}

func reportIDForPath(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return reportID(path, data)
}

func storedReportState(t *testing.T, storage storageapi.Service, id string) ReportState {
	t.Helper()
	var state ReportState
	if err := storage.Get(context.Background(), id, &state); err != nil {
		t.Fatalf("get stored report state %q: %v", id, err)
	}
	return state
}

func writeDebugLog(t *testing.T, dir string, lines []string) string {
	t.Helper()
	path := filepath.Join(dir, "debug.log")
	writeFile(t, path, strings.Join(lines, "\n")+"\n")
	return path
}

func writeNumberedDebugLog(t *testing.T, dir string, count int) string {
	t.Helper()
	lines := make([]string, 0, count)
	for i := 1; i <= count; i++ {
		lines = append(lines, fmt.Sprintf("log line %d", i))
	}
	return writeDebugLog(t, dir, lines)
}

func payloadYAMLMap(t *testing.T, payload Payload) map[string]any {
	t.Helper()
	var report map[string]any
	if err := yaml.Unmarshal(payload.YAML, &report); err != nil {
		t.Fatalf("payload YAML is invalid: %v", err)
	}
	return report
}

func requirePendingCount(
	t *testing.T,
	m *Manager,
	want int,
) []ReportState {
	t.Helper()
	pending, err := m.PendingReports(context.Background())
	if err != nil {
		t.Fatalf("PendingReports: %v", err)
	}
	if len(pending) != want {
		t.Fatalf("pending count = %d, want %d", len(pending), want)
	}
	return pending
}
