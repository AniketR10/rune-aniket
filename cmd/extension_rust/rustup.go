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

package main

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/debug"
)

// userBinaries are the rustup-managed binaries symlinked from
// $CARGO_HOME/bin into the package bin dir ($RUNE_DATADIR/bin), which is
// Rune's guaranteed on-PATH location. rust-analyzer and lldb-dap are
// bundled separately and resolved by path, so they are not symlinked.
var userBinaries = []string{"cargo", "rustc", "rustfmt", "cargo-clippy"}

// detectRustProject reports whether the workspace root looks like a Rust
// project: a Cargo.toml manifest or any top-level .rs source file.
func detectRustProject(_ context.Context, fs workspaceapi.FileSystem) bool {
	if info, err := fs.Stat("Cargo.toml"); err == nil && info != nil && !info.IsDir() {
		return true
	}
	if entries, err := fs.ReadDir("."); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".rs") {
				return true
			}
		}
	}
	return false
}

// toolchainInstalled reports whether a rustup toolchain already exists
// under $RUSTUP_HOME, so first-run bootstrap is skipped on subsequent
// launches. rustup stores toolchains in $RUSTUP_HOME/toolchains.
func toolchainInstalled(fs workspaceapi.FileSystem, rustupHome string) bool {
	if rustupHome == "" {
		return false
	}
	dir := path.Join(rustupHome, "toolchains")
	entries, err := fs.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			return true
		}
	}
	return false
}

// bootstrapRustup installs the stable toolchain and the components the
// language server needs when none is present, reporting step-based
// progress through a single notification. It is best-effort: a failure
// is surfaced as a warning and the caller proceeds to LSP bring-up,
// since rust-analyzer is still useful against an already-present
// toolchain. After install it refreshes the user-facing symlinks.
func bootstrapRustup(
	ctx context.Context,
	rustupBin string,
	exec workspaceapi.Executor,
	notify browserapi.Notifications,
	fs workspaceapi.FileSystem,
	rustupHome, cargoHome, dataDir, dir string,
) error {
	if toolchainInstalled(fs, rustupHome) {
		return refreshSymlinks(ctx, exec, fs, cargoHome, dataDir, dir)
	}

	notifID, _ := notify.Notify(browserapi.LevelInfo, "Preparing Rust toolchain")

	total := int64(3)
	_ = notify.UpdateNotificationProgress(notifID, "Installing Rust toolchain", 1, total)
	if err := runRustup(ctx, rustupBin, exec, dir,
		"toolchain", "install", "stable", "--profile", "minimal"); err != nil {
		return fmt.Errorf("install toolchain: %w", err)
	}

	_ = notify.UpdateNotificationProgress(notifID, "Adding Rust components", 2, total)
	if err := runRustup(ctx, rustupBin, exec, dir,
		"component", "add", "rust-src", "clippy", "rustfmt"); err != nil {
		return fmt.Errorf("add components: %w", err)
	}

	if err := refreshSymlinks(ctx, exec, fs, cargoHome, dataDir, dir); err != nil {
		return err
	}
	return notify.UpdateNotificationProgress(notifID, "Rust toolchain ready", total, total)
}

// refreshSymlinks links the user-facing rustup binaries from
// $CARGO_HOME/bin into $RUNE_DATADIR/bin so they resolve on Rune's
// guaranteed PATH. It is idempotent: existing links are replaced
// (ln -sf) so toolchain or default-channel changes repoint them.
func refreshSymlinks(
	ctx context.Context,
	exec workspaceapi.Executor,
	fs workspaceapi.FileSystem,
	cargoHome, dataDir, dir string,
) error {
	if cargoHome == "" || dataDir == "" {
		return nil
	}
	binDir := path.Join(dataDir, "bin")
	if err := fs.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("create bin dir: %w", err)
	}
	srcBin := path.Join(cargoHome, "bin")
	for _, name := range userBinaries {
		src := path.Join(srcBin, name)
		dst := path.Join(binDir, name)
		if err := runCommand(ctx, exec, dir, "ln", "-sf", src, dst); err != nil {
			return fmt.Errorf("symlink %s: %w", name, err)
		}
	}
	return nil
}

// runRustup runs `rustup <args>` through the workspace executor,
// draining stderr so the process never blocks on a full pipe. It relies
// on the inherited RUSTUP_HOME/CARGO_HOME from gui.env rather than
// setting them per-command.
func runRustup(
	ctx context.Context,
	rustupBin string,
	exec workspaceapi.Executor,
	dir string,
	args ...string,
) error {
	bin := rustupBin
	if bin == "" {
		bin = "rustup"
	}
	return runCommand(ctx, exec, dir, bin, args...)
}

// runCommand starts bin with args through the executor and waits for it
// to exit, draining stderr in a panic-captured goroutine.
func runCommand(
	ctx context.Context,
	exec workspaceapi.Executor,
	dir string,
	bin string,
	args ...string,
) error {
	stderrR, stderrW := io.Pipe()
	ch := make(chan error, 1)
	cmd := workspaceapi.Cmd{
		Path:    bin,
		Dir:     dir,
		Args:    args,
		Stderr:  stderrW,
		Watcher: workspaceapi.ChanProcessWatcher(ch),
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go debug.CapturePanicReport(func() {
		defer wg.Done()
		_, _ = io.Copy(io.Discard, stderrR)
	})

	if _, err := exec.Start(ctx, cmd); err != nil {
		_ = stderrW.Close()
		wg.Wait()
		return fmt.Errorf("start %s %s: %w", bin, strings.Join(args, " "), err)
	}

	var runErr error
	select {
	case runErr = <-ch:
	case <-ctx.Done():
		runErr = ctx.Err()
	}
	_ = stderrW.Close()
	wg.Wait()
	if runErr != nil {
		return fmt.Errorf("%s %s: %w", bin, strings.Join(args, " "), runErr)
	}
	return nil
}
