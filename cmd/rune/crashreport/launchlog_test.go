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

package crashreport

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"gopkg.in/yaml.v3"
)

const stackOverflowFatal = `runtime: goroutine stack exceeds 1000000000-byte limit
runtime: sp=0x109cf5088330 stack=[0x109cf5088000, 0x109d15088000]
fatal error: stack overflow

runtime stack:
runtime.throw({0x1015a4f8d?, 0x100750c94?})
	/go/src/runtime/panic.go:1229 +0x38
goroutine 1228 [running]:
unstable.build/rune/term/vte.(*viHandler).Edit(...)
	/Users/x/term/vte/vi.go:375 +0x6b8
`

const concurrentMapFatal = `fatal error: concurrent map writes

goroutine 42 [running]:
main.main()
	/tmp/x.go:10 +0x18
`

func TestScanFatal(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantCount  int
		wantHeader []string
	}{
		{
			name:      "empty input has no events",
			input:     "",
			wantCount: 0,
		},
		{
			name:      "non-fatal noise is discarded",
			input:     "time=2026 level=INFO msg=hello\nsome other line\n",
			wantCount: 0,
		},
		{
			name:      "single stack overflow block",
			input:     "warmup\n" + stackOverflowFatal,
			wantCount: 1,
			wantHeader: []string{
				"runtime: goroutine stack exceeds 1000000000-byte limit",
			},
		},
		{
			name:      "single fatal-error-only block",
			input:     concurrentMapFatal,
			wantCount: 1,
			wantHeader: []string{
				"fatal error: concurrent map writes",
			},
		},
		{
			name: "two back-to-back fatal blocks",
			input: stackOverflowFatal +
				"some noise between\n" +
				concurrentMapFatal,
			wantCount: 2,
			wantHeader: []string{
				"runtime: goroutine stack exceeds 1000000000-byte limit",
				"fatal error: concurrent map writes",
			},
		},
		{
			name: "unrecovered panic counts as fatal",
			input: `panic: runtime error: index out of range [3] with length 2

goroutine 1 [running]:
main.main()
	/tmp/x.go:10 +0x18
`,
			wantCount: 1,
			wantHeader: []string{
				"panic: runtime error: index out of range [3] with length 2",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			events, err := ScanFatal(strings.NewReader(test.input))
			if err != nil {
				t.Fatalf("ScanFatal: %v", err)
			}
			if len(events) != test.wantCount {
				t.Fatalf("events = %d, want %d", len(events), test.wantCount)
			}
			for i, want := range test.wantHeader {
				if events[i].Header != want {
					t.Errorf("event[%d].Header = %q, want %q",
						i, events[i].Header, want)
				}
				if !strings.HasPrefix(events[i].Body, want) {
					t.Errorf("event[%d].Body does not start with header", i)
				}
			}
		})
	}
}

func TestBuildLaunchLogReportEmbedsFullStack(t *testing.T) {
	ev := FatalEvent{
		Header: "fatal error: stack overflow",
		Body:   stackOverflowFatal,
	}
	data, err := BuildLaunchLogReport("rune", "v0.0.0-test", ev)
	if err != nil {
		t.Fatalf("BuildLaunchLogReport: %v", err)
	}

	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("yaml decode: %v", err)
	}

	if doc["package"] != "rune" {
		t.Errorf("package = %v, want rune", doc["package"])
	}
	meta, ok := doc["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("metadata is not a map: %T", doc["metadata"])
	}
	if meta["source"] != "launchlog" {
		t.Errorf("metadata.source = %v, want launchlog", meta["source"])
	}
	if got, _ := meta["stack"].(string); got != stackOverflowFatal {
		t.Errorf("metadata.stack did not round-trip the fatal body")
	}
	if got, _ := meta["error"].(string); got != ev.Header {
		t.Errorf("metadata.error = %q, want %q", got, ev.Header)
	}
}

