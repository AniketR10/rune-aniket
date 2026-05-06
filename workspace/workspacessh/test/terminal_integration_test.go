// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY.

package workspacetest

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// TestIntegrationTerminalShell exercises the protocol contract used by
// vte.Component when the user opens a terminal in an SSH workspace:
// the IDE sends an empty Cmd.Path and empty Cmd.Args, and the remote
// fileScheme is responsible for turning that into the user's login
// shell on the *remote* host.
//
// Regression scenario: a remote host advertises a login shell at a
// path that exists on a typical interactive shell's view of the
// filesystem but not on disk in the way fork/exec needs (e.g.
// $SHELL=/usr/bin/bash on a host that only ships /bin/bash). With
// the buggy resolveLoginShell, the remote fileScheme would forward
// the bogus path straight into exec.CommandContext and surface the
// confusing error "fork/exec /usr/bin/bash: no such file or
// directory" all the way back through the gRPC channel to the IDE.
//
// We reproduce that here by installing a tiny login-shell wrapper on
// the container that exports SHELL=/usr/bin/bash before delegating
// to /bin/sh, then chsh-ing the test user to use it. SSH then sets
// SHELL=/usr/bin/bash for the runesvc process, exactly mirroring the
// shape of the bug. The test passes only if the executor falls back
// to a real shell on disk instead of forwarding the broken path.
func TestIntegrationTerminalShell(t *testing.T) {
	SkipIfNoDocker(t)
	EnsureImage(t)

	c := StartContainer(t, SSHDScenario{
		PublicKeyFile:     "/id_ed25519.pub",
		InstallRuneBinary: true,
	})

	// Stage a wrapper login shell that simulates the bug: it
	// advertises a non-existent SHELL path before exec'ing the
	// real shell. usermod the test user to use it so every
	// subsequent SSH session — including the one that launches
	// runesvc — inherits the broken $SHELL.
	const wrapperPath = "/usr/local/bin/rune-test-loginshell"
	const bogusShell = "/usr/bin/bash" // not present on this image
	installLoginShellWrapper(t, c.ID, wrapperPath, bogusShell)
	chshUser(t, c.ID, "test", wrapperPath)

	keyPath := PrivateKeyPath(t, "id_ed25519")

	cfgs := map[string]config.Config{
		"openssh_proc_remote": config.MapConfig(map[string]any{
			"command": "ssh -o StrictHostKeyChecking=no -i " + keyPath +
				" %h -p %p",
			"timeout": "20s",
		}),
		"go_stdlib_remote": config.MapConfig(map[string]any{
			"private_keys": []any{keyPath},
			"timeout":      "20s",
			"insecure":     true,
		}),
	}

	for desc, cfg := range cfgs {
		t.Run(desc, func(t *testing.T) {
			t.Run("empty Path resolves to a remote shell that exists", func(t *testing.T) {
				s := newSchemeIntegration(t, c.HostPort, cfg)

				// Empty Path AND empty Args: the protocol contract
				// vte.Component sends when the user hasn't
				// configured terminal.shell. We use `-c "..."` via
				// non-empty Args only so the shell exits cleanly;
				// the resolution we care about (Path) is still left
				// to the executor.
				var stdout bytes.Buffer
				ch := make(chan error, 1)
				cmd := workspaceapi.Cmd{
					// Path intentionally empty.
					Args:    []string{"-c", "echo terminal-ok"},
					Stdout:  &stdout,
					Watcher: workspaceapi.ChanProcessWatcher(ch),
				}

				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				_, err := s.StartCommand(ctx, cmd)
				require.NoError(t, err,
					"empty-Path StartCommand must succeed: that's "+
						"the protocol contract vte.Component relies "+
						"on for terminal open over ssh")

				select {
				case err := <-ch:
					require.NoError(t, err,
						"shell launched by empty-Path Cmd must "+
							"exit cleanly; got %v, stdout=%q",
						err, stdout.String())
				case <-ctx.Done():
					t.Fatalf("timed out waiting for shell to exit; "+
						"stdout so far: %q", stdout.String())
				}

				assert.Equal(t, "terminal-ok\n", stdout.String(),
					"shell resolved on the remote should run our "+
						"-c command; if this is empty the executor "+
						"likely fell through to a non-existent "+
						"binary and silently swallowed the error")
			})

			t.Run("empty Path with empty Args boots a login shell", func(t *testing.T) {
				// Same as above but the *true* terminal-open
				// contract: empty Path AND empty Args. We feed the
				// shell an `exit` via stdin so it doesn't hang in
				// interactive mode (--login -i).
				s := newSchemeIntegration(t, c.HostPort, cfg)

				var stdout bytes.Buffer
				ch := make(chan error, 1)
				cmd := workspaceapi.Cmd{
					// Both empty: protocol contract for "use the
					// remote's login shell".
					Stdin:   strings.NewReader("echo login-shell-ok\nexit\n"),
					Stdout:  &stdout,
					Watcher: workspaceapi.ChanProcessWatcher(ch),
				}

				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				_, err := s.StartCommand(ctx, cmd)
				require.NoError(t, err)

				select {
				case err := <-ch:
					// The login shell may exit non-zero on EOF in
					// some configurations; what we care about is
					// that fork/exec succeeded and the shell ran
					// our command. Surface the error in the
					// failure message but don't fail solely on it.
					t.Logf("shell exit: %v", err)
				case <-ctx.Done():
					t.Fatalf("timed out waiting for shell; stdout=%q",
						stdout.String())
				}

				assert.Contains(t, stdout.String(), "login-shell-ok",
					"empty-Path empty-Args Cmd should boot a real "+
						"login shell on the remote and run echo from "+
						"stdin; got %q", stdout.String())
			})

			t.Run("pty + Setctty under empty-Path login shell", func(t *testing.T) {
				// Reproduces the vte.Component code path end-to-
				// end: allocate a remote pty, attach Slave to
				// stdin/stdout/stderr, set Setsid+Setctty, and let
				// the executor resolve the empty Path into the
				// remote's login shell.
				//
				// Regression: when an extension runner inadvertently
				// became the vte's executor, the pty's *remoteFile
				// was sent through a *local* fileScheme that didn't
				// recognize the fd, so it materialized as a plain
				// pipe. Setctty then failed with "inappropriate
				// ioctl for device" because the kernel was asked to
				// install a controlling tty on a non-tty fd.
				s := newSchemeIntegration(t, c.HostPort, cfg)

				ctx, cancel := context.WithTimeout(
					context.Background(), 30*time.Second)
				defer cancel()

				pty, err := s.NewPty(ctx)
				require.NoError(t, err, "remote NewPty must succeed")
				t.Cleanup(func() {
					_ = pty.Master.Close()
					_ = pty.Slave.Close()
				})

				// Drain master so the shell isn't blocked on a
				// full pipe; collect output until the shell exits.
				var output atomic.Value
				output.Store([]byte(nil))
				done := make(chan struct{})
				go func() {
					defer close(done)
					var buf bytes.Buffer
					_, _ = buf.ReadFrom(pty.Master)
					output.Store(buf.Bytes())
				}()

				ch := make(chan error, 1)
				cmd := workspaceapi.Cmd{
					// Empty Path + empty Args: same as vte sends.
					SysProcAttr: &syscall.SysProcAttr{
						Setsid:  true,
						Setctty: true,
					},
					Stdin:   pty.Slave,
					Stdout:  pty.Slave,
					Stderr:  pty.Slave,
					Watcher: workspaceapi.ChanProcessWatcher(ch),
				}

				_, err = s.StartCommand(ctx, cmd)
				require.NoError(t, err,
					"pty-attached empty-Path StartCommand must "+
						"succeed: this is exactly what "+
						"vte.Component sends when the user opens a "+
						"terminal in an SSH workspace")

				// Send a command + exit through the pty. The shell
				// runs `echo pty-ok` and exits; the read goroutine
				// drains output until EOF.
				_, err = pty.Master.Write([]byte("echo pty-ok\nexit\n"))
				require.NoError(t, err)

				select {
				case <-ch:
				case <-ctx.Done():
					t.Fatalf("timed out waiting for pty shell to exit")
				}
				_ = pty.Slave.Close()
				<-done

				out := string(output.Load().([]byte))
				assert.Contains(t, out, "pty-ok",
					"shell launched under a remote pty should run "+
						"our echo command; got %q", out)
			})
		})
	}
}

