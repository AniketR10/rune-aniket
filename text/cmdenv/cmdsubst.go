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
