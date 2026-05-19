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

package extensionv2

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/component/markdown"
	"unstable.build/go-tui/text"
)

const (
	extensionsREPLCommand        = "extensions"
	extensionsREPLCommandStatus  = "status"
	extensionsREPLCommandInfo    = "info"
	extensionsREPLCommandStart   = "start"
	extensionsREPLCommandStop    = "stop"
	extensionsREPLCommandRestart = "restart"
	extensionsREPLCommandLogs    = "logs"
)

func registerExtensionsREPLCommand(
	runner *workspaceRunner, editor text.Editor,
) error {
	return editor.RegisterREPLCommand(textapi.CommandManual{
		Name:     extensionsREPLCommand,
		Summary:  "Manage workspace extensions.",
		Synopsis: "<status|info|start|stop|restart|logs> ...",
		Commands: []textapi.CommandManual{
			{Name: extensionsREPLCommandStatus, Summary: "Show workspace extension status."},
			{Name: extensionsREPLCommandInfo, Summary: "Show detailed workspace extension information.", Synopsis: "<id>"},
			{
				Name:     extensionsREPLCommandStart,
				Summary:  "Start a workspace extension.",
				Synopsis: "<id> <cmdAndArgs> [--config <json>]",
			},
			{Name: extensionsREPLCommandStop, Summary: "Stop a running workspace extension.", Synopsis: "<id>"},
			{Name: extensionsREPLCommandRestart, Summary: "Restart a known workspace extension.", Synopsis: "<id>"},
			{
				Name:     extensionsREPLCommandLogs,
				Summary:  "Show the captured stderr logs of an extension.",
				Synopsis: "<id> [--tail N]",
			},
		},
	}, extensionsREPLHandler{runner: runner})
}

type extensionsREPLHandler struct {
	runner *workspaceRunner
}

func (h extensionsREPLHandler) HandleCommand(
	ctx context.Context, cmd repl.Command, _ repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	if len(cmd.Args) == 0 {
		return extensionsREPLHelp(), nil
	}
	switch cmd.Args[0] {
	case extensionsREPLCommandStatus:
		return h.handleStatus(), nil
	case extensionsREPLCommandInfo:
		return h.handleInfo(cmd.Args[1:])
	case extensionsREPLCommandStart:
		return h.handleStart(ctx, cmd.Args[1:])
	case extensionsREPLCommandStop:
		return h.handleStop(cmd.Args[1:])
	case extensionsREPLCommandRestart:
		return h.handleRestart(ctx, cmd.Args[1:])
	case extensionsREPLCommandLogs:
		return h.handleLogs(cmd.Args[1:])
	default:
		return nil, fmt.Errorf("unknown extensions subcommand %q", cmd.Args[0])
	}
}

func (h extensionsREPLHandler) Complete(
	_ context.Context, _ string, args []string,
) (iterator.Iterator[string], error) {
	subcommands := []string{
		extensionsREPLCommandStatus,
		extensionsREPLCommandInfo,
		extensionsREPLCommandStart,
		extensionsREPLCommandStop,
		extensionsREPLCommandRestart,
		extensionsREPLCommandLogs,
	}
	if len(args) == 0 {
		return iterator.FromSlice(subcommands), nil
	}
	if len(args) == 1 {
		var ret []string
		for _, subcommand := range subcommands {
			if strings.HasPrefix(subcommand, args[0]) {
				ret = append(ret, subcommand)
			}
		}
		return iterator.FromSlice(ret), nil
	}
	if args[0] != extensionsREPLCommandStop && args[0] != extensionsREPLCommandRestart &&
		args[0] != extensionsREPLCommandInfo && args[0] != extensionsREPLCommandLogs {
		return iterator.Empty[string](), nil
	}
	if args[0] == extensionsREPLCommandLogs && len(args) >= 3 {
		// After `logs <id>`: complete the optional `--tail` flag
		// (or stay silent once it has been entered and is awaiting a
		// numeric argument).
		last := args[len(args)-1]
		prev := args[len(args)-2]
		if prev == "--tail" {
			return iterator.Empty[string](), nil
		}
		if strings.HasPrefix("--tail", last) {
			return iterator.FromSlice([]string{"--tail"}), nil
		}
		return iterator.Empty[string](), nil
	}
	prefix := args[len(args)-1]
	var ret []string
	for _, state := range h.runner.listExtensions() {
		if strings.HasPrefix(state.ID, prefix) {
			ret = append(ret, state.ID)
		}
	}
	return iterator.FromSlice(ret), nil
}

