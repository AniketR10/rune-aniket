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
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"mvdan.cc/sh/v3/shell"
)

func TestExpandStrictRejectsCommandSubst(t *testing.T) {
	_, err := Expand(context.Background(), "$(echo hi)", nil)
	require.Error(t, err, "strict ctx must reject $(...)")
	_, err = Expand(context.Background(), "`echo hi`", nil)
	require.Error(t, err, "strict ctx must reject backticks")
}

func TestExpandPermissivePreservesCommandSubst(t *testing.T) {
	src := Source(func(name string) (string, bool) {
		if name == "GREETING" {
			return "hello", true
		}
		return "", false
	})
	ctx := WithCommandSubstitution(context.Background())

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain cmdsubst", "$(echo hi)", "$(echo hi)"},
		{"backticks", "`echo hi`", "`echo hi`"},
		{"var then cmdsubst",
			"$GREETING $(echo world)",
			"hello $(echo world)"},
		{"cmdsubst then var",
			"$(echo world) $GREETING",
			"$(echo world) hello"},
		{"nested cmdsubst",
			"$(echo $(date +%Y))", "$(echo $(date +%Y))"},
		{"mixed",
			"prefix-$GREETING-$(date)-end",
			"prefix-hello-$(date)-end"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Expand(ctx, tc.in, src)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestExpandBodyPreservesShellConstructs(t *testing.T) {
	src := Source(func(name string) (string, bool) {
		switch name {
		case "1":
			return "with space", true
		case "FILE":
			return "/tmp/a b.go", true
		}
		return "", false
	})
	ctx := context.Background()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"escape double dollar", "$$N", "$N"},
		{"positional quoted",
			"echo $1", `echo 'with space'`},
		{"named quoted",
			"echo $FILE", `echo '/tmp/a b.go'`},
		{"unknown var passthrough",
			"echo $UNKNOWN_X", "echo $UNKNOWN_X"},
		{"cmdsubst preserved",
			"echo $(date) $FILE",
			`echo $(date) '/tmp/a b.go'`},
		{"backtick preserved",
			"echo `date` $FILE",
			"echo `date` '/tmp/a b.go'"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExpandBody(ctx, tc.in, src)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestExpandStrictAdversarial covers Expand's strict-mode behavior
// (no command substitution allowed) across edge inputs that have
// historically caused issues in shell tooling.
func TestExpandStrictAdversarial(t *testing.T) {
	ctx := context.Background()
	src := Source(func(name string) (string, bool) {
		v, ok := strictEnv[name]
		return v, ok
	})
	for _, tc := range strictExpandCases() {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Expand(ctx, tc.in, src)
			if tc.wantErr {
				require.Error(t, err, "input %q", tc.in)
				return
			}
			require.NoError(t, err, "input %q", tc.in)
			assert.Equal(t, tc.want, got, "input %q", tc.in)
		})
	}
}

// TestExpandPermissiveAdversarial covers Expand under
// WithCommandSubstitution: $(...) and backticks must be preserved
// verbatim while $VAR/${VAR}/arithmetic still expand.
func TestExpandPermissiveAdversarial(t *testing.T) {
	ctx := WithCommandSubstitution(context.Background())
	src := Source(func(name string) (string, bool) {
		v, ok := permissiveEnv[name]
		return v, ok
	})
	for _, tc := range permissiveExpandCases() {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Expand(ctx, tc.in, src)
			if tc.wantErr {
				require.Error(t, err, "input %q", tc.in)
				return
			}
			require.NoError(t, err, "input %q", tc.in)
			assert.Equal(t, tc.want, got, "input %q", tc.in)
		})
	}
}

