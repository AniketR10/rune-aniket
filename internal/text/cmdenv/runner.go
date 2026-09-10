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

package cmdenv

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// Runner runs a parsed shell line via mvdan.cc/sh/v3/interp,
// dispatching each non-builtin command through the workspace
// Executor so remote workspaces (ssh, in-memory) see the same alias
// semantics as local file:// ones.
type Runner struct {
	Executor schemeapi.Executor
	// EnvSource resolves the parent shell environment. May be nil.
	EnvSource Source
	Dir       string
	// Stdin/Stdout/Stderr are passed to interp; nil Stdout/Stderr is
	// treated as io.Discard.
	Stdin          io.Reader
	Stdout, Stderr io.Writer
}

// Run executes line and returns the variables the script assigned,
// filtered to plain strings and excluding interp bootstrap names so
// callers can publish them into a parent env. When parsed is nil it
// is recomputed from line.
func (r Runner) Run(
	ctx context.Context, line string, parsed *syntax.File,
) (map[string]string, error) {
	if parsed == nil {
		p, err := syntax.NewParser().Parse(strings.NewReader(line), "")
		if err != nil {
			return nil, fmt.Errorf("parse shell line %q: %w", line, err)
		}
		parsed = p
	}

	stdout := r.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := r.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	opts := []interp.RunnerOption{
		interp.Env(parentEnv(r.EnvSource)),
		interp.StdIO(r.Stdin, stdout, stderr),
		interp.ExecHandlers(func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
			return workspaceExecHandler(r.Executor)
		}),
	}
	if r.Dir != "" {
		opts = append(opts, interp.Dir(r.Dir))
	}
	runner, err := interp.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("init shell runner: %w", err)
	}
	if runErr := runner.Run(ctx, parsed); runErr != nil {
		if errors.Is(runErr, context.DeadlineExceeded) ||
			errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, errors.New(
				"command was taking too long and so it was canceled")
		}
		stderrText := ""
		if buf, ok := stderr.(fmt.Stringer); ok {
			stderrText = strings.TrimSpace(buf.String())
		}
		if exit, ok := runErr.(interp.ExitStatus); ok {
			return nil, fmt.Errorf("%s: exit status %d: %s",
				strings.TrimSpace(line), uint8(exit), stderrText)
		}
		return nil, fmt.Errorf("%s: %v: %s",
			strings.TrimSpace(line), runErr, stderrText)
	}
	return collectShellVars(runner), nil
}

// FuncEnviron's no-op Each means interp.Runner.Vars only surfaces
// variables that the script itself sets — what callers capturing
// assignments rely on.
func parentEnv(src Source) expand.Environ {
	return expand.FuncEnviron(func(name string) string {
		if src != nil {
			if v, ok := src(name); ok {
				return v
			}
		}
		return ""
	})
}

func workspaceExecHandler(exe schemeapi.Executor) interp.ExecHandlerFunc {
	return func(ctx context.Context, args []string) error {
		if len(args) == 0 {
			return nil
		}
		hc := interp.HandlerCtx(ctx)
		watchCh := make(chan error, 1)
		cmd := workspaceapi.Cmd{
			Path:    args[0],
			Args:    args[1:],
			Stdin:   hc.Stdin,
			Stdout:  hc.Stdout,
			Stderr:  hc.Stderr,
			Watcher: workspaceapi.ChanProcessWatcher(watchCh),
		}
		if _, startErr := exe.StartCommand(ctx, cmd); startErr != nil {
			return interp.ExitStatus(127)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case watchErr := <-watchCh:
			if watchErr != nil {
				return interp.ExitStatus(1)
			}
			return nil
		}
	}
}

func collectShellVars(r *interp.Runner) map[string]string {
	out := make(map[string]string, len(r.Vars))
	for name, vr := range r.Vars {
		if !vr.IsSet() || vr.Kind != expand.String {
			continue
		}
		if shellBootstrapVar(name) {
			continue
		}
		out[name] = vr.Str
	}
	return out
}

func shellBootstrapVar(name string) bool {
	switch name {
	case "HOME", "PWD", "UID", "EUID", "GID",
		"IFS", "OPTIND", "OLDPWD":
		return true
	}
	return false
}
