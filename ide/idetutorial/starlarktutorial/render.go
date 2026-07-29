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

package starlarktutorial

import (
	"fmt"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"

	"unstable.build/go-tui/handler/command"
)

// overlayComponent draws the active step's overlay in local
// coordinates so a single [component.Virtual] both positions the box
// on screen and answers ComponentAt hit-tests against the same
// geometry the last Draw painted.
type overlayComponent struct {
	width, height int
	draw          func(w term.Writer)
}

// Resize satisfies tui.Component.
func (c *overlayComponent) Resize(width, height int) {
	c.width, c.height = width, height
}

// Draw satisfies tui.Component.
func (c *overlayComponent) Draw(w term.Writer) {
	if c.draw == nil {
		return
	}
	c.draw(w)
}

// drawBanner paints lines top-left into v within (width, height).
// Out-of-range writes are dropped silently.
func drawBanner(
	v *component.Virtual[*overlayComponent], w term.Writer,
	width, height int, lines []string,
) {
	if width <= 0 || height <= 0 {
		return
	}
	maxW := 0
	for _, line := range lines {
		maxW = max(maxW, min(len(line), width))
	}
	v.Move(term.Coordinates{})
	v.Resize(maxW, min(len(lines), height))
	v.C.draw = func(lw term.Writer) {
		for y, line := range lines {
			for x, r := range line {
				lw.SetCell(term.Coordinates{X: x, Y: y}, term.Cell{Ch: r})
			}
		}
	}
	v.Draw(w)
}

// commandPromptTopFraction mirrors the command prompt's vertical
// anchor: the IDE opens the prompt at Y = 0.2 * height (see
// (*ex).newCommandPrompt). The bottom-anchored wait hint window caps
// its height against that row so it never grows up into the prompt the
// user is asked to open.
const commandPromptTopFraction = 0.2

// hintBoxMinInnerH keeps the hint window tall enough to show its
// wrapped instruction line even on short screens, where the
// prompt-aware cap (0.2*height) would otherwise squeeze it below
// readability.
const hintBoxMinInnerH = 4

// hintBoxMaxHeightSlack lets the hint window extend a few rows past the
// prompt-aware cap before truncating its body, so longer command
// manuals are not cut off mid-sentence.
const hintBoxMaxHeightSlack = 3

// buildWaitCommandHint composes the markdown body for the
// wait_command hint window. The body always opens with a one-line
// prompt — "Waiting for you to open the command prompt `<cmd>` and
// try the `<command>` command:" — followed by the command's
// markdown-rendered manual when lookup returns one. A non-empty
// request.text wins over the manual and is rendered verbatim after
// the prompt opener: it is either the step's own instruction or the
// on_error message the runtime swapped in after a failed dispatch.
func buildWaitCommandHint(
	r *request, cmdKey string, lookup CommandManualLookup,
	keyForCommand func(cmd string, args []string) string,
) string {
	if r == nil {
		return ""
	}
	cmdName := r.command
	var b strings.Builder
	fmt.Fprintf(&b,
		"Waiting for you to open the command prompt `%s` and try the `%s` command:\n\n",
		cmdKey, cmdName)
	if boundKey := waitCommandBoundKey(cmdName, keyForCommand); boundKey != "" &&
		!strings.Contains(r.text, "`"+boundKey+"`") {
		fmt.Fprintf(&b,
			"Or you can press `%s` to run it.\n\n", boundKey)
	}
	if r.text != "" {
		// Author-supplied markdown, rendered as-is.
		b.WriteString(expandCmdTemplate(r.text, cmdKey))
		b.WriteString("\n")
		return b.String()
	}
	if lookup != nil {
		if man, ok := lookup(commandName(cmdName)); ok {
			b.WriteString(renderCommandManual(man))
			return b.String()
		}
	}
	return b.String()
}

// waitCommandBoundKey resolves the pretty key spec bound to the
// awaited command (the first token of cmd is the command name and the
// rest are arguments). Returns "" when no resolver is wired or the
// command has no binding.
func waitCommandBoundKey(
	cmd string, keyForCommand func(name string, args []string) string,
) string {
	if keyForCommand == nil {
		return ""
	}
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return ""
	}
	return keyForCommand(fields[0], fields[1:])
}

// commandName is the command name of an awaited, possibly argument-
// qualified wait_command spec ("! git log" -> "!").
func commandName(cmd string) string {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return cmd
	}
	return fields[0]
}

// buildWaitShellHint composes the markdown body for a wait_shell hint
// window: it asks the user to run the expected command inside Rune's
// console. A non-empty request.text is rendered verbatim after the
// prompt opener: it is either the step's own instruction or the
// on_error message the runtime swapped in after a wrong command.
func buildWaitShellHint(r *request, cmdKey string) string {
	if r == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b,
		"Waiting for you to run `%s` in Rune's console:\n\n",
		strings.Join(r.shellArgs, " "))
	if r.text != "" {
		b.WriteString(expandCmdTemplate(r.text, cmdKey))
		b.WriteString("\n")
	}
	return b.String()
}

// renderCommandManual returns a compact, markdown summary of man
// suitable for embedding in the wait_command hint window. The
// layout intentionally mirrors what handler/command renders in its
// own manual overlay so the user sees the same shape they would in
// the live prompt.
func renderCommandManual(man command.Manual) string {
	var b strings.Builder
	if man.Synopsis != "" {
		fmt.Fprintf(&b, "**Usage:** `%s %s`\n\n", man.Name, man.Synopsis)
	} else {
		fmt.Fprintf(&b, "**Usage:** `%s`\n\n", man.Name)
	}
	if len(man.AliasOf) > 0 {
		if len(man.AliasOf) == 1 {
			fmt.Fprintf(&b, "Alias of `%s`.\n\n", man.AliasOf[0])
		} else {
			b.WriteString("Alias of the following sequence of commands:\n\n")
			for _, c := range man.AliasOf {
				fmt.Fprintf(&b, "- `%s`\n", c)
			}
			b.WriteString("\n")
		}
	}
	if man.Summary != "" {
		b.WriteString(man.Summary)
		b.WriteString("\n")
	}
	if len(man.Commands) > 0 {
		b.WriteString("\n**Subcommands:**\n\n")
		for _, sub := range man.Commands {
			if sub.Summary != "" {
				fmt.Fprintf(&b, "- `%s` — %s\n", sub.Name, sub.Summary)
			} else {
				fmt.Fprintf(&b, "- `%s`\n", sub.Name)
			}
		}
	}
	return b.String()
}

// expandCmdTemplate replaces every <cmd> token in s with the
// already-prettified command-key display string.
func expandCmdTemplate(s, cmdKey string) string {
	return strings.ReplaceAll(s, "<cmd>", cmdKey)
}

// prettyKeySpecs maps a rendered key spec to a friendlier form for
// tutorial copy. It is display-only and never affects key matching.
var prettyKeySpecs = map[string]string{"<shift-;>": ":"}

// PrettyKeySpec rewrites a rendered key spec to its tutorial-friendly
// form (e.g. "<shift-;>" becomes ":"). Unmapped specs pass through
// unchanged. Used only for display in tutorial copy.
func PrettyKeySpec(s string) string {
	if p, ok := prettyKeySpecs[s]; ok {
		return p
	}
	return s
}
