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


// Package workspaceshell provides a workspaceapi.Executor wrapper
// that tracks running processes and exposes a "process" shell
// command via ideshell.CommandHandler.
package workspaceshell

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/component/markdown"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/ide/ideshell"
	"unstable.build/go-tui/workspace/processctx"
)

var (
	_ workspaceapi.Executor   = (*Executor)(nil)
	_ schemeapi.Executor      = (*Executor)(nil)
	_ ideshell.CommandHandler = (*Executor)(nil)
)

// Executor wraps a workspaceapi.Executor, tracking every
// started process so it can be listed, signalled, or stopped
// from a repl.Handler. It implements both
// workspaceapi.Executor and ideshell.CommandHandler.
type Executor struct {
	mu         sync.RWMutex
	underlying workspaceapi.Executor
	processes  map[workspaceapi.Pid]processInfo
	history    map[workspaceapi.Pid]processInfo
	stats      map[cmdKey]*cmdStats
	extensions map[string]workspaceapi.Pid
	now        func() time.Time // for testing
	stopGrace  time.Duration    // grace period for stop
}

type processInfo struct {
	pid     workspaceapi.Pid
	path    string
	args    []string
	dir     string
	env     []string
	started time.Time
	ended   time.Time
	key     cmdKey
	parent  workspaceapi.Pid // 0 means no parent
	done    chan struct{}    // closed when process exits
	lastErr error            // set on exit for audit history
}

// cmdKey identifies a command by its path and arguments,
// used to aggregate stats across process lifetimes.
type cmdKey string

func makeCmdKey(path string, args []string) cmdKey {
	return cmdKey(path + "\x00" + strings.Join(args, "\x00"))
}

// cmdStats tracks aggregate statistics for a command
// across process lifetimes.
type cmdStats struct {
	lastErr error
}

const defaultStopGrace = 3 * time.Second

// NewExecutor returns an Executor that delegates to
// underlying while tracking running processes.
func NewExecutor(underlying workspaceapi.Executor) *Executor {
	return &Executor{
		underlying: underlying,
		processes:  make(map[workspaceapi.Pid]processInfo),
		history:    make(map[workspaceapi.Pid]processInfo),
		stats:      make(map[cmdKey]*cmdStats),
		extensions: make(map[string]workspaceapi.Pid),
		now:        time.Now,
		stopGrace:  defaultStopGrace,
	}
}

// Start delegates to the underlying executor and tracks the
// started process. The cmd's Watcher is piped so that
// process exit automatically removes the entry.
func (e *Executor) Start(
	ctx context.Context, cmd workspaceapi.Cmd,
) (workspaceapi.Pid, error) {
	ch := make(chan error, 1)
	ours := workspaceapi.ChanProcessWatcher(ch)
	if cmd.Watcher != nil {
		cmd.Watcher = workspaceapi.MultiProcessWatcher(
			cmd.Watcher, ours,
		)
	} else {
		cmd.Watcher = ours
	}

	pid, err := e.underlying.Start(ctx, cmd)
	if err != nil {
		return pid, err
	}

	key := makeCmdKey(cmd.Path, cmd.Args)
	info := processInfo{
		pid:     pid,
		path:    cmd.Path,
		args:    append([]string{}, cmd.Args...),
		dir:     cmd.Dir,
		env:     append([]string{}, cmd.Env...),
		started: e.now(),
		key:     key,
		done:    make(chan struct{}),
	}

	if parent, ok := ParentPidFromContext(ctx); ok {
		info.parent = parent
	}

	e.mu.Lock()
	if info.parent == 0 {
		if extensionID, ok := processctx.ExtensionIDFromContext(ctx); ok {
			info.parent = e.extensions[extensionID]
		}
	}
	e.processes[pid] = info
	e.history[pid] = info
	if extensionID, ok := processctx.ExtensionIDFromContext(ctx); ok && info.parent == 0 {
		e.extensions[extensionID] = pid
	}
	e.mu.Unlock()

	go debug.CapturePanicReport(func() {

		exitErr := <-ch
		e.mu.Lock()
		s := e.stats[key]
		if s == nil {
			s = &cmdStats{}
			e.stats[key] = s
		}
		s.lastErr = exitErr
		if p, ok := e.processes[pid]; ok {
			close(p.done)
		}
		delete(e.processes, pid)
		for extensionID, extensionPid := range e.extensions {
			if extensionPid == pid {
				delete(e.extensions, extensionID)
			}
		}
		if h, ok := e.history[pid]; ok {
			h.ended = e.now()
			h.lastErr = exitErr
			e.history[pid] = h
		}
		e.mu.Unlock()

	})

	return pid, nil
}

// Signal delegates to the underlying executor.
func (e *Executor) Signal(
	pid workspaceapi.Pid, sig syscall.Signal,
) error {
	return e.underlying.Signal(pid, sig)
}