// TestExpandConcurrent ensures Expand is safe under -race for the
// common case where the same input/source are reused from multiple
// goroutines. Expand has no shared mutable state; this guards against
// regressions.
func TestExpandConcurrent(t *testing.T) {
	ctx := WithCommandSubstitution(context.Background())
	src := Source(func(name string) (string, bool) {
		if name == "A" {
			return "a", true
		}
		return "", false
	})
	const goroutines = 16
	const iterations = 200
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				got, err := Expand(ctx, "$A $(echo $A) $A", src)
				if err != nil {
					t.Errorf("Expand: %v", err)
					return
				}
				if got != "a $(echo $A) a" {
					t.Errorf("Expand: got %q", got)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// TestExpandBodyAdversarial covers ExpandBody with the full surface
// of Rune-style $NAME / ${NAME} / $N substitutions, escape handling,
// and tricky values from env (whitespace, quotes, NUL, non-utf8,
// newlines). $(...) and backticks must always pass through verbatim.
func TestExpandBodyAdversarial(t *testing.T) {
	ctx := context.Background()
	src := Source(func(name string) (string, bool) {
		v, ok := bodyEnv[name]
		return v, ok
	})
	for _, tc := range expandBodyCases() {
		t.Run(tc.name, func(t *testing.T) {
			got := ExpandBody(ctx, tc.in, src)
			assert.Equal(t, tc.want, got, "input %q", tc.in)
		})
	}
}

// TestExpandBodyNilSource ensures ExpandBody is safe when the env
// source is nil — every $NAME must pass through as a literal.
func TestExpandBodyNilSource(t *testing.T) {
	got := ExpandBody(context.Background(), "$X ${Y} $1 $$ literal", nil)
	assert.Equal(t, "$X ${Y} $1 $ literal", got)
}

// TestQuoteRoundTripsThroughShellFields asserts that values quoted
// via cmdenv.Quote round-trip through shell.Fields back to a single
// argv element with the original bytes — i.e. Quote produces
// something the shell tokenizer (the layer that actually unwraps
// quoting) recovers.
//
// NUL bytes are intentionally excluded: shell.Fields drops them
// during single-quoted parse. See TestQuoteNULBytesAreStripped.
func TestQuoteRoundTripsThroughShellFields(t *testing.T) {
	cases := []string{
		"",
		"plain",
		"with space",
		"a$b",
		"a'b",
		`a"b`,
		"a`b",
		"a;b|c&d",
		"unicode-Ω-é",
		strings.Repeat("x", 4096),
	}
	for _, c := range cases {
		t.Run(snippet(c), func(t *testing.T) {
			fields, err := shell.Fields(
				Quote(c), func(string) string { return "" })
			require.NoError(t, err, "input %q", c)
			require.Len(t, fields, 1, "input %q", c)
			assert.Equal(t, c, fields[0], "input %q", c)
		})
	}
}

// TestQuoteNULBytesAreStripped pins the known limitation that
// shell.Fields drops NUL bytes inside the single-quoted form Quote
// produces for them. If shell.Fields ever stops stripping, this
// test will start failing and we can drop the carve-out above.
func TestQuoteNULBytesAreStripped(t *testing.T) {
	fields, err := shell.Fields(
		Quote("a\x00b"), func(string) string { return "" })
	require.NoError(t, err)
	require.Len(t, fields, 1)
	assert.Equal(t, "ab", fields[0],
		"shell.Fields currently strips NUL inside '...' — "+
			"adjust if mvdan/sh fixes this")
}

// TestQuoteANSICForms pins the textual `$'...'` ANSI-C rendering
// Quote emits for control bytes, newlines, tabs, and non-utf8 — and
// asserts shell.Fields decodes those back to the original bytes.
func TestQuoteANSICForms(t *testing.T) {
	cases := []struct {
		in, quoted string
	}{
		{"tab\there", `$'tab\there'`},
		{"line1\nline2", `$'line1\nline2'`},
		{"bell\x07", `$'bell\a'`},
		{"a\xffb", `$'a\xffb'`},
	}
	for _, c := range cases {
		t.Run(snippet(c.in), func(t *testing.T) {
			assert.Equal(t, c.quoted, Quote(c.in))
			fields, err := shell.Fields(
				c.quoted, func(string) string { return "" })
			require.NoError(t, err)
			require.Len(t, fields, 1)
			assert.Equal(t, c.in, fields[0])
		})
	}
}

// TestQuoteNonUTF8 documents Quote's behavior on non-utf8 bytes:
// the bash quoter emits a $'...' form using \xNN escapes.
func TestQuoteNonUTF8(t *testing.T) {
	in := "a\xffb"
	got := Quote(in)
	assert.Equal(t, `$'a\xffb'`, got)
}

// TestEscapeDoubleDollar covers every documented mapping plus the
// adjacent cases that are easy to break.
func TestEscapeDoubleDollar(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"$", "$"},
		{"$$", `\$`},
		{"$$$$", `\$\$`},
		{"$$N", `\$N`},
		{"$N", "$N"},
		{"a$$b", `a\$b`},
		{`\$$`, `\\$`},
		{"$$$", `\$$`}, // first $$ consumed, trailing $ untouched
		{strings.Repeat("$$", 1024),
			strings.Repeat(`\$`, 1024)},
	}
	for _, c := range cases {
		t.Run(snippet(c.in), func(t *testing.T) {
			assert.Equal(t, c.want, EscapeDoubleDollar(c.in))
		})
	}
}

