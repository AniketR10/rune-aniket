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

package command

import (
	"strings"
)

// SplitCommandLine tokenises a command-prompt buffer using a deliberately
// minimal grammar: ASCII whitespace separates tokens, `\` escapes the
// following byte, single quotes ('…') wrap a literal region with no
// escapes, and double quotes ("…") wrap a region in which only `\"`,
// `\\`, and `\<newline>` are escapes (every other `\<c>` is two literal
// bytes). No other characters are special; in particular, `<`, `>`,
// `|`, `{`, `}`, `&`, `;`, `(`, `)`, `$`, `~`, `*`, `?`, and `#` are all
// returned verbatim. This is intentionally narrower than POSIX shell —
// any per-command argv grammar (echo's `<…>` keys, searchast's `|`
// separator, …) lives in Layer 3 and is unaffected by Layer 1.
//
// Tokens retain their literal escape and quote characters so the buffer
// can be round-tripped (modulo separator collapsing) via the prompt;
// use UnquoteToken to obtain the unescaped value of a single token.
//
// A trailing dangling backslash or an unclosed quote are treated as if
// the remainder of the buffer were inside the open region; this keeps
// the live prompt usable while the user is still typing.
func SplitCommandLine(s string) []string {
	var tokens []string
	i := 0
	n := len(s)
	for i < n {
		// skip leading separators
		for i < n && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
		if i >= n {
			break
		}
		start := i
		state := stateOutside
	tokLoop:
		for i < n {
			c := s[i]
			switch state {
			case stateOutside:
				switch c {
				case ' ', '\t':
					break tokLoop
				case '\\':
					if i+1 >= n {
						// dangling backslash: keep it as part of the token
						i++
						break tokLoop
					}
					i += 2
				case '\'':
					state = stateSingle
					i++
				case '"':
					state = stateDouble
					i++
				default:
					i++
				}
			case stateSingle:
				if c == '\'' {
					state = stateOutside
				}
				i++
			case stateDouble:
				if c == '\\' && i+1 < n {
					next := s[i+1]
					if next == '"' || next == '\\' || next == '\n' {
						i += 2
						continue
					}
					i++
					continue
				}
				if c == '"' {
					state = stateOutside
				}
				i++
			}
		}
		tokens = append(tokens, s[start:i])
	}
	return tokens
}

// UnquoteToken removes the Layer 1 backslash escapes and single/double
// quote groupings produced by SplitCommandLine, returning the literal
// argument value that handlers should receive. Unclosed quotes and
// dangling backslashes are tolerated: their contents are returned
// verbatim minus the opening delimiter, matching the lenient
// "still typing" semantics of SplitCommandLine itself.
func UnquoteToken(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	n := len(s)
	for i < n {
		c := s[i]
		switch c {
		case '\\':
			if i+1 >= n {
				// dangling backslash: drop it
				i++
				continue
			}
			b.WriteByte(s[i+1])
			i += 2
		case '\'':
			i++
			for i < n && s[i] != '\'' {
				b.WriteByte(s[i])
				i++
			}
			if i < n {
				i++ // skip closing '
			}
		case '"':
			i++
			for i < n && s[i] != '"' {
				if s[i] == '\\' && i+1 < n {
					next := s[i+1]
					if next == '"' || next == '\\' {
						b.WriteByte(next)
						i += 2
						continue
					}
					if next == '\n' {
						i += 2
						continue
					}
					// every other \<c> is two literal bytes (POSIX)
					b.WriteByte('\\')
					b.WriteByte(next)
					i += 2
					continue
				}
				b.WriteByte(s[i])
				i++
			}
			if i < n {
				i++ // skip closing "
			}
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

// LastTokenIsIncomplete reports whether s ends inside an open
// single-quoted region, an open double-quoted region, or with an
// odd-length run of trailing backslashes. In any of those cases the
// next typed space should stay inside the current token instead of
// splitting arguments.
func LastTokenIsIncomplete(s string) bool {
	state := stateOutside
	i := 0
	n := len(s)
	for i < n {
		c := s[i]
		switch state {
		case stateOutside:
			switch c {
			case '\\':
				if i+1 >= n {
					return true
				}
				i += 2
			case '\'':
				state = stateSingle
				i++
			case '"':
				state = stateDouble
				i++
			default:
				i++
			}
		case stateSingle:
			if c == '\'' {
				state = stateOutside
			}
			i++
		case stateDouble:
			if c == '\\' && i+1 < n {
				next := s[i+1]
				if next == '"' || next == '\\' || next == '\n' {
					i += 2
					continue
				}
				i++
				continue
			}
			if c == '"' {
				state = stateOutside
			}
			i++
		}
	}
	return state != stateOutside
}

// ShellQuote wraps s such that re-tokenising the result via
// SplitCommandLine + UnquoteToken yields s unchanged as a single
// argument. Strings that contain no Layer 1 metacharacters and are
// non-empty are returned as-is.
func ShellQuote(s string) string {
	if s == "" {
		return `''`
	}
	if !needsShellQuote(s) {
		return s
	}
	if !strings.ContainsRune(s, '\'') {
		return "'" + s + "'"
	}
	// fall back to double-quoting so we can keep embedded single
	// quotes verbatim. Inside double quotes only `\` and `"` need
	// escaping under Layer 1.
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' || c == '"' {
			b.WriteByte('\\')
		}
		b.WriteByte(c)
	}
	b.WriteByte('"')
	return b.String()
}

func needsShellQuote(s string) bool {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ', '\t', '\\', '\'', '"':
			return true
		}
	}
	return false
}

type quotingState uint8

const (
	stateOutside quotingState = iota
	stateSingle
	stateDouble
)
