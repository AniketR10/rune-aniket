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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// hoverCmd is a rust-specific hover: it renders rust-analyzer's hover
// documentation as markdown in a floating window and, when the server
// attaches hover actions (Run/Debug, Go to implementations/references,
// Go to type), offers them in a picker.
//
// Hover actions only come back when the client advertised both
// experimental.hoverActions and the matching command names in
// experimental.commands.commands (see experimentalCaps in initialize.go),
// which the extension does under the `experimental` config flag. Without
// that opt-in, rust-analyzer suppresses every action link and this command
// degrades to a plain markdown hover.
type hoverCmd struct {
	pickDeps
	lsp semanticapi.LSP
	// runner executes the runnable carried by a runSingle/debugSingle hover
	// action. Debug has no adapter here, so it runs the target like Run.
	runner *runCmd
}

var _ textapi.CommandHandler = (*hoverCmd)(nil)

// hoverResult is the subset of rust-analyzer's hover response this command
// consumes: the markup contents and the optional action link groups.
type hoverResult struct {
	Contents struct {
		Value string `json:"value"`
	} `json:"contents"`
	Actions []hoverCommandGroup `json:"actions"`
}

// hoverCommandGroup is a titled group of command links, matching
// rust-analyzer's CommandLinkGroup.
type hoverCommandGroup struct {
	Title    string             `json:"title"`
	Commands []hoverCommandLink `json:"commands"`
}

// hoverCommandLink is one actionable link in a hover action group. It
// extends the LSP Command with a tooltip, mirroring rust-analyzer's
// CommandLink.
type hoverCommandLink struct {
	Title     string            `json:"title"`
	Command   string            `json:"command"`
	Tooltip   string            `json:"tooltip"`
	Arguments []json.RawMessage `json:"arguments"`
}

func (c *hoverCmd) HandleCommand(ctx context.Context, cmd textapi.Command) error {
	if err := requireFile(cmd); err != nil {
		return err
	}
	// Capture the invoking window before the markdown float (and any
	// action picker) takes the focus: hover actions jump into it.
	win, err := c.wm.Focus()
	if err != nil {
		return err
	}
	res, err := execRequest[*hoverResult](ctx, c.lsp, "textDocument/hover", posParams(cmd))
	if err != nil {
		return err
	}
	if res == nil || strings.TrimSpace(res.Contents.Value) == "" {
		_, _ = c.notify.Notify(browserapi.LevelInfo, "No hover information at the cursor")
		return nil
	}
	if err := showMarkdown(c.wm, c.parser, c.interrupt, res.Contents.Value); err != nil {
		return err
	}
	links := flattenHoverActions(res.Actions)
	if len(links) == 0 {
		return nil
	}
	return c.pickAction(ctx, win, cmd.URI, links)
}

// pickAction floats a picker of the available hover actions and dispatches
// the chosen one.
func (c *hoverCmd) pickAction(
	ctx context.Context, win browserapi.Window,
	base workspaceapi.URI, links []hoverCommandLink,
) error {
	ch := make(chan int, 1)
	titles := make([]string, len(links))
	for i, l := range links {
		titles[i] = strings.TrimSpace(l.Title)
		if titles[i] == "" {
			titles[i] = l.Command
		}
	}
	picker := newListPicker(titles, ch)
	if _, err := c.wm.Floating(picker, browserapi.FloatingConfig{
		Alignment: component.AlignmentCentered,
	}); err != nil {
		return fmt.Errorf("show hover actions: %w", err)
	}
	select {
	case idx := <-ch:
		if idx < 0 || idx >= len(links) {
			return nil
		}
		return c.dispatch(ctx, win, base, links[idx])
	case <-ctx.Done():
		return ctx.Err()
	}
}

// dispatch performs the client-side action for a hover command link.
// gotoLocation and showReferences navigate the buffer; runSingle/debugSingle
// execute the carried runnable (debug lacks an adapter, so it runs like run).
func (c *hoverCmd) dispatch(
	ctx context.Context, win browserapi.Window,
	base workspaceapi.URI, link hoverCommandLink,
) error {
	switch link.Command {
	case "rust-analyzer.gotoLocation":
		loc, ok := parseGotoLocation(link.Arguments)
		if !ok {
			_, _ = c.notify.Notify(browserapi.LevelInfo, "No location for %q", link.Title)
			return nil
		}
		return openLocation(c.editor, c.wm, c.opener, win, base, loc)
	case "rust-analyzer.showReferences":
		locs := parseShowReferences(link.Arguments)
		return c.showLocations(ctx, win, base, locs, link.Tooltip)
	case "rust-analyzer.runSingle", "rust-analyzer.debugSingle":
		r, ok := parseRunnable(link.Arguments)
		if !ok {
			_, _ = c.notify.Notify(browserapi.LevelInfo, "No runnable for %q", link.Title)
			return nil
		}
		return c.runner.run(ctx, r)
	default:
		label := strings.TrimSpace(link.Tooltip)
		if label == "" {
			label = strings.TrimSpace(link.Title)
		}
		_, _ = c.notify.Notify(browserapi.LevelInfo, "%s", label)
		return nil
	}
}

// showLocations offers the reference locations in a picker and jumps to
// the chosen one.
func (c *hoverCmd) showLocations(
	ctx context.Context, win browserapi.Window,
	base workspaceapi.URI, locs []semanticapi.Location, label string,
) error {
	entries := make([]pickEntry, len(locs))
	for i, l := range locs {
		entries[i] = pickEntry{
			loc: l,
			display: fmt.Sprintf("%s:%d:%d", c.relPath(base, l.URI),
				l.Range.Start.Line+1, l.Range.Start.Character+1),
		}
	}
	return c.present(ctx, win, base, entries, fmt.Sprintf("No references for %q", label))
}

func (c *hoverCmd) Complete(_ context.Context, _ string, _ []string) (
	iterator.Iterator[string], error,
) {
	return iterator.Empty[string](), nil
}

// flattenHoverActions collapses the grouped hover actions into a flat,
// display-ordered list of command links.
func flattenHoverActions(groups []hoverCommandGroup) []hoverCommandLink {
	var out []hoverCommandLink
	for _, g := range groups {
		out = append(out, g.Commands...)
	}
	return out
}

// parseGotoLocation decodes rust-analyzer.gotoLocation's single {uri, range}
// argument into a Location.
func parseGotoLocation(args []json.RawMessage) (semanticapi.Location, bool) {
	if len(args) == 0 {
		return semanticapi.Location{}, false
	}
	var loc semanticapi.Location
	if err := json.Unmarshal(args[0], &loc); err != nil || loc.URI == "" {
		return semanticapi.Location{}, false
	}
	return loc, true
}

// parseShowReferences decodes rust-analyzer.showReferences' [uri, position,
// locations] arguments into the reference locations (the third element).
func parseShowReferences(args []json.RawMessage) []semanticapi.Location {
	if len(args) < 3 {
		return nil
	}
	var locs []semanticapi.Location
	if err := json.Unmarshal(args[2], &locs); err != nil {
		return nil
	}
	return locs
}

// parseRunnable decodes rust-analyzer.runSingle/debugSingle's single Runnable
// argument.
func parseRunnable(args []json.RawMessage) (runnable, bool) {
	if len(args) == 0 {
		return runnable{}, false
	}
	var r runnable
	if err := json.Unmarshal(args[0], &r); err != nil || r.Kind == "" {
		return runnable{}, false
	}
	return r, true
}
