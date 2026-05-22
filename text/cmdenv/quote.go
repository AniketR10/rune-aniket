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
