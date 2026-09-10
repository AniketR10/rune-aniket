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

package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// runCommand executes a command-type hook through the workspace
// Executor. It writes the JSON envelope to the child's stdin,
// captures stdout/stderr, and converts exit codes to a hookResult per
// the wire protocol:
//
//	exit 0  → proceed; stdout JSON parsed if present, otherwise (for
//	          UserPromptSubmit/SessionStart only) treated as plain
//	          text additionalContext.
//	exit 2  → block; stderr text becomes the block reason.
//	other   → log warning, proceed.
func (r *Runner) runCommand(
	ctx context.Context, h Hook, env []string, payload Payload, payloadJSON []byte,
) hookResult {
	if r.executor == nil {
		return hookResult{warn: fmt.Errorf("hook command: no executor configured")}
	}

	shell := h.Shell
	if shell == "" {
		shell = "bash"
	}

	cctx, cancel := context.WithTimeout(ctx, h.Timeout)
	defer cancel()

	var stdout, stderr bytes.Buffer
	watcher := newHookWatcher()
	cmd := workspaceapi.Cmd{
		Path:    shell,
		Args:    []string{"-c", h.Command},
		Dir:     r.projectDir,
		Env:     env,
		Stdin:   bytes.NewReader(payloadJSON),
		Stdout:  &stdout,
		Stderr:  &stderr,
		Watcher: watcher,
	}

	if _, err := r.executor.Start(cctx, cmd); err != nil {
		return hookResult{warn: fmt.Errorf("hook exec: %w", err)}
	}

	// Always wait for the watcher to close so the executor's
	// stdout/stderr copying goroutines have finished writing into
	// our buffers before we read them. Cancellation/timeout fires
	// via cctx, which the executor honors by killing the process.
	procErr := <-watcher.WatchProcess()

	res := hookResult{stdout: stdout.String(), stderr: stderr.String()}
	if cctx.Err() == context.DeadlineExceeded {
		res.warn = fmt.Errorf("hook timed out after %s", h.Timeout)
		return res
	}

	exitCode := exitCodeFromError(procErr)
	if exitCode < 0 {
		res.warn = fmt.Errorf("hook exec: %w", procErr)
		return res
	}

	switch exitCode {
	case 0:
		// Try to parse stdout as JSON; fall back to plain-text
		// additionalContext for events that allow it.
		out, ok := parseHookOutput(stdout.Bytes())
		if ok {
			res.output = out
			return res
		}
		text := strings.TrimSpace(stdout.String())
		if text != "" && allowsPlainText(payload.HookEventName) {
			res.output = HookOutput{HookSpecificOutput: &HookSpecificOutput{
				HookEventName:     string(payload.HookEventName),
				AdditionalContext: text,
			}}
		}
		return res
	case 2:
		// Block: stderr is the reason.
		reason := strings.TrimSpace(stderr.String())
		res.output = HookOutput{Decision: "block", Reason: reason}
		return res
	default:
		res.warn = fmt.Errorf("hook exited with code %d: %s",
			exitCode, strings.TrimSpace(stderr.String()))
		return res
	}
}

// hookWatcher implements workspaceapi.ProcessWatcher backed by a
// single-buffered channel.
type hookWatcher struct{ ch chan error }

func newHookWatcher() *hookWatcher { return &hookWatcher{ch: make(chan error, 1)} }

func (w *hookWatcher) WatchProcess() chan error { return w.ch }

// exitCodeFromError extracts an exit code from a process error.
// Returns 0 for nil; -1 when the code cannot be determined (e.g. the
// executor returned a non-exit error).
func exitCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	type exitCoder interface{ ExitCode() int }
	if ec, ok := err.(exitCoder); ok {
		return ec.ExitCode()
	}
	return -1
}

func parseHookOutput(b []byte) (HookOutput, bool) {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || b[0] != '{' {
		return HookOutput{}, false
	}
	var out HookOutput
	if err := json.Unmarshal(b, &out); err != nil {
		return HookOutput{}, false
	}
	return out, true
}

func allowsPlainText(ev Event) bool {
	return ev == EventUserPromptSubmit || ev == EventSessionStart
}