func (h extensionsREPLHandler) Help(
	context.Context, []string,
) (iterator.Iterator[component.Responsive], error) {
	return extensionsREPLHelp(), nil
}

func (h extensionsREPLHandler) handleStatus() iterator.Iterator[component.Responsive] {
	states := h.runner.listExtensions()
	if len(states) == 0 {
		return extensionsREPLLines("No workspace extensions.")
	}
	var b strings.Builder
	b.WriteString("| ID | Status | PID | Starts | Uptime |\n")
	b.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, state := range states {
		status := extensionsStatusLabel(state)
		pid := "-"
		uptime := "-"
		if state.Running {
			pid = fmt.Sprintf("%d", state.Pid)
			if !state.Started.IsZero() {
				uptime = extensionsFormatDuration(time.Since(state.Started))
			}
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n",
			extensionsMarkdownTableCell(state.ID),
			extensionsMarkdownTableCell(status),
			extensionsMarkdownTableCell(pid),
			extensionsMarkdownTableCell(strconv.Itoa(state.StartCount)),
			extensionsMarkdownTableCell(uptime),
		)
	}
	md, err := markdown.New(b.String())
	if err != nil {
		return extensionsREPLLines(b.String())
	}
	return iterator.FromSlice([]component.Responsive{md})
}

func (h extensionsREPLHandler) handleInfo(
	args []string,
) (iterator.Iterator[component.Responsive], error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("usage: extensions info <id>")
	}
	state, ok := h.runner.extension(args[0])
	if !ok {
		return nil, fmt.Errorf("extension %q not found", args[0])
	}
	content := extensionsInfoMarkdown(state)
	cfg := markdown.DefaultConfig()
	statusAttr := extensionsStatusAttr(extensionsStatusLabel(state))
	cfg.Bold = statusAttr
	md, err := markdown.NewWithConfig(content, cfg)
	if err != nil {
		return extensionsREPLLines(content), nil
	}
	return iterator.FromSlice([]component.Responsive{md}), nil
}

func extensionsMarkdownTableCell(value string) string {
	value = strings.ReplaceAll(value, "|", `\|`)
	value = strings.ReplaceAll(value, "\n", "<br>")
	if value == "" {
		return "-"
	}
	return value
}

func (h extensionsREPLHandler) handleStart(
	ctx context.Context, args []string,
) (iterator.Iterator[component.Responsive], error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("usage: extensions start <id> <cmdAndArgs> [--config <json>]")
	}
	id := args[0]
	configIdx := -1
	for i := 1; i < len(args); i++ {
		if args[i] == "--config" {
			configIdx = i
			break
		}
	}
	cmdParts := args[1:]
	cfg := config.MapConfig(map[string]any{})
	if configIdx != -1 {
		cmdParts = args[1:configIdx]
		if configIdx+1 >= len(args) {
			return nil, fmt.Errorf("usage: extensions start <id> <cmdAndArgs> [--config <json>]")
		}
		var values map[string]any
		if err := json.Unmarshal([]byte(strings.Join(args[configIdx+1:], " ")), &values); err != nil {
			return nil, fmt.Errorf("parse config json: %w", err)
		}
		cfg = config.MapConfig(values)
	}
	if len(cmdParts) == 0 {
		return nil, fmt.Errorf("usage: extensions start <id> <cmdAndArgs> [--config <json>]")
	}
	cmdAndArgs := strings.Join(cmdParts, " ")
	if err := h.runner.startExtension(ctx, id, cmdAndArgs, cfg); err != nil {
		return nil, err
	}
	return extensionsREPLLines(fmt.Sprintf("Started extension %s", id)), nil
}

func (h extensionsREPLHandler) handleStop(args []string) (iterator.Iterator[component.Responsive], error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("usage: extensions stop <id>")
	}
	if err := h.runner.stopExtensionByID(args[0]); err != nil {
		return nil, err
	}
	return extensionsREPLLines(fmt.Sprintf("Stopped extension %s", args[0])), nil
}