// installLoginShellWrapper writes a tiny `sh` wrapper inside the
// container that exports SHELL=<bogus> before exec'ing /bin/sh with
// the original args. We use it to simulate the production bug where
// the remote host advertises a $SHELL that fork/exec can't find.
//
// We use `docker cp` instead of piping into `docker exec` because
// `docker exec` does not attach stdin unless `-i` is set, and adding
// `-i` here doesn't reliably propagate EOF on macOS docker desktop —
// `cat > <path>` would hang.
func installLoginShellWrapper(t *testing.T, id, dst, bogus string) {
	t.Helper()
	script := "#!/bin/sh\n" +
		"export SHELL=" + bogus + "\n" +
		"exec /bin/sh \"$@\"\n"
	tmp := filepath.Join(t.TempDir(), "rune-test-loginshell")
	if err := os.WriteFile(tmp, []byte(script), 0o755); err != nil {
		t.Fatalf("write wrapper: %v", err)
	}
	if out, err := exec.Command("docker", "cp", tmp, id+":"+dst).
		CombinedOutput(); err != nil {
		t.Fatalf("docker cp wrapper: %v: %s", err, string(out))
	}
	if out, err := exec.Command("docker", "exec", id, "chmod", "0755", dst).
		CombinedOutput(); err != nil {
		t.Fatalf("chmod wrapper: %v: %s", err, string(out))
	}
}

// chshUser changes the login shell of the given container user.
// /etc/passwd has 7 colon-separated fields; the last is the login
// shell. We rewrite it with sed to avoid depending on chsh/usermod
// availability or PAM rules in the test image.
func chshUser(t *testing.T, id, user, shell string) {
	t.Helper()
	script := "set -e\n" +
		"sed -i 's|^\\(" + user + ":[^:]*:[^:]*:[^:]*:[^:]*:[^:]*:\\).*$|\\1" +
		shell + "|' /etc/passwd\n" +
		"getent passwd " + user + "\n"
	out, err := exec.Command("docker", "exec", id, "sh", "-c", script).
		CombinedOutput()
	if err != nil {
		t.Fatalf("chsh %s -> %s: %v: %s", user, shell, err, string(out))
	}
	t.Logf("chsh %s -> %s; passwd line: %s", user, shell,
		strings.TrimSpace(string(out)))
}
