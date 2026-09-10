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
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
	"time"
)

// headscaleImage is the coordination server Rune's network is designed
// around. Pinned so a test failure means our code changed, not the
// upstream image.
const headscaleImage = "headscale/headscale:v0.26.1"

//go:embed headscale.yaml.tmpl
var headscaleConfigTemplate string

// SkipIfNoDocker skips the test when docker is not usable on this host.
func SkipIfNoDocker(t *testing.T) {
	t.Helper()
	if os.Getenv("CI") == "true" {
		t.SkipNow()
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skipf("docker not available: %v", err)
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skipf("docker daemon not reachable: %v", err)
	}
}

// StartHeadscale runs a Headscale coordination server in docker and
// returns a control plane with a pre-authorization key already minted,
// so nodes join without an interactive login.
//
// Ports are published one-to-one rather than letting docker choose:
// Headscale advertises its own listen and STUN ports to clients, so a
// remapped port would hand nodes an address that does not exist on the
// host.
func StartHeadscale(t *testing.T) ControlPlane {
	t.Helper()
	EnsureHeadscaleImage(t)

	httpPort := freeTCPPort(t)
	stunPort := freeUDPPort(t)
	serverURL := fmt.Sprintf("http://127.0.0.1:%d", httpPort)
	configPath := writeHeadscaleConfig(t, serverURL, httpPort, stunPort)

	out, err := exec.Command("docker", "run", "-d", "--rm",
		"-p", fmt.Sprintf("%d:%d", httpPort, httpPort),
		"-p", fmt.Sprintf("%d:%d/udp", stunPort, stunPort),
		"-v", configPath+":/etc/headscale/config.yaml:ro",
		headscaleImage, "serve").Output()
	if err != nil {
		t.Fatalf("docker run headscale: %v: %s", err, exitStderr(err))
	}
	id := strings.TrimSpace(string(out))
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", id).Run() })

	if err := waitHeadscaleHealthy(serverURL, 60*time.Second); err != nil {
		dumpDockerLogs(t, id)
		t.Fatalf("headscale did not become healthy on %s: %v", serverURL, err)
	}

	return ControlPlane{
		URL:     serverURL,
		AuthKey: headscaleAuthKey(t, id, "rune-e2e"),
	}
}

// EnsureHeadscaleImage pulls the coordination server image if it is not
// cached yet.
func EnsureHeadscaleImage(t *testing.T) {
	t.Helper()
	if err := exec.Command("docker", "image", "inspect", headscaleImage).Run(); err == nil {
		return
	}
	t.Logf("pulling %s (one-time per-machine cost)", headscaleImage)
	cmd := exec.Command("docker", "pull", headscaleImage)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("docker pull %s: %v", headscaleImage, err)
	}
}

// headscaleAuthKey creates a user and returns a reusable
// pre-authorization key for it. Reusable because every node in a test
// registers with the same key, which is also what makes them peers
// owned by one account.
func headscaleAuthKey(t *testing.T, id, user string) string {
	t.Helper()

	if out, err := exec.Command("docker", "exec", id,
		"headscale", "users", "create", user).CombinedOutput(); err != nil {
		t.Fatalf("headscale users create: %v: %s", err, string(out))
	}

	out, err := exec.Command("docker", "exec", id,
		"headscale", "users", "list", "--output", "json").Output()
	if err != nil {
		t.Fatalf("headscale users list: %v: %s", err, exitStderr(err))
	}
	var users []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out, &users); err != nil {
		t.Fatalf("parse headscale users: %v: %s", err, string(out))
	}
	userID := 0
	for _, u := range users {
		if u.Name == user {
			userID = u.ID
		}
	}
	if userID == 0 {
		t.Fatalf("headscale user %q not found in %s", user, string(out))
	}

	key, err := exec.Command("docker", "exec", id,
		"headscale", "preauthkeys", "create",
		"--user", fmt.Sprint(userID),
		"--reusable", "--expiration", "24h").Output()
	if err != nil {
		t.Fatalf("headscale preauthkeys create: %v: %s", err, exitStderr(err))
	}
	// The key is the last line; earlier lines are log output.
	lines := strings.Fields(strings.TrimSpace(string(key)))
	if len(lines) == 0 {
		t.Fatalf("headscale returned an empty pre-auth key")
	}
	return lines[len(lines)-1]
}

func writeHeadscaleConfig(
	t *testing.T, serverURL string, httpPort, stunPort int,
) string {
	t.Helper()

	tmpl, err := template.New("headscale").Parse(headscaleConfigTemplate)
	if err != nil {
		t.Fatalf("parse headscale config template: %v", err)
	}
	// Written outside t.TempDir(): docker on macOS only bind-mounts
	// paths the VM shares, and /tmp is shared while the per-test
	// directory under /var/folders is not.
	f, err := os.CreateTemp("/tmp", "headscale-*.yaml")
	if err != nil {
		t.Fatalf("create headscale config: %v", err)
	}
	defer f.Close()
	t.Cleanup(func() { _ = os.Remove(f.Name()) })

	if err := tmpl.Execute(f, map[string]any{
		"ServerURL": serverURL,
		"HTTPPort":  httpPort,
		"STUNPort":  stunPort,
	}); err != nil {
		t.Fatalf("render headscale config: %v", err)
	}
	if err := os.Chmod(f.Name(), 0o644); err != nil {
		t.Fatalf("chmod headscale config: %v", err)
	}
	return f.Name()
}

func waitHeadscaleHealthy(serverURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := http.Get(serverURL + "/health") //nolint:noctx
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("health returned %s", resp.Status)
		} else {
			lastErr = err
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("timed out: %w", lastErr)
}

// freeTCPPort asks the kernel for an unused port. The listener is
// closed before the port is handed out, so this races any other process
// binding in between; in practice the window is small and the harness
// fails loudly rather than silently misbehaving.
func freeTCPPort(t *testing.T) int {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve tcp port: %v", err)
	}
	defer lis.Close()
	return lis.Addr().(*net.TCPAddr).Port
}

func freeUDPPort(t *testing.T) int {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve udp port: %v", err)
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).Port
}

func dumpDockerLogs(t *testing.T, id string) {
	t.Helper()
	out, err := exec.Command("docker", "logs", id).CombinedOutput()
	if err != nil {
		t.Logf("docker logs %s: %v", id, err)
		return
	}
	t.Logf("headscale %s logs:\n%s", id, string(out))
}

func exitStderr(err error) string {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(exitErr.Stderr)
	}
	return ""
}

// repoRoot walks upwards from this file until it finds the go.mod that
// anchors the module.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}