func (h extensionsREPLHandler) handleRestart(
	ctx context.Context, args []string,
) (iterator.Iterator[component.Responsive], error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("usage: extensions restart <id>")
	}
	if err := h.runner.restartExtension(ctx, args[0]); err != nil {
		return nil, err
	}
	return extensionsREPLLines(fmt.Sprintf("Restarted extension %s", args[0])), nil
}

func (h extensionsREPLHandler) handleLogs(
	args []string,
) (iterator.Iterator[component.Responsive], error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("usage: extensions logs <id> [--tail N]")
	}
	id := args[0]
	// Default to 0 (no limit). The user can Ctrl-C the iterator if
	// the file is large; the file path is also surfaced in the
	// header so they can open it externally.
	tail := 0
	rest := args[1:]
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--tail":
			if i+1 >= len(rest) {
				return nil, fmt.Errorf("usage: extensions logs <id> [--tail N]")
			}
			n, err := strconv.Atoi(rest[i+1])
			if err != nil || n < 0 {
				return nil, fmt.Errorf("--tail expects a non-negative integer, got %q", rest[i+1])
			}
			tail = n
			i++
		default:
			return nil, fmt.Errorf("unknown extensions logs argument %q", rest[i])
		}
	}
	state, ok := h.runner.extension(id)
	if !ok {
		return nil, fmt.Errorf("extension %q not found", id)
	}
	return newLogFileIterator(state.LogPath, tail)
}

// newLogFileIterator returns an iterator that emits a header line
// followed by the contents of logPath one line at a time. When
// tail > 0 the iterator emits only the last tail lines; tail == 0
// emits every line. If the file does not exist the iterator yields
// a single "No logs captured yet." line. The underlying file is
// closed by the iterator's Close.
func newLogFileIterator(
	logPath string, tail int,
) (iterator.Iterator[component.Responsive], error) {
	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return extensionsREPLLines("No logs captured yet."), nil
		}
		return nil, fmt.Errorf("open log file %q: %w", logPath, err)
	}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	// With --tail N>0 we scan the whole file up-front and keep just
	// the last N lines in a ring buffer. The iterator then replays
	// them. tail == 0 streams every line directly from the scanner.
	var ring []string
	if tail > 0 {
		ring = make([]string, 0, tail)
		for scanner.Scan() {
			if len(ring) == tail {
				ring = append(ring[1:], scanner.Text())
			} else {
				ring = append(ring, scanner.Text())
			}
		}
		if err := scanner.Err(); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("scan log file %q: %w", logPath, err)
		}
	}

	emittedHeader := false
	ringIdx := 0
	return iterator.FromFunc(
		func(context.Context) (component.Responsive, bool, error) {
			if !emittedHeader {
				emittedHeader = true
				return logFileHeader(logPath), true, nil
			}
			if ring != nil {
				if ringIdx >= len(ring) {
					return nil, false, nil
				}
				line := ring[ringIdx]
				ringIdx++
				return responsiveString(line), true, nil
			}
			if scanner.Scan() {
				return responsiveString(scanner.Text()), true, nil
			}
			if err := scanner.Err(); err != nil {
				return nil, false, fmt.Errorf("scan log file %q: %w", logPath, err)
			}
			return nil, false, nil
		},
		f.Close,
	), nil
}

// logFileHeader renders the log file path as a bold markdown line
// so the path stands out from the (potentially long) stream of log
// lines that follow. Falls back to a plain string if markdown
// rendering fails.
func logFileHeader(logPath string) component.Responsive {
	md, err := markdown.New(fmt.Sprintf("**Log file:** `%s`", logPath))
	if err != nil {
		return responsiveString(fmt.Sprintf("Log file: %s", logPath))
	}
	return md
}

func responsiveString(s string) component.Responsive {
	return component.NewResponsiveString(s, component.StringResponsiveConfig{})
}

func extensionsREPLHelp() iterator.Iterator[component.Responsive] {
	return extensionsREPLLines(
		"Usage: extensions <status|info|start|stop|restart|logs>",
		"Show status/info/logs, start, stop, or restart workspace extensions.",
	)
}

func extensionsStatusLabel(state extensionRunStateSnapshot) string {
	if state.LastErr != nil {
		return "Errored"
	}
	if state.Running {
		return "Running"
	}
	return "Stopped"
}

