// Copyright (C) 2017-2026 Unstable Build, LLC
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
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"mvdan.cc/sh/v3/syntax"
)

// CommandSubstResolver is a vte.CommandExpander that resolves every
// $(...) and `...` node in a shell line via a Runner backed by
// Executor, splices the captured stdout back into the line, and
// leaves everything else (pipes, redirects, $VAR, etc.) for the
// pty's $SHELL to evaluate.
type CommandSubstResolver struct {
	Executor  schemeapi.Executor
	EnvSource Source
}

// NewCommandSubstResolver returns a CommandSubstResolver suitable for
// vte.Config.CommandExpander.
func NewCommandSubstResolver(
	exe schemeapi.Executor, envSource Source,
) CommandSubstResolver {
	return CommandSubstResolver{Executor: exe, EnvSource: envSource}
}

// ExpandCommand satisfies vte.CommandExpander.
func (r CommandSubstResolver) ExpandCommand(
	ctx context.Context, line string,
) (string, error) {
	return expandCommandSubst(ctx, r.Executor, r.EnvSource, line)
}

func expandCommandSubst(
	ctx context.Context, exe schemeapi.Executor,
	envSource Source, line string,
) (string, error) {
	for {
		parsed, err := syntax.NewParser().Parse(strings.NewReader(line), "")
		if err != nil {
			return "", fmt.Errorf("parse shell line %q: %w", line, err)
		}

		// Resolve one rightmost innermost CmdSubst per pass and
		// reparse. O(N*depth) keeps offset bookkeeping simple in the
		// face of arbitrary nesting.
		type cmdsubst struct {
			start, end int
			node       *syntax.CmdSubst
		}
		var nodes []cmdsubst
		syntax.Walk(parsed, func(n syntax.Node) bool {
			if n == nil {
				return true
			}
			if cs, ok := n.(*syntax.CmdSubst); ok {
				nodes = append(nodes, cmdsubst{
					start: int(cs.Pos().Offset()),
					end:   int(cs.End().Offset()),
					node:  cs,
				})
			}
			return true
		})
		if len(nodes) == 0 {
			return line, nil
		}

		sort.Slice(nodes, func(i, j int) bool {
			if nodes[i].start != nodes[j].start {
				return nodes[i].start > nodes[j].start
			}
			return nodes[i].end < nodes[j].end
		})

		var target cmdsubst
		found := false
		for _, n := range nodes {
			hasInner := false
			syntax.Walk(n.node, func(c syntax.Node) bool {
				if c == nil || c == n.node {
					return true
				}
				if _, ok := c.(*syntax.CmdSubst); ok {
					hasInner = true
					return false
				}
				return true
			})
			if !hasInner {
				target = n
				found = true
				break
			}
		}
		if !found {
			target = nodes[0]
		}

		var inner bytes.Buffer
		printer := syntax.NewPrinter()
		for i, stmt := range target.node.Stmts {
			if i > 0 {
				inner.WriteString("\n")
			}
			if err := printer.Print(&inner, stmt); err != nil {
				return "", fmt.Errorf("print cmdsubst body: %w", err)
			}
		}

		var stdout, stderr bytes.Buffer
		runner := Runner{
			Executor:  exe,
			EnvSource: envSource,
			Stdout:    &stdout,
			Stderr:    &stderr,
		}
		if _, err := runner.Run(ctx, inner.String(), nil); err != nil {
			return "", err
		}

		// POSIX: strip trailing newlines from $(...).
		captured := strings.TrimRight(stdout.String(), "\n")
		line = line[:target.start] + captured + line[target.end:]
	}
}