// Close delegates to the underlying executor.
func (e *Executor) Close() error {
	return e.underlying.Close()
}

// StartCommand implements schemeapi.Executor by delegating to Start.
func (e *Executor) StartCommand(
	ctx context.Context, cmd workspaceapi.Cmd,
) (workspaceapi.Pid, error) {
	return e.Start(ctx, cmd)
}

// RegisterCommands registers the "process" command in the
// given registry with the executor as its handler.
func (e *Executor) RegisterCommands(r *ideshell.CommandRegistry) {
	r.Register("process", "Process management", e)
}

// HandleCommand dispatches process subcommands.
func (e *Executor) HandleCommand(
	ctx context.Context, cmd repl.Command, _ repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	if cmd.Name != "process" {
		return nil, repl.ErrNotFound
	}
	if len(cmd.Args) == 0 {
		return e.Help(ctx, nil)
	}
	switch cmd.Args[0] {
	case "status":
		return e.handleStatus(), nil
	case "tree":
		return e.handleTree(), nil
	case "audit":
		return e.handleAudit(), nil
	case "info":
		return e.handleInfo(cmd.Args[1:])
	case "signal":
		return e.handleSignal(cmd.Args[1:])
	case "stop":
		return e.handleStop(cmd.Args[1:])
	default:
		return nil, fmt.Errorf("unknown process subcommand: %s", cmd.Args[0])
	}
}

// Complete returns process subcommand and PID completions.
func (e *Executor) Complete(
	_ context.Context, cmd string, args []string,
) (iterator.Iterator[string], error) {
	if cmd != "process" {
		return iterator.Empty[string](), nil
	}
	if len(args) == 0 {
		return iterator.FromSlice(processSubcommands()), nil
	}
	if len(args) == 1 {
		return completeStrings(processSubcommands(), args[0]), nil
	}
	if completesPID(args[0]) {
		return e.completePids(args[len(args)-1]), nil
	}
	return iterator.Empty[string](), nil
}

// Help returns usage information for process subcommands.
func (e *Executor) Help(
	_ context.Context, _ []string,
) (iterator.Iterator[component.Responsive], error) {
	return toLines(
		"Process management commands:",
		"",
		"  process status             List running processes",
		"  process audit              List all processes (including exited)",
		"  process tree               Show process tree (parent→child)",
		"  process info <pid>         Show detailed process information",
		"  process signal <pid> [N]   Send signal N to a process (default: SIGTERM)",
		"  process signal -N <pid>    Send signal N to a process",
		"  process stop <pid>         Gracefully stop a process (SIGTERM, then SIGKILL)",
	), nil
}

type psEntry struct {
	pid     workspaceapi.Pid
	uptime  time.Duration
	lastErr string
	command string
}

func (e *Executor) handleStatus() iterator.Iterator[component.Responsive] {
	now := e.now()
	e.mu.RLock()
	entries := make([]psEntry, 0, len(e.processes))
	for _, info := range e.processes {
		cmd := info.path
		if len(info.args) > 0 {
			cmd += " " + strings.Join(info.args, " ")
		}
		lastErr := "—"
		if s := e.stats[info.key]; s != nil {
			lastErr = formatLastErr(s.lastErr)
		}
		entries = append(entries, psEntry{
			pid:     info.pid,
			uptime:  now.Sub(info.started),
			lastErr: lastErr,
			command: cmd,
		})
	}
	e.mu.RUnlock()
	return renderProcessTableMarkdown(entries)
}

func (e *Executor) handleAudit() iterator.Iterator[component.Responsive] {
	now := e.now()
	e.mu.RLock()
	entries := make([]psEntry, 0, len(e.history))
	for _, info := range e.history {
		cmd := info.path
		if len(info.args) > 0 {
			cmd += " " + strings.Join(info.args, " ")
		}
		uptime := now.Sub(info.started)
		if !info.ended.IsZero() {
			uptime = info.ended.Sub(info.started)
		}
		lastErr := "—"
		if info.lastErr != nil {
			lastErr = formatLastErr(info.lastErr)
		} else if s := e.stats[info.key]; s != nil {
			lastErr = formatLastErr(s.lastErr)
		}
		entries = append(entries, psEntry{
			pid:     info.pid,
			uptime:  uptime,
			lastErr: lastErr,
			command: cmd,
		})
	}
	e.mu.RUnlock()
	return renderProcessTableMarkdown(entries)
}

