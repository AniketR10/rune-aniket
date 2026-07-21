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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	mdcomp "unstable.build/go-tui/component/markdown"
	mdhandler "unstable.build/go-tui/handler/markdown"
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
	lsp    semanticapi.LSP
	editor textapi.Editor
	wm     browserapi.WindowManager
	opener browserapi.ResourceOpener
	notify browserapi.Notifications
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
	res, err := execRequest[*hoverResult](ctx, c.lsp, "textDocument/hover", posParams(cmd))
	if err != nil {
		return err
	}
	if res == nil || strings.TrimSpace(res.Contents.Value) == "" {
		_, _ = c.notify.Notify(browserapi.LevelInfo, "No hover information at the cursor")
		return nil
	}
	if err := c.showMarkdown(res.Contents.Value); err != nil {
		return err
	}
	links := flattenHoverActions(res.Actions)
	if len(links) == 0 {
		return nil
	}
	return c.pickAction(ctx, cmd.URI, links)
}

// showMarkdown floats a scrollable, dismiss-on-key markdown view of the
// hover documentation, reusing the same rendering stack as `lsp hover`.
func (c *hoverCmd) showMarkdown(value string) error {
	comp, err := mdcomp.NewWithConfig(value, mdcomp.DefaultConfig())
	if err != nil {
		return fmt.Errorf("render hover markdown: %w", err)
	}
	mdh := mdhandler.New(comp)
	span := handler.NewSpan(mdh, component.SpanConfig{
		PadHorizontal:    2,
		ContentAlignment: component.AlignmentCentered,
	})
	var win browserapi.Window
	floating := browserapi.FuncFloatingHandler(span, func() error {
		defer c.wm.CloseWindow(win) //nolint:errcheck
		return mdh.Close()
	})
	w, err := c.wm.Floating(floating, browserapi.FloatingConfig{
		Alignment: component.AlignmentCentered,
	})
	if err != nil {
		return fmt.Errorf("show hover: %w", err)
	}
	win = w
	return nil
}

// pickAction floats a picker of the available hover actions and dispatches
// the chosen one.
func (c *hoverCmd) pickAction(
	ctx context.Context, base workspaceapi.URI, links []hoverCommandLink,
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
		return c.dispatch(ctx, base, links[idx])
	case <-ctx.Done():
		return ctx.Err()
	}
}

// dispatch performs the client-side action for a hover command link.
// gotoLocation and showReferences navigate the buffer; runSingle/debugSingle
// execute the carried runnable (debug lacks an adapter, so it runs like run).
func (c *hoverCmd) dispatch(
	ctx context.Context, base workspaceapi.URI, link hoverCommandLink,
) error {
	switch link.Command {
	case "rust-analyzer.gotoLocation":
		loc, ok := parseGotoLocation(link.Arguments)
		if !ok {
			_, _ = c.notify.Notify(browserapi.LevelInfo, "No location for %q", link.Title)
			return nil
		}
		return openLocation(c.editor, c.wm, c.opener, base, loc)
	case "rust-analyzer.showReferences":
		locs := parseShowReferences(link.Arguments)
		if len(locs) == 0 {
			_, _ = c.notify.Notify(browserapi.LevelInfo, "No references for %q", link.Tooltip)
			return nil
		}
		if len(locs) == 1 {
			return openLocation(c.editor, c.wm, c.opener, base, locs[0])
		}
		return c.showLocations(locs)
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

// showLocations floats a read-only viewer listing reference locations.
func (c *hoverCmd) showLocations(locs []semanticapi.Location) error {
	var b strings.Builder
	for _, l := range locs {
		fmt.Fprintf(&b, "%s:%d:%d\n",
			trimFileURI(l.URI), l.Range.Start.Line+1, l.Range.Start.Character+1)
	}
	if _, err := c.wm.Floating(newTextView(b.String()), browserapi.FloatingConfig{
		Alignment: component.AlignmentCentered,
	}); err != nil {
		return fmt.Errorf("show references: %w", err)
	}
	return nil
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
