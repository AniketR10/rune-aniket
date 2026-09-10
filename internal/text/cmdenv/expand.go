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

package cmdenv

import (
	"context"
	"strings"

	"mvdan.cc/sh/v3/shell"
)

// Expand performs POSIX-style parameter and arithmetic expansion on
// s as a single double-quoted field. Variable lookups consult src
// first and fall back to os.Getenv. The result never contains word
// splitting or globbing.
//
// When AllowsCommandSubstitution(ctx) is true, $(...) and backtick
// spans are copied through verbatim and the caller is responsible
// for evaluating them downstream. Otherwise they are rejected.
func Expand(ctx context.Context, s string, src Source) (string, error) {
	if !AllowsCommandSubstitution(ctx) {
		return shell.Expand(s, Lookup(src))
	}
	var out strings.Builder
	out.Grow(len(s))
	lookup := Lookup(src)
	i := 0
	n := len(s)
	for i < n {
		if s[i] == '\\' && i+1 < n {
			i += 2
			continue
		}
		if s[i] == '`' {
			end := findBacktickEnd(s, i+1)
			if end < 0 {
				break
			}
			if err := expandSegment(&out, s[:i], lookup); err != nil {
				return "", err
			}
			out.WriteString(s[i : end+1])
			s = s[end+1:]
			i = 0
			n = len(s)
			continue
		}
		if s[i] == '$' && i+1 < n && s[i+1] == '(' {
			end := findCmdSubstEnd(s, i+2)
			if end < 0 {
				break
			}
			if err := expandSegment(&out, s[:i], lookup); err != nil {
				return "", err
			}
			out.WriteString(s[i : end+1])
			s = s[end+1:]
			i = 0
			n = len(s)
			continue
		}
		i++
	}
	if err := expandSegment(&out, s, lookup); err != nil {
		return "", err
	}
	return out.String(), nil
}

func expandSegment(out *strings.Builder, seg string, lookup func(string) string) error {
	if seg == "" {
		return nil
	}
	v, err := shell.Expand(seg, lookup)
	if err != nil {
		return err
	}
	out.WriteString(v)
	return nil
}

// findCmdSubstEnd scans s for the ')' that closes a $(...) span
// whose body starts at start. Nested $(...) is treated as one outer
// span. Returns -1 on no match.
func findCmdSubstEnd(s string, start int) int {
	depth := 1
	i := start
	n := len(s)
	for i < n {
		switch s[i] {
		case '\\':
			if i+1 < n {
				i += 2
				continue
			}
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		case '$':
			if i+1 < n && s[i+1] == '(' {
				depth++
				i += 2
				continue
			}
		}
		i++
	}
	return -1
}

// findBacktickEnd scans s for the closing backtick of a span that
// started at start-1, honoring `\“ escapes. Returns -1 on no match.
func findBacktickEnd(s string, start int) int {
	i := start
	n := len(s)
	for i < n {
		if s[i] == '\\' && i+1 < n {
			i += 2
			continue
		}
		if s[i] == '`' {
			return i
		}
		i++
	}
	return -1
}

// ExpandBody substitutes Rune-style $NAME, ${NAME}, and $N references
// in a plugin (! / !!) body using env, then returns the result.
// $(...) command substitutions, backtick spans, and backslash escapes
// are copied through untouched for the downstream shell interpreter
// to resolve. $$ collapses to a literal $. Resolved values are
// shell-quoted via Quote so values with whitespace or metacharacters
// remain a single field. Names absent from env pass through verbatim
// so the downstream shell can consult its own environment.
//
// ctx is accepted for symmetry with Expand and is currently unused.
func ExpandBody(_ context.Context, s string, env Source) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	n := len(s)
	for i < n {
		c := s[i]
		if c == '\\' && i+1 < n {
			b.WriteByte(c)
			b.WriteByte(s[i+1])
			i += 2
			continue
		}
		if c != '$' {
			b.WriteByte(c)
			i++
			continue
		}
		if i+1 < n && s[i+1] == '$' {
			b.WriteByte('$')
			i += 2
			continue
		}
		if i+1 < n && s[i+1] == '{' {
			j := i + 2
			for j < n && s[j] != '}' {
				j++
			}
			name := s[i+2 : j]
			if v, ok := envLookup(env, name); ok {
				b.WriteString(Quote(v))
			} else {
				b.WriteString(s[i : j+1])
			}
			if j < n {
				i = j + 1
			} else {
				i = j
			}
			continue
		}
		if i+1 < n && isDollarHead(s[i+1]) {
			j := i + 2
			if s[i+1] < '0' || s[i+1] > '9' {
				for j < n && isDollarTail(s[j]) {
					j++
				}
			}
			name := s[i+1 : j]
			if v, ok := envLookup(env, name); ok {
				b.WriteString(Quote(v))
			} else {
				b.WriteString(s[i:j])
			}
			i = j
			continue
		}
		b.WriteByte('$')
		i++
	}
	return b.String()
}

func envLookup(env Source, name string) (string, bool) {
	if env == nil {
		return "", false
	}
	return env(name)
}

func isDollarHead(b byte) bool {
	return (b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		b == '_' ||
		(b >= '0' && b <= '9')
}

func isDollarTail(b byte) bool {
	return (b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') ||
		b == '_'
}