func (e *Executor) handleTree() iterator.Iterator[component.Responsive] {
	now := e.now()
	e.mu.RLock()
	infos := make([]processInfo, 0, len(e.processes))
	for _, info := range e.processes {
		infos = append(infos, info)
	}
	e.mu.RUnlock()

	// Build children map and find roots.
	children := make(map[workspaceapi.Pid][]processInfo)
	var roots []processInfo
	for _, info := range infos {
		if info.parent == 0 {
			roots = append(roots, info)
		} else {
			children[info.parent] = append(
				children[info.parent], info)
		}
	}
	sort.Slice(roots, func(i, j int) bool {
		return roots[i].pid < roots[j].pid
	})
	for k := range children {
		c := children[k]
		sort.Slice(c, func(i, j int) bool {
			return c[i].pid < c[j].pid
		})
	}

	var b strings.Builder
	b.WriteString("- **Process tree**\n")

	var walk func(info processInfo, depth int)
	walk = func(info processInfo, depth int) {
		cmd := info.path
		if len(info.args) > 0 {
			cmd += " " + strings.Join(info.args, " ")
		}
		indent := strings.Repeat("  ", depth)
		ppidStr := "—"
		if info.parent != 0 {
			ppidStr = strconv.Itoa(int(info.parent))
		}
		fmt.Fprintf(
			&b,
			"%s- **PID %d** (PPID: %s, uptime: %s) — `%s`\n",
			indent,
			info.pid,
			ppidStr,
			formatDuration(now.Sub(info.started)),
			cmd,
		)
		for _, child := range children[info.pid] {
			walk(child, depth+1)
		}
	}

	for _, root := range roots {
		walk(root, 0)
	}
	return markdownResponsive(b.String())
}

func (e *Executor) handleInfo(
	args []string,
) (iterator.Iterator[component.Responsive], error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("usage: process info <pid>")
	}
	pidVal, err := strconv.Atoi(args[0])
	if err != nil {
		return nil, fmt.Errorf("invalid pid: %s", args[0])
	}
	pid := workspaceapi.Pid(pidVal)

	now := e.now()
	e.mu.RLock()
	info, ok := e.processes[pid]
	var stats *cmdStats
	if ok {
		stats = e.stats[info.key]
	}
	e.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("process %d not found", pid)
	}

	cmd := info.path
	if len(info.args) > 0 {
		cmd += " " + strings.Join(info.args, " ")
	}

	lastErr := "—"
	if stats != nil {
		lastErr = formatLastErr(stats.lastErr)
	}

	ppidStr := "—"
	if info.parent != 0 {
		ppidStr = strconv.Itoa(int(info.parent))
	}

	infoLines := []string{
		fmt.Sprintf("- **PID:** `%d`", info.pid),
		fmt.Sprintf("- **Parent PID:** `%s`", ppidStr),
		fmt.Sprintf("- **Command:** `%s`", cmd),
		fmt.Sprintf("- **Directory:** `%s`", info.dir),
		fmt.Sprintf("- **Started:** %s", info.started.Format(time.RFC3339)),
		fmt.Sprintf("- **Uptime:** %s", formatDuration(now.Sub(info.started))),
		fmt.Sprintf("- **Last Error:** %s", lastErr),
	}

	if len(info.env) > 0 {
		infoLines = append(infoLines, "- **Environment:**")
		for _, envVar := range info.env {
			infoLines = append(infoLines,
				fmt.Sprintf("  - `%s`", redactEnv(envVar)))
		}
	}

	return markdownResponsive(strings.Join(infoLines, "\n")), nil
}

func formatLastErr(err error) string {
	if err == nil {
		return "—"
	}
	const maxLen = 30
	s := err.Error()
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

func formatDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		m := int(d.Minutes())
		s := int(d.Seconds()) - m*60
		return fmt.Sprintf("%dm%ds", m, s)
	case d < 24*time.Hour:
		h := int(d.Hours())
		m := int(d.Minutes()) - h*60
		return fmt.Sprintf("%dh%dm", h, m)
	default:
		days := int(d.Hours()) / 24
		h := int(d.Hours()) - days*24
		return fmt.Sprintf("%dd%dh", days, h)
	}
}

func (e *Executor) handleSignal(
	args []string,
) (iterator.Iterator[component.Responsive], error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("usage: process signal [-signal] <pid>")
	}

	sig := syscall.SIGTERM
	pidStr := args[0]

	if strings.HasPrefix(pidStr, "-") && len(args) > 1 {
		n, err := strconv.Atoi(pidStr[1:])
		if err != nil {
			return nil, fmt.Errorf("invalid signal: %s", pidStr[1:])
		}
		sig = syscall.Signal(n)
		pidStr = args[1]
	} else if len(args) > 1 {
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return nil, fmt.Errorf("invalid signal: %s", args[1])
		}
		sig = syscall.Signal(n)
	}

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return nil, fmt.Errorf("invalid pid: %s", pidStr)
	}

	if err := e.Signal(workspaceapi.Pid(pid), sig); err != nil {
		return nil, err
	}

	return iterator.Empty[component.Responsive](), nil
}

