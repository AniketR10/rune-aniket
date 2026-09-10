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

package runetest

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"unstable.build/rune/internal/debug"
)

// StartInstance launches a second Rune instance as its own process,
// joined to control under hostname and serving its workspaces. It
// returns the instance's data directory once the instance reports
// itself reachable.
func StartInstance(t *testing.T, control ControlPlane, hostname string) string {
	t.Helper()

	// Not t.TempDir(): the instance holds its network identity
	// here and must be able to write it for the whole test.
	dataDir := t.TempDir()
	cmd := exec.Command(instanceBinary(t),
		"-hostname", hostname,
		"-control-url", control.URL,
		"-auth-key", control.AuthKey,
		"-datadir", dataDir,
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start rune instance %q: %v", hostname, err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	go debug.CapturePanicReport(func() {
		forwardOutput(t, hostname, stderr)
	})
	waitInstanceReady(t, hostname, stdout)
	return dataDir
}

// waitInstanceReady blocks until the instance prints its ready line, so
// tests never race the peer's registration with the coordination
// server.
func waitInstanceReady(t *testing.T, hostname string, stdout io.Reader) {
	t.Helper()

	ready := make(chan struct{})
	go debug.CapturePanicReport(func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, readyLine) {
				close(ready)
				continue
			}
			t.Logf("%s: %s", hostname, line)
		}
	})

	select {
	case <-ready:
	case <-time.After(nodeJoinTimeout):
		t.Fatalf("rune instance %q never became ready", hostname)
	}
}

func forwardOutput(t *testing.T, hostname string, r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		t.Logf("%s: %s", hostname, scanner.Text())
	}
}

// readyLine mirrors the sentinel runenetsvc prints once it is
// reachable by peers.
const readyLine = "runenetsvc: ready"

var (
	instanceBinaryOnce sync.Once
	instanceBinaryPath string
	instanceBinaryErr  error
)

// instanceBinary builds the peer instance once per test binary and
// caches it, so a package with several e2e tests pays for one compile.
func instanceBinary(t *testing.T) string {
	t.Helper()

	instanceBinaryOnce.Do(func() {
		dir, err := os.MkdirTemp("", "runenetsvc")
		if err != nil {
			instanceBinaryErr = err
			return
		}
		instanceBinaryPath = filepath.Join(dir, "runenetsvc")
		cmd := exec.Command("go", "build",
			"-o", instanceBinaryPath,
			"unstable.build/rune/internal/workspace/workspacerune/test/cmd/runenetsvc")
		cmd.Dir = repoRoot(t)
		if out, err := cmd.CombinedOutput(); err != nil {
			instanceBinaryErr = fmt.Errorf("go build runenetsvc: %v: %s", err, string(out))
		}
	})
	if instanceBinaryErr != nil {
		t.Fatalf("build rune instance: %v", instanceBinaryErr)
	}
	return instanceBinaryPath
}
