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
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// runnable is rust-analyzer's experimental/runnables result item. It is a
// tagged union on kind ("cargo" | "shell"); only the matching args block is
// populated.
type runnable struct {
	Label string           `json:"label"`
	Kind  string           `json:"kind"`
	Args  runnableArgsJSON `json:"args"`
}

// runnableArgsJSON is the superset of CargoRunnableArgs and ShellRunnableArgs;
// which fields are set depends on the runnable kind.
type runnableArgsJSON struct {
	// Common.
	Environment map[string]string `json:"environment"`
	Cwd         string            `json:"cwd"`
	// Cargo.
	WorkspaceRoot  string   `json:"workspaceRoot"`
	CargoArgs      []string `json:"cargoArgs"`
	ExecutableArgs []string `json:"executableArgs"`
	OverrideCargo  string   `json:"overrideCargo"`
	// Shell.
	Program string   `json:"program"`
	Args    []string `json:"args"`
}

// runCmd fetches the runnables at the cursor, lets the user pick one, and
// executes it, showing the captured output in a floating viewer. It mirrors
// the VS Code client's command reconstruction (createTaskFromRunnable) so
// the executed cargo/shell command matches what rust-analyzer intends.
type runCmd struct {
	lsp    semanticapi.LSP
	exec   workspaceapi.Executor
	wm     browserapi.WindowManager
	notify browserapi.Notifications
	cwd    string
}

var _ textapi.CommandHandler = (*runCmd)(nil)

func (c *runCmd) HandleCommand(ctx context.Context, cmd textapi.Command) error {
	if err := requireFile(cmd); err != nil {
		return err
	}
	pos := posParams(cmd).Position
	params := struct {
		TextDocument semanticapi.TextDocumentIdentifier `json:"textDocument"`
		Position     *semanticapi.Position              `json:"position,omitempty"`
	}{TextDocument: docParams(cmd), Position: &pos}
	runnables, err := execRequest[[]runnable](ctx, c.lsp, "experimental/runnables", params)
	if err != nil {
		return err
	}
	if len(runnables) == 0 {
		_, _ = c.notify.Notify(browserapi.LevelInfo, "No runnables at the cursor")
		return nil
	}
	if len(runnables) == 1 {
		return c.run(ctx, runnables[0])
	}
	labels := make([]string, len(runnables))
	for i, r := range runnables {
		labels[i] = r.Label
	}
	ch := make(chan int, 1)
	if _, err := c.wm.Floating(newListPicker(labels, ch), browserapi.FloatingConfig{
		Alignment: component.AlignmentCentered,
	}); err != nil {
		return fmt.Errorf("show runnables: %w", err)
	}
	select {
	case idx := <-ch:
		if idx < 0 || idx >= len(runnables) {
			return nil
		}
		return c.run(ctx, runnables[idx])
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *runCmd) run(ctx context.Context, r runnable) error {
	spec, err := runnableCommand(r, c.cwd)
	if err != nil {
		return err
	}
	var stdout, stderr bytes.Buffer
	done := make(chan error, 1)
	proc := workspaceapi.Cmd{
		Path:    spec.path,
		Args:    spec.args,
		Dir:     spec.dir,
		Env:     spec.env,
		Stdout:  &stdout,
		Stderr:  &stderr,
		Watcher: workspaceapi.ChanProcessWatcher(done),
	}
	if _, err := c.exec.Start(ctx, proc); err != nil {
		return fmt.Errorf("run %s: %w", r.Label, err)
	}
	var runErr error
	select {
	case runErr = <-done:
	case <-ctx.Done():
		return ctx.Err()
	}
	out := strings.TrimRight(stdout.String()+stderr.String(), "\n")
	header := fmt.Sprintf("$ %s\n\n", strings.Join(append([]string{spec.path}, spec.args...), " "))
	if runErr != nil {
		out = strings.TrimSpace(out + "\n\n" + runErr.Error())
	}
	if _, err := c.wm.Floating(newTextView(header+out), browserapi.FloatingConfig{
		Alignment: component.AlignmentCentered,
	}); err != nil {
		return fmt.Errorf("show output: %w", err)
	}
	return nil
}

func (c *runCmd) Complete(_ context.Context, _ string, _ []string) (
	iterator.Iterator[string], error,
) {
	return iterator.Empty[string](), nil
}

// commandSpec is a fully resolved process invocation.
type commandSpec struct {
	path string
	args []string
	dir  string
	env  []string
}

// runnableCommand reconstructs the process invocation for a runnable,
// following the same rules as the VS Code client's createTaskFromRunnable:
// cargo runnables join cargoArgs with executableArgs after a `--` separator
// and honor overrideCargo; shell runnables run program+args directly.
func runnableCommand(r runnable, fallbackDir string) (commandSpec, error) {
	switch r.Kind {
	case "cargo":
		program := "cargo"
		args := cargoArgs(r.Args)
		if o := strings.TrimSpace(r.Args.OverrideCargo); o != "" {
			parts := strings.Fields(o)
			program = parts[0]
			args = append(append([]string(nil), parts[1:]...), args...)
		}
		dir := r.Args.WorkspaceRoot
		if dir == "" {
			dir = fallbackDir
		}
		return commandSpec{
			path: program,
			args: args,
			dir:  dir,
			env:  runnableEnv(r.Args.Environment),
		}, nil
	case "shell":
		if r.Args.Program == "" {
			return commandSpec{}, fmt.Errorf("shell runnable %q has no program", r.Label)
		}
		return commandSpec{
			path: r.Args.Program,
			args: append([]string(nil), r.Args.Args...),
			dir:  r.Args.Cwd,
			env:  runnableEnv(r.Args.Environment),
		}, nil
	default:
		return commandSpec{}, fmt.Errorf("unsupported runnable kind %q", r.Kind)
	}
}

// cargoArgs joins the cargo arguments with the executable arguments,
// separating them with `--` when the runnable passes any.
func cargoArgs(a runnableArgsJSON) []string {
	args := append([]string(nil), a.CargoArgs...)
	if len(a.ExecutableArgs) > 0 {
		args = append(args, "--")
		args = append(args, a.ExecutableArgs...)
	}
	return args
}

// runnableEnv builds the process environment for a runnable, layering (in
// increasing precedence) the inherited process environment, RUST_BACKTRACE,
// and the runnable's own environment, matching the VS Code client's
// prepareEnv. Returning nil (no runnable env) would make os/exec inherit the
// parent environment, but RUST_BACKTRACE must always be set, so the base is
// materialized explicitly.
func runnableEnv(env map[string]string) []string {
	base := os.Environ()
	out := make([]string, 0, len(base)+len(env)+1)
	out = append(out, base...)
	out = append(out, "RUST_BACKTRACE=short")
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}