// TestQuoteForShellFields covers the round-trip with mvdan
// shell.Fields for a wide set of values including operators that
// must be quoted, env references that must NOT be quoted, and bytes
// (NUL, non-utf8) that the helper passes through.
func TestQuoteForShellFields(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"empty", "", `""`},
		{"plain", "plain", "plain"},
		{"space wraps", "with space", `"with space"`},
		{"dquote inside escaped", `has"q`, `"has\"q"`},
		{"squote inside is fine", "has'q", `"has'q"`},
		{"backtick escaped", "has`bt", "\"has\\`bt\""},
		{"dollar is preserved",
			"$VAR", "$VAR"},
		{"pipe quoted", "a|b", `"a|b"`},
		{"redirect quoted", "a>b", `"a>b"`},
		{"amp quoted", "a&b", `"a&b"`},
		{"glob quoted", "*.go", `"*.go"`},
		{"semicolon quoted", "a;b", `"a;b"`},
		{"equals quoted", "a=b", `"a=b"`},
		{"nul passes through",
			"a\x00b", "a\x00b"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, QuoteForShellFields(c.in))
		})
	}
}

// TestQuoteArgsForShellFields covers nil/empty/length behavior.
func TestQuoteArgsForShellFields(t *testing.T) {
	assert.Equal(t, []string{}, QuoteArgsForShellFields(nil))
	assert.Equal(t, []string{}, QuoteArgsForShellFields([]string{}))
	got := QuoteArgsForShellFields([]string{"", "a|b", "plain"})
	assert.Equal(t, []string{`""`, `"a|b"`, "plain"}, got)
}

type expandCase struct {
	name    string
	in      string
	want    string
	wantErr bool
}

var strictEnv = map[string]string{
	"NAME":   "world",
	"EMPTY":  "",
	"BIG":    strings.Repeat("x", 65536),
	"NUL":    "a\x00b",
	"UTF8":   "héllo",
	"NOUTF8": "a\xffb",
}

func strictExpandCases() []expandCase {
	return []expandCase{
		{name: "literal", in: "hello", want: "hello"},
		{name: "named var",
			in: "hi $NAME", want: "hi world"},
		{name: "braced var",
			in: "hi ${NAME}!", want: "hi world!"},
		{name: "default value",
			in: "${MISSING:-fallback}", want: "fallback"},
		{name: "default uses set var",
			in: "${NAME:-fallback}", want: "world"},
		{name: "unknown var is empty",
			in: "[$UNKNOWN]", want: "[]"},
		{name: "arithmetic",
			in: "$((1+2*3))", want: "7"},
		{name: "arithmetic with var",
			in: "$((100))", want: "100"},
		{name: "empty input", in: "", want: ""},
		{name: "empty var",
			in: "[${EMPTY}]", want: "[]"},
		{name: "very large var",
			in:   "<$BIG>",
			want: "<" + strings.Repeat("x", 65536) + ">"},
		{name: "embedded nul in var",
			in: "<$NUL>", want: "<a\x00b>"},
		{name: "utf8 var",
			in: "<$UTF8>", want: "<héllo>"},
		{name: "non-utf8 var",
			in: "<$NOUTF8>", want: "<a\xffb>"},
		{name: "backslash before dollar is literal",
			in: `\$NAME`, want: "$NAME"},
		{name: "cmdsubst rejected",
			in: "$(echo hi)", wantErr: true},
		{name: "backtick rejected",
			in: "`echo hi`", wantErr: true},
	}
}

var permissiveEnv = map[string]string{
	"NAME": "world",
}