func extensionsStatusAttr(status string) term.Attributes {
	switch status {
	case "Running":
		return term.Attributes{Fg: term.ColorGreen, Attrs: term.AttrBold}
	case "Stopped":
		return term.Attributes{Fg: term.ColorYellow, Attrs: term.AttrBold}
	case "Errored":
		return term.Attributes{Fg: term.ColorRed, Attrs: term.AttrBold}
	default:
		return term.Attributes{Fg: term.ColorDefault, Attrs: term.AttrBold}
	}
}

func extensionsFormatDuration(d time.Duration) string {
	if d < time.Second {
		return "0s"
	}
	return d.Round(time.Second).String()
}

func extensionsInfoMarkdown(state extensionRunStateSnapshot) string {
	started := "-"
	if !state.Started.IsZero() {
		started = state.Started.Format(time.RFC3339)
	}
	uptime := "-"
	if state.Running && !state.Started.IsZero() {
		uptime = extensionsFormatDuration(time.Since(state.Started))
	}
	pid := "-"
	if state.Pid != 0 {
		pid = fmt.Sprintf("%d", state.Pid)
	}
	lastErr := "-"
	if state.LastErr != nil {
		lastErr = state.LastErr.Error()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Extension `%s`\n\n", strings.ReplaceAll(state.ID, "`", ""))
	fmt.Fprintf(&b, "Status: **%s**\n\n", extensionsStatusLabel(state))
	b.WriteString("| Field | Value |\n")
	b.WriteString("| --- | --- |\n")
	fmt.Fprintf(&b, "| PID | %s |\n", extensionsMarkdownTableCell(pid))
	fmt.Fprintf(&b, "| Starts | %d |\n", state.StartCount)
	fmt.Fprintf(&b, "| Started | %s |\n", extensionsMarkdownTableCell(started))
	fmt.Fprintf(&b, "| Uptime | %s |\n", extensionsMarkdownTableCell(uptime))
	fmt.Fprintf(&b, "| Command | %s |\n", extensionsInfoValue(state.CmdAndArgs))
	fmt.Fprintf(&b, "| Last error | %s |\n", extensionsInfoValue(lastErr))
	fmt.Fprintf(&b, "| Log file | %s |\n", extensionsInfoValue(state.LogPath))

	flatCfg := extensionsFlattenConfig(state.Config)
	if len(flatCfg) == 0 {
		b.WriteString("\n## Configuration\n\nNo configuration.\n")
		return b.String()
	}
	b.WriteString("\n## Configuration\n\n")
	b.WriteString("| Key | Value |\n")
	b.WriteString("| --- | --- |\n")
	for _, entry := range flatCfg {
		fmt.Fprintf(&b, "| %s | %s |\n",
			extensionsMarkdownTableCell(entry.key),
			extensionsInfoValue(entry.value),
		)
	}
	return b.String()
}

type extensionsConfigEntry struct {
	key   string
	value string
}

func extensionsFlattenConfig(cfg config.Config) []extensionsConfigEntry {
	values := map[string]any{}
	if cfg != nil {
		cfg.Iterate(func(k string, value any) {
			values[k] = value
		})
	}
	var entries []extensionsConfigEntry
	var walk func(prefix string, value any)
	walk = func(prefix string, value any) {
		switch v := value.(type) {
		case map[string]any:
			keys := make([]string, 0, len(v))
			for key := range v {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				next := key
				if prefix != "" {
					next = prefix + "." + key
				}
				walk(next, v[key])
			}
		default:
			entries = append(entries, extensionsConfigEntry{
				key:   prefix,
				value: extensionsConfigValue(value),
			})
		}
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		walk(key, values[key])
	}
	return entries
}

func extensionsConfigValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", value)
		}
		return string(data)
	}
}

func extensionsInfoValue(value string) string {
	value = strings.ReplaceAll(value, "`", "'")
	value = strings.ReplaceAll(value, "\n", " ")
	return "`" + value + "`"
}

func extensionsREPLLines(lines ...string) iterator.Iterator[component.Responsive] {
	out := make([]component.Responsive, len(lines))
	for i, line := range lines {
		out[i] = component.NewResponsiveString(line, component.StringResponsiveConfig{})
	}
	return iterator.FromSlice(out)
}
