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

// rustCommandName is the top-level REPL command exposed by this extension.
const rustCommandName = "rust"

// rustupRoutes maps a `rust` subcommand to the rustup argv prefix it
// expands to. The user-supplied trailing args are appended verbatim.
// The `reload` subcommand is handled in-process and is not present here.
var rustupRoutes = map[string][]string{
	"toolchain": {"toolchain"},
	"component": {"component"},
	"target":    {"target"},
	"update":    {"update"},
	"which":     {"which"},
	"show":      {"show"},
	"default":   {"default"},
	"override":  {"override"},
	"run":       {"run"},
	"doc":       {"doc"},
	"self":      {"self"},
	"check":     {"check"},
}

// rustSubcommandNames is the deterministic completion order at depth 0.
var rustSubcommandNames = []string{
	"check", "component", "default", "doc", "override", "reload", "run",
	"self", "show", "target", "toolchain", "update", "which",
}

var rustNestedSubcommands = map[string][]string{
	"toolchain": {"install", "uninstall", "link", "list"},
	"component": {"add", "remove", "list"},
	"target":    {"add", "remove", "list"},
	"self":      {"update", "uninstall", "upgrade-data"},
	"override":  {"list", "set", "unset"},
}

var rustManual = textapi.CommandManual{
	Name:     rustCommandName,
	Summary:  "Manage the Rust toolchain through rustup.",
	Synopsis: "<command> [<args>]",
	Commands: []textapi.CommandManual{
		{Name: "toolchain", Summary: "Install, list, or remove toolchains.", Synopsis: "<args>"},
		{Name: "component", Summary: "Add, remove, or list toolchain components.", Synopsis: "<args>"},
		{Name: "target", Summary: "Add, remove, or list cross-compilation targets.", Synopsis: "<args>"},
		{Name: "update", Summary: "Update Rust toolchains.", Synopsis: "[<toolchain>]"},
		{Name: "default", Summary: "Set the default toolchain.", Synopsis: "<toolchain>"},
		{Name: "override", Summary: "Manage per-directory toolchain overrides.", Synopsis: "<args>"},
		{Name: "which", Summary: "Show the path to a toolchain binary.", Synopsis: "<binary>"},
		{Name: "show", Summary: "Show the active and installed toolchains."},
		{Name: "run", Summary: "Run a command with a given toolchain.", Synopsis: "<toolchain> <command>"},
		{Name: "doc", Summary: "Open the Rust documentation.", Synopsis: "[<args>]"},
		{Name: "check", Summary: "Check for updates to Rust toolchains."},
		{Name: "self", Summary: "Manage the rustup installation.", Synopsis: "<args>"},
		{Name: "reload", Summary: "Reinitialize the language server."},
	},
}

// reloadFunc reinitializes the language server after a toolchain or
// default-channel change.
type reloadFunc func(ctx context.Context) error

type rustHandler struct {
	exec      workspaceapi.Executor
	notify    browserapi.Notifications
	cwd       string
	rustupBin string
	reload    reloadFunc
}

var _ textapi.REPLHandler = (*rustHandler)(nil)

// newRustHandler builds the `rust` REPL command handler and its manual.
// reload may be nil, in which case the `rust reload` subcommand is a
// no-op.
func newRustHandler(
	exec workspaceapi.Executor,
	notify browserapi.Notifications,
	cwd, rustupBin string,
	reload reloadFunc,
) (textapi.CommandManual, textapi.REPLHandler) {
	return rustManual, &rustHandler{
		exec:      exec,
		notify:    notify,
		cwd:       cwd,
		rustupBin: rustupBin,
		reload:    reload,
	}
}

// HandleCommand routes the first arg to the matching rustup subcommand,
// or reinitializes the language server for `reload`. Toolchain-mutating
// subcommands (default, toolchain, target) trigger a reload afterward so
// the server tracks the new channel.
func (h *rustHandler) HandleCommand(
	ctx context.Context, cmd repl.Command, _ repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	if len(cmd.Args) == 0 {
		return markdownOutput(usageMarkdown(rustManual)), nil
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
		return markdownOutput("Rust language server reloaded."), nil
	}

	prefix, ok := rustupRoutes[sub]
	if !ok {
		return nil, fmt.Errorf("unknown command: %s", sub)
	}
	args := append(append([]string{}, prefix...), cmd.Args[1:]...)
	out, err := h.runRustupCapture(ctx, args...)
	if err != nil {
		return nil, err
	}
	if mutatesToolchain(sub) && h.reload != nil {
		if err := h.reload(ctx); err != nil {
			return nil, err
		}
	}
	return markdownOutput(fmt.Sprintf("```\n%s\n```", out)), nil
}

// mutatesToolchain reports whether a subcommand may change the active
// toolchain or its binaries, requiring server reinitialization.
func mutatesToolchain(sub string) bool {
	switch sub {
	case "default", "toolchain", "target", "component", "update":
		return true
	default:
		return false
	}
}

// Complete offers the `rust` subcommands at depth 0 and the nested
// rustup subcommands of a command group at depth 1. Deeper positions
// defer to rustup at runtime and return no completions.
func (h *rustHandler) Complete(
	_ context.Context, _ string, args []string,
) (iterator.Iterator[string], error) {
	if len(args) <= 1 {
		filter := ""
		if len(args) == 1 {
			filter = args[0]
		}
		return iterator.FromSlice(filterNames(rustSubcommandNames, filter)), nil
	}
	if len(args) == 2 {
		if nested, ok := rustNestedSubcommands[args[0]]; ok {
			return iterator.FromSlice(filterNames(nested, args[1])), nil
		}
	}
	return iterator.FromSlice[string](nil), nil
}

// Help renders the manual for the command tree, descending into
// subcommands named in args.
func (h *rustHandler) Help(
	_ context.Context, args []string,
) (iterator.Iterator[component.Responsive], error) {
	man := rustManual
	for _, a := range args {
		sub, ok := findSubcommand(man, a)
		if !ok {
			break
		}
		man = sub
	}
	return markdownOutput(usageMarkdown(man)), nil
}

func (h *rustHandler) runRustupCapture(
	ctx context.Context, args ...string,
) (string, error) {
	bin := h.rustupBin
	if bin == "" {
		bin = "rustup"
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
		return "", fmt.Errorf("start rustup %s: %w", strings.Join(args, " "), err)
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
		return "", fmt.Errorf("rustup %s: %w\n%s", strings.Join(args, " "), runErr, combined)
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
