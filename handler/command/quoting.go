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

package command

import (
	"strings"

	shsyntax "mvdan.cc/sh/v3/syntax"
)

// SplitCommandLine tokenises a command-prompt buffer like a POSIX shell would,
// honouring backslash escapes, single-quoted ('…'), and double-quoted ("…")
// regions. Tokens retain their literal escape and quote characters so that the
// buffer can be round-tripped via strings.Join(tokens, " ") without information
// loss. Use UnquoteToken to obtain the unescaped value of a single token.
//
// A trailing dangling backslash or an unclosed quote are treated as if the
// remainder of the buffer were inside the open region; this keeps the live
// prompt usable while the user is still typing.
//
// p is reset and reused; pass a single per-handler parser to avoid
// re-allocating one on every call.
func SplitCommandLine(p *shsyntax.Parser, s string) []string {
	var tokens []string
	lastEnd := 0
	for w, err := range p.WordsSeq(strings.NewReader(s)) {
		if err != nil {
			// Stop after the first unparseable region; the trailing
			// fallback below captures it as one final token.
			break
		}
		start := int(w.Pos().Offset())
		end := int(w.End().Offset())
		tokens = append(tokens, s[start:end])
		lastEnd = end
	}
	// Capture any trailing region the parser couldn't consume (open
	// quote, dangling backslash, unfinished word) as a single token so
	// the live prompt remains usable while the user is still typing.
	rest := strings.TrimLeft(s[lastEnd:], " \t")
	if rest != "" {
		tokens = append(tokens, rest)
	}
	return tokens
}

// UnquoteToken removes the shell-style backslash escapes and single/double
// quote groupings produced by SplitCommandLine, returning the literal argument
// value that handlers should receive. Unclosed quotes and dangling backslashes
// are tolerated: their contents are returned verbatim minus the opening
// delimiter, matching the lenient behaviour of SplitCommandLine itself.
//
// p is reset and reused; pass a single per-handler parser to avoid
// re-allocating one on every call.
func UnquoteToken(p *shsyntax.Parser, s string) string {
	var b strings.Builder
	consumed := 0
	for w, err := range p.WordsSeq(strings.NewReader(s)) {
		if err != nil {
			break
		}
		for _, part := range w.Parts {
			writeUnquotedPart(&b, part)
		}
		consumed = int(w.End().Offset())
	}
	// Lenient tail handling: drop any unmatched opening quote and emit
	// the rest verbatim minus the leading delimiter. This matches the
	// "still typing" semantics of SplitCommandLine.
	tail := strings.TrimLeft(s[consumed:], " \t")
	switch {
	case strings.HasPrefix(tail, "'"), strings.HasPrefix(tail, `"`):
		b.WriteString(tail[1:])
	case strings.HasSuffix(tail, `\`):
		b.WriteString(tail[:len(tail)-1])
	default:
		b.WriteString(tail)
	}
	return b.String()
}

// writeUnquotedPart writes the literal value of a parsed shell word part,
// stripping backslash escapes and quote groupings. Unsupported parts (e.g.
// `$var`) are re-printed verbatim via the syntax printer.
func writeUnquotedPart(b *strings.Builder, node shsyntax.WordPart) {
	switch p := node.(type) {
	case *shsyntax.Lit:
		writeUnquotedLit(b, p.Value, false)
	case *shsyntax.SglQuoted:
		b.WriteString(p.Value)
	case *shsyntax.DblQuoted:
		for _, part := range p.Parts {
			if lit, ok := part.(*shsyntax.Lit); ok {
				writeUnquotedLit(b, lit.Value, true)
				continue
			}
			_ = shsyntax.NewPrinter().Print(b, part)
		}
	default:
		_ = shsyntax.NewPrinter().Print(b, node)
	}
}

func writeUnquotedLit(b *strings.Builder, s string, inDoubleQuotes bool) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' {
			b.WriteByte(c)
			continue
		}
		if i+1 >= len(s) {
			// Trailing dangling backslash (still-typing); drop it
			// rather than passing it through, so partial completions
			// see a clean prefix.
			continue
		}
		next := s[i+1]
		if inDoubleQuotes && next != '"' && next != '\\' {
			b.WriteByte(c)
			continue
		}
		b.WriteByte(next)
		i++
	}
}
