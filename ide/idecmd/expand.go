// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package idecmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"unstable.build/go-tui/handler/command"
	"unstable.build/go-tui/text/cmdenv"
)

// expandTarget expands one alias target (a single step body) into an
// argv. base provides FILE/WORD/builtin/user values; positional $1..$9
// and chain captures overlay on top.
func expandTarget(
	ctx context.Context,
	target string,
	args []string,
	base cmdenv.Source,
	chain *Chain,
) (argv []string, referenced map[int]struct{}, err error) {
	env := overlay(args, base, chain)
	if head, rest, ok := splitPluginPrefix(target); ok {
		referenced = make(map[int]struct{})
		if err := checkReferences(rest, len(args)); err != nil {
			return nil, nil, err
		}
		recordReferences(rest, len(args), referenced)
		envOSFallback := cmdenv.Source(func(name string) (string, bool) {
			if v, ok := env(name); ok {
				return v, true
			}
			if v, ok := os.LookupEnv(name); ok {
				return v, true
			}
			return "", false
		})
		expanded := cmdenv.ExpandBody(
			cmdenv.WithCommandSubstitution(ctx), rest, envOSFallback)
		return []string{head, expanded}, referenced, nil
	}
	tokens := command.SplitCommandLine(target)
	argv = make([]string, 0, len(tokens))
	referenced = make(map[int]struct{})
	for _, tok := range tokens {
		raw := command.UnquoteToken(tok)
		if err := checkReferences(raw, len(args)); err != nil {
			return nil, nil, err
		}
		expanded, expErr := cmdenv.Expand(ctx, cmdenv.EscapeDoubleDollar(raw), env)
		if expErr != nil {
			return nil, nil, fmt.Errorf(
				"expand alias target token %q: %v", raw, expErr)
		}
		recordReferences(raw, len(args), referenced)
		argv = append(argv, expanded)
	}
	return argv, referenced, nil
}

// overlay returns a cmdenv.Source that resolves $1..$9 against args,
// then chain captures, then delegates to base.
func overlay(args []string, base cmdenv.Source, chain *Chain) cmdenv.Source {
	return func(name string) (string, bool) {
		if len(name) == 1 && name[0] >= '1' && name[0] <= '9' {
			idx := int(name[0] - '1')
			if idx < len(args) {
				return args[idx], true
			}
			return "", false
		}
		if v, ok := chain.Get(name); ok {
			return v, true
		}
		if base != nil {
			return base(name)
		}
		return "", false
	}
}

// splitPluginPrefix returns ok=false when `!` is not a standalone
// token, so bodies like `!foo` (no whitespace) stay as ordinary
// command names.
func splitPluginPrefix(target string) (head, rest string, ok bool) {
	trimmed := strings.TrimLeft(target, " \t")
	switch {
	case strings.HasPrefix(trimmed, "!! "), strings.HasPrefix(trimmed, "!!\t"):
		return "!!", strings.TrimLeft(trimmed[2:], " \t"), true
	case strings.HasPrefix(trimmed, "! "), strings.HasPrefix(trimmed, "!\t"):
		return "!", strings.TrimLeft(trimmed[1:], " \t"), true
	}
	return "", "", false
}

// isPluginTarget reports whether target is a `!` or `!!` shell body.
// Used by callers that need to decide whether to opt the ctx into
// cmdenv command-substitution preservation before re-dispatching.
func isPluginTarget(target string) bool {
	_, _, ok := splitPluginPrefix(target)
	return ok
}
