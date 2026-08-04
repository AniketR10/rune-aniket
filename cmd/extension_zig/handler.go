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
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/component/markdown"
)

// zigCommandName is the top-level REPL command exposed by this extension.
const zigCommandName = "zig"

// zigRoutes maps a `zig` subcommand to the zig argv prefix it expands
// to. The user-supplied trailing args are appended verbatim. The
// `reload` subcommand is handled in-process and is not present here.
var zigRoutes = map[string][]string{
	"build":   {"build"},
	"test":    {"test"},
	"run":     {"run"},
	"fmt":     {"fmt"},
	"version": {"version"},
}

// zigSubcommandNames is the deterministic completion order at depth 0.
var zigSubcommandNames = []string{
	"build", "fmt", "reload", "run", "test", "version",
}

var zigManual = textapi.CommandManual{
	Name:     zigCommandName,
	Summary:  "Drive the Zig toolchain and the zls language server.",
	Synopsis: "<command> [<args>]",
	Commands: []textapi.CommandManual{
		{Name: "build", Summary: "Build the project from build.zig.", Synopsis: "[<step>] [<args>]"},
		{Name: "test", Summary: "Run the project's tests.", Synopsis: "[<file>] [<args>]"},
		{Name: "run", Summary: "Build and run an executable.", Synopsis: "[<file>] [<args>]"},
		{Name: "fmt", Summary: "Format Zig sources in place.", Synopsis: "[<paths>]"},
		{Name: "version", Summary: "Show the zig compiler version."},
		{Name: "reload", Summary: "Reinitialize the language server."},
	},
}

// reloadFunc reinitializes the language server after an out-of-band
// change.
type reloadFunc func(ctx context.Context) error

// binResolver lazily locates the zig binary, so a toolchain installed
// after workspace startup is still found by later commands.
type binResolver func(ctx context.Context) string

type zigHandler struct {
	exec       workspaceapi.Executor
	notify     browserapi.Notifications
	cwd        string
	resolveBin binResolver
	reload     reloadFunc
}

var _ textapi.REPLHandler = (*zigHandler)(nil)

// newZigHandler builds the `zig` REPL command handler and its manual.
// reload may be nil, in which case the `zig reload` subcommand is a
// no-op.
func newZigHandler(
	exec workspaceapi.Executor,
	notify browserapi.Notifications,
	cwd string,
	resolveBin binResolver,
	reload reloadFunc,
) (textapi.CommandManual, textapi.REPLHandler) {
	return zigManual, &zigHandler{
		exec:       exec,
		notify:     notify,
		cwd:        cwd,
		resolveBin: resolveBin,
		reload:     reload,
	}
}

// HandleCommand routes the first arg to the matching zig subcommand, or
// reinitializes the language server for `reload`.
func (h *zigHandler) HandleCommand(
	ctx context.Context, cmd repl.Command, _ repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	if len(cmd.Args) == 0 {
		return markdownOutput(usageMarkdown(zigManual)), nil
	}
	sub := cmd.Args[0]
	switch sub {
	case "help":
		return h.Help(ctx, cmd.Args[1:])
	case "reload":
		if h.reload != nil {
			if err := h.reload(ctx); err != nil {
				return nil, err
			}
		}
		return markdownOutput("Zig language server reloaded."), nil
	}

	prefix, ok := zigRoutes[sub]
	if !ok {
		return nil, fmt.Errorf("unknown command: %s", sub)
	}
	args := append(append([]string{}, prefix...), cmd.Args[1:]...)
	out, err := h.runZigCapture(ctx, args...)
	if err != nil {
		return nil, err
	}
	return markdownOutput(fmt.Sprintf("```\n%s\n```", out)), nil
}

// Complete offers the `zig` subcommands at depth 0. Deeper positions
// defer to zig at runtime and return no completions.
func (h *zigHandler) Complete(
	_ context.Context, _ string, args []string,
) (iterator.Iterator[string], error) {
	if len(args) <= 1 {
		filter := ""
		if len(args) == 1 {
			filter = args[0]
		}
		return iterator.FromSlice(filterNames(zigSubcommandNames, filter)), nil
	}
	return iterator.FromSlice[string](nil), nil
}

// Help renders the manual for the command tree, descending into
// subcommands named in args.
func (h *zigHandler) Help(
	_ context.Context, args []string,
) (iterator.Iterator[component.Responsive], error) {
	man := zigManual
	for _, a := range args {
		sub, ok := findSubcommand(man, a)
		if !ok {
			break
		}
		man = sub
	}
	return markdownOutput(usageMarkdown(man)), nil
}

func (h *zigHandler) runZigCapture(
	ctx context.Context, args ...string,
) (string, error) {
	bin := h.resolveBin(ctx)
	if bin == "" {
		return "", fmt.Errorf("zig binary not found on the workspace host; " +
			"set extensions.zig.config.zig_path in your config")
	}
	var stdout, stderr bytes.Buffer
	ch := make(chan error, 1)
	cmd := workspaceapi.Cmd{
		Path:    bin,
		Args:    args,
		Dir:     h.cwd,
		Stdout:  &stdout,
		Stderr:  &stderr,
		Watcher: workspaceapi.ChanProcessWatcher(ch),
	}
	if _, err := h.exec.Start(ctx, cmd); err != nil {
		return "", fmt.Errorf("start zig %s: %w", strings.Join(args, " "), err)
	}
	var runErr error
	select {
	case runErr = <-ch:
	case <-ctx.Done():
		runErr = ctx.Err()
	}

	out := strings.TrimRight(stdout.String(), "\n")
	errOut := strings.TrimRight(stderr.String(), "\n")
	if runErr != nil {
		combined := strings.TrimSpace(out + "\n" + errOut)
		return "", fmt.Errorf("zig %s: %w\n%s", strings.Join(args, " "), runErr, combined)
	}
	if out == "" {
		out = errOut
	}
	return out, nil
}

// markdownOutput falls back to a plain responsive string when the
// markdown parser rejects the content.
func markdownOutput(content string) iterator.Iterator[component.Responsive] {
	md, err := markdown.New(content)
	if err != nil {
		r := component.NewResponsiveString(content, component.StringResponsiveConfig{})
		return iterator.FromSlice([]component.Responsive{r})
	}
	return iterator.FromSlice([]component.Responsive{md})
}

func filterNames(names []string, prefix string) []string {
	if prefix == "" {
		return names
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		if strings.HasPrefix(n, prefix) {
			out = append(out, n)
		}
	}
	return out
}

func findSubcommand(man textapi.CommandManual, name string) (textapi.CommandManual, bool) {
	for _, c := range man.Commands {
		if c.Name == name {
			return c, true
		}
	}
	return textapi.CommandManual{}, false
}

func usageMarkdown(man textapi.CommandManual) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## `%s`\n\n", man.Name)
	if man.Summary != "" {
		fmt.Fprintf(&b, "%s\n\n", man.Summary)
	}
	if man.Synopsis != "" {
		fmt.Fprintf(&b, "**Usage:** `%s %s`\n\n", man.Name, man.Synopsis)
	}
	if len(man.Commands) > 0 {
		b.WriteString("### Subcommands\n\n")
		for _, c := range man.Commands {
			invocation := c.Name
			if c.Synopsis != "" {
				invocation = fmt.Sprintf("%s %s", c.Name, c.Synopsis)
			}
			fmt.Fprintf(&b, "- `%s`\n  %s\n", invocation, c.Summary)
		}
		b.WriteByte('\n')
	}
	return b.String()
}