func (e *Executor) handleStop(
	args []string,
) (iterator.Iterator[component.Responsive], error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("usage: process stop <pid>")
	}
	pidVal, err := strconv.Atoi(args[0])
	if err != nil {
		return nil, fmt.Errorf("invalid pid: %s", args[0])
	}
	pid := workspaceapi.Pid(pidVal)

	// Look up the done channel while holding the lock.
	e.mu.RLock()
	info, tracked := e.processes[pid]
	e.mu.RUnlock()

	// Send SIGTERM first.
	if err := e.Signal(pid, syscall.SIGTERM); err != nil {
		return nil, err
	}

	if !tracked {
		// Process not tracked; fall through to SIGKILL.
		if err := e.Signal(pid, syscall.SIGKILL); err != nil {
			return nil, err
		}
		return toLines("  sent SIGKILL (untracked process)"), nil
	}

	// Wait for graceful exit or timeout.
	select {
	case <-info.done:
		return toLines(
			fmt.Sprintf("  process %d stopped", pid),
		), nil
	case <-time.After(e.stopGrace):
		if err := e.Signal(pid, syscall.SIGKILL); err != nil {
			return nil, err
		}
		return toLines(
			fmt.Sprintf("  process %d killed (SIGTERM timed out)", pid),
		), nil
	}
}

func processSubcommands() []string {
	return []string{"status", "audit", "tree", "info", "signal", "stop"}
}

func completesPID(subcommand string) bool {
	return subcommand == "signal" || subcommand == "stop" ||
		subcommand == "info"
}

func completeStrings(values []string, prefix string) iterator.Iterator[string] {
	matches := make([]string, 0, len(values))
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			matches = append(matches, value)
		}
	}
	return iterator.FromSlice(matches)
}

func (e *Executor) completePids(prefix string) iterator.Iterator[string] {
	e.mu.RLock()
	pids := make([]string, 0, len(e.processes))
	for pid := range e.processes {
		s := strconv.Itoa(int(pid))
		if strings.HasPrefix(s, prefix) {
			pids = append(pids, s)
		}
	}
	e.mu.RUnlock()
	sort.Strings(pids)
	return iterator.FromSlice(pids)
}

func toLines(ss ...string) iterator.Iterator[component.Responsive] {
	out := make([]component.Responsive, len(ss))
	for i, s := range ss {
		out[i] = toResponsive(s)
	}
	return iterator.FromSlice(out)
}

func renderProcessTableMarkdown(entries []psEntry) iterator.Iterator[component.Responsive] {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].pid < entries[j].pid
	})
	var b strings.Builder
	b.WriteString("| PID | UPTIME | LAST ERR | COMMAND |\n")
	b.WriteString("| --- | --- | --- | --- |\n")
	for _, ent := range entries {
		fmt.Fprintf(
			&b,
			"| `%d` | `%s` | %s | `%s` |\n",
			ent.pid,
			formatDuration(ent.uptime),
			escapeMarkdownTableCell(ent.lastErr),
			escapeMarkdownTableCell(ent.command),
		)
	}
	return markdownResponsive(b.String())
}

func toResponsive(s string) component.Responsive {
	return component.NewResponsiveString(
		s, component.StringResponsiveConfig{},
	)
}

func markdownResponsive(content string) iterator.Iterator[component.Responsive] {
	md, err := markdown.New(content)
	if err != nil {
		return iterator.FromSlice([]component.Responsive{toResponsive(content)})
	}
	return iterator.FromSlice([]component.Responsive{md})
}

func escapeMarkdownTableCell(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

// ContextWithParentPid returns a context carrying the given
// parent PID. When a process is started with this context, the
// executor records it as a child of the specified parent.
func ContextWithParentPid(
	ctx context.Context, pid workspaceapi.Pid,
) context.Context {
	return processctx.ContextWithParentPid(ctx, pid)
}

// ParentPidFromContext extracts a parent PID previously stored
// via ContextWithParentPid. Returns 0, false when absent.
func ParentPidFromContext(ctx context.Context) (workspaceapi.Pid, bool) {
	return processctx.ParentPidFromContext(ctx)
}

// secretEnvKeys lists environment variable names whose values
// must be redacted in process info output.
var secretEnvKeys = map[string]bool{
	"RUNE_CERT":  true,
	"RUNE_TOKEN": true,
	"IDE_CERT":   true,
	"IDE_TOKEN":  true,
}

// redactEnv replaces the value portion of sensitive environment
// variables with "****". Non-sensitive variables and entries
// without an "=" are returned unchanged.
func redactEnv(envVar string) string {
	key, _, ok := strings.Cut(envVar, "=")
	if !ok {
		return envVar
	}
	if secretEnvKeys[key] {
		return key + "=****"
	}
	return envVar
}
