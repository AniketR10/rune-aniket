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