func TestIngestLaunchLog(t *testing.T) {
	t.Run("missing log is a no-op", func(t *testing.T) {
		env := newLaunchLogEnv(t)
		n, err := env.mgr.IngestLaunchLog(
			context.Background(),
			filepath.Join(t.TempDir(), "missing.log"),
			"rune", "v0",
		)
		if err != nil {
			t.Fatalf("IngestLaunchLog: %v", err)
		}
		if n != 0 {
			t.Fatalf("n = %d, want 0", n)
		}
		assertReportCount(t, env.reportsDir, 0)
	})

	t.Run("emits one report per fatal block on first run", func(t *testing.T) {
		env := newLaunchLogEnv(t)
		writeFile(t, env.launchLogPath,
			"noise\n"+stackOverflowFatal+"more noise\n"+concurrentMapFatal)

		n, err := env.mgr.IngestLaunchLog(
			context.Background(), env.launchLogPath, "rune", "v0",
		)
		if err != nil {
			t.Fatalf("IngestLaunchLog: %v", err)
		}
		if n != 2 {
			t.Fatalf("n = %d, want 2", n)
		}
		assertReportCount(t, env.reportsDir, 2)
		assertFileEmpty(t, env.launchLogPath)
	})

	t.Run("second run with no new content emits nothing", func(t *testing.T) {
		env := newLaunchLogEnv(t)
		writeFile(t, env.launchLogPath, stackOverflowFatal)

		first, err := env.mgr.IngestLaunchLog(
			context.Background(), env.launchLogPath, "rune", "v0",
		)
		if err != nil {
			t.Fatalf("first IngestLaunchLog: %v", err)
		}
		if first != 1 {
			t.Fatalf("first n = %d, want 1", first)
		}

		second, err := env.mgr.IngestLaunchLog(
			context.Background(), env.launchLogPath, "rune", "v0",
		)
		if err != nil {
			t.Fatalf("second IngestLaunchLog: %v", err)
		}
		if second != 0 {
			t.Fatalf("second n = %d, want 0", second)
		}
		assertReportCount(t, env.reportsDir, 1)
	})

	t.Run("only new fatals after the offset are emitted", func(t *testing.T) {
		env := newLaunchLogEnv(t)
		writeFile(t, env.launchLogPath, stackOverflowFatal)
		if _, err := env.mgr.IngestLaunchLog(
			context.Background(), env.launchLogPath, "rune", "v0",
		); err != nil {
			t.Fatalf("first IngestLaunchLog: %v", err)
		}

		// The first ingestion truncates the log, so appending to
		// it after the fact simulates a brand-new fatal occurring
		// later in the session.
		assertFileEmpty(t, env.launchLogPath)
		appendFile(t, env.launchLogPath, "interleaved noise\n"+concurrentMapFatal)
		n, err := env.mgr.IngestLaunchLog(
			context.Background(), env.launchLogPath, "rune", "v0",
		)
		if err != nil {
			t.Fatalf("second IngestLaunchLog: %v", err)
		}
		if n != 1 {
			t.Fatalf("n = %d, want 1", n)
		}
		assertReportCount(t, env.reportsDir, 2)
		assertFileEmpty(t, env.launchLogPath)
	})

	t.Run("truncated log re-scans from the beginning", func(t *testing.T) {
		env := newLaunchLogEnv(t)
		writeFile(t, env.launchLogPath, stackOverflowFatal+concurrentMapFatal)
		if _, err := env.mgr.IngestLaunchLog(
			context.Background(), env.launchLogPath, "rune", "v0",
		); err != nil {
			t.Fatalf("first IngestLaunchLog: %v", err)
		}

		// Truncate-and-replace simulates the wrapper rotating the file.
		writeFile(t, env.launchLogPath, concurrentMapFatal)
		n, err := env.mgr.IngestLaunchLog(
			context.Background(), env.launchLogPath, "rune", "v0",
		)
		if err != nil {
			t.Fatalf("second IngestLaunchLog: %v", err)
		}
		if n != 1 {
			t.Fatalf("n = %d, want 1", n)
		}
		assertReportCount(t, env.reportsDir, 3)
	})
}

type launchLogEnv struct {
	reportsDir    string
	launchLogPath string
	mgr           *Manager
}

func newLaunchLogEnv(t *testing.T) *launchLogEnv {
	t.Helper()
	dir := t.TempDir()
	env := &launchLogEnv{
		reportsDir:    filepath.Join(dir, "reports"),
		launchLogPath: DefaultLaunchLogPath(dir),
	}
	if err := os.MkdirAll(env.reportsDir, 0o777); err != nil {
		t.Fatal(err)
	}
	env.mgr = NewManager(env.reportsDir, "",
		storagestub.NewInMemoryService(), nil)
	return env
}

func appendFile(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
}

func assertReportCount(t *testing.T, dir string, want int) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := 0
	for _, e := range entries {
		if strings.Contains(e.Name(), "crash_report") {
			got++
		}
	}
	if got != want {
		t.Fatalf("crash report count = %d, want %d", got, want)
	}
}

// assertFileEmpty fails the test if path does not exist or has non-zero
// size. After a successful IngestLaunchLog the launch log is truncated
// so the next session starts with a clean slate.
func assertFileEmpty(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %q: %v", path, err)
	}
	if info.Size() != 0 {
		t.Fatalf("file %q size = %d, want 0", path, info.Size())
	}
}
