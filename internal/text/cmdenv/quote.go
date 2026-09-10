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
	"strings"

	shsyntax "mvdan.cc/sh/v3/syntax"
)

// Quote returns a POSIX-shell-safe rendering of v suitable for
// substitution into a shell line. Values without shell metacharacters
// round-trip unchanged. Values that the bash quoter rejects (e.g.
// embedded NULs) fall back to a single-quoted form.
func Quote(v string) string {
	q, err := shsyntax.Quote(v, shsyntax.LangBash)
	if err != nil {
		return "'" + strings.ReplaceAll(v, "'", `'"'"'`) + "'"
	}
	return q
}

// EscapeDoubleDollar rewrites "$$" to "\$" so the downstream
// shell.Expand emits a literal '$' instead of the parent process
// PID.
func EscapeDoubleDollar(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	n := len(s)
	for i < n {
		if i+1 < n && s[i] == '$' && s[i+1] == '$' {
			b.WriteString("\\$")
			i += 2
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// QuoteForShellFields wraps s in double quotes when it contains any
// character that mvdan.cc/sh/v3/shell.Fields would otherwise
// re-tokenise, expand, or interpret. The empty string is returned as
// `""` so it survives shell tokenisation as a single empty field.
func QuoteForShellFields(s string) string {
	if s == "" {
		return `""`
	}
	if !needsShellFieldsQuote(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' || c == '"' || c == '`' {
			b.WriteByte('\\')
		}
		b.WriteByte(c)
	}
	b.WriteByte('"')
	return b.String()
}

// QuoteArgsForShellFields applies QuoteForShellFields to every arg
// in args. Use it before joining dispatched argv into a shell line
// that will be re-tokenised by shell.Fields.
func QuoteArgsForShellFields(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = QuoteForShellFields(a)
	}
	return out
}

func needsShellFieldsQuote(s string) bool {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ', '\t', '\n', '\r',
			'\\', '"', '\'', '`',
			'|', '&', ';', '(', ')', '<', '>',
			'*', '?', '[', ']', '#', '~', '=':
			return true
		}
	}
	return false
}