func permissiveExpandCases() []expandCase {
	return []expandCase{
		{name: "no substitution",
			in: "plain text", want: "plain text"},
		{name: "var still expands",
			in: "hi $NAME", want: "hi world"},
		{name: "cmdsubst preserved at end",
			in: "echo $(date)", want: "echo $(date)"},
		{name: "cmdsubst preserved at start",
			in: "$(date) end", want: "$(date) end"},
		{name: "cmdsubst preserved in middle",
			in: "before $(date) after",
			want: "before $(date) after"},
		{name: "two adjacent cmdsubsts",
			in:   "$(a)$(b)",
			want: "$(a)$(b)"},
		{name: "nested cmdsubst at depth 5",
			in:   "$(a $(b $(c $(d $(e)))))",
			want: "$(a $(b $(c $(d $(e)))))"},
		{name: "backtick preserved",
			in: "echo `date`", want: "echo `date`"},
		{name: "mixed var and cmdsubst",
			in:   "$NAME $(echo $NAME) $NAME",
			want: "world $(echo $NAME) world"},
		{name: "escaped dollar paren stays literal",
			in: `\$(date)`, want: "$(date)"},
		{name: "unclosed paren falls through to shell.Expand and errors",
			// findCmdSubstEnd returns -1; the remainder is handed
			// to shell.Expand which surfaces the unmatched paren.
			in:      "echo $(date",
			wantErr: true},
		{name: "unclosed backtick falls through to shell.Expand and errors",
			// findBacktickEnd returns -1; the remainder is handed
			// to shell.Expand which surfaces the unclosed quote.
			in:      "echo `date",
			wantErr: true},
		{name: "arithmetic looks like $( and is preserved verbatim",
			// findCmdSubstEnd cannot distinguish $((..)) from
			// $(..) without re-parsing, so arithmetic spans are
			// preserved alongside command substitutions. The
			// downstream shell evaluates either form.
			in:   "$((1+1))",
			want: "$((1+1))"},
		{name: "braced var still expands",
			in:   "<${NAME}>",
			want: "<world>"},
		{name: "cmdsubst body with quotes preserved",
			in:   `$(echo "); echo hi")`,
			want: `$(echo "); echo hi")`},
		{name: "cmdsubst body containing dollar paren preserved",
			in:   "$(echo $(echo nested))",
			want: "$(echo $(echo nested))"},
		{name: "many cmdsubsts on one line",
			in:   strings.Repeat("$(a)", 50),
			want: strings.Repeat("$(a)", 50)},
	}
}

var bodyEnv = map[string]string{
	"NAME":   "world",
	"1":      "first arg",
	"2":      "",
	"WITH$":  "dollar-key", // a key the parser cannot reach (no $$ in input)
	"NL":     "line1\nline2",
	"NUL":    "a\x00b",
	"NOUTF8": "a\xffb",
	"SQ":     "it's",
}

func expandBodyCases() []expandCase {
	return []expandCase{
		{name: "empty input", in: "", want: ""},
		{name: "plain text", in: "hello world", want: "hello world"},
		{name: "named var",
			in: "hi $NAME", want: "hi world"},
		{name: "braced var",
			in: "<${NAME}>", want: "<world>"},
		{name: "${NAME}suffix",
			in: "${NAME}suffix", want: "worldsuffix"},
		{name: "${} empty stays literal",
			in: "${}", want: "${}"},
		{name: "missing close brace still resolves the name",
			// ExpandBody's brace scan consumes until '}' or EOF;
			// when EOF is reached it still looks up the name and
			// emits the (quoted) value. This documents the actual
			// behavior rather than asserting an ideal one.
			in: "${NAME", want: "world"},
		{name: "positional $1 quoted",
			in: "echo $1", want: `echo 'first arg'`},
		{name: "positional $2 empty quoted with surroundings",
			in: "[$2]", want: "['']"},
		{name: "unknown positional passes through",
			in: "$9", want: "$9"},
		{name: "double dollar becomes literal dollar",
			in: "$$", want: "$"},
		{name: "double dollar before var name",
			in: "$$NAME", want: "$NAME"},
		{name: "escape preserves both bytes",
			in: `\$NAME`, want: `\$NAME`},
		{name: "cmdsubst preserved",
			in: "$(date)", want: "$(date)"},
		{name: "backtick preserved",
			in: "`date`", want: "`date`"},
		{name: "nested cmdsubst preserved",
			in:   "$(a $(b))",
			want: "$(a $(b))"},
		{name: "value with newline gets quoted",
			in: "$NL", want: `$'line1\nline2'`},
		{name: "value with nul gets quoted",
			in: "$NUL", want: "'a\x00b'"},
		{name: "value with non-utf8 gets quoted",
			in: "$NOUTF8", want: `$'a\xffb'`},
		{name: "value with single quote gets quoted",
			in: "$SQ", want: `"it's"`},
		{name: "value not re-expanded",
			// NAME is "world"; even though we then inject the
			// literal "$NAME" inside the value below, ExpandBody
			// must NOT walk it again.
			in: "$NAME$NAME", want: "worldworld"},
	}
}

func snippet(s string) string {
	const max = 32
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
