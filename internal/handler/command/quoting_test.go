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

package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitCommandLine(t *testing.T) {
	tcases := []struct {
		desc string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"single arg", "edit", []string{"edit"}},
		{"two args", "edit foo", []string{"edit", "foo"}},
		{"collapses extra spaces", "edit   foo", []string{"edit", "foo"}},
		{
			"escaped space stays in same arg",
			`workspaceopen ~/Unstable\ Build`,
			[]string{"workspaceopen", `~/Unstable\ Build`},
		},
		{
			"escaped backslash",
			`echo a\\b`,
			[]string{"echo", `a\\b`},
		},
		{
			"single quoted arg with spaces",
			`edit 'path with space'`,
			[]string{"edit", `'path with space'`},
		},
		{
			"double quoted arg with spaces",
			`edit "path with space"`,
			[]string{"edit", `"path with space"`},
		},
		{
			"double quoted arg with escapes",
			`echo "a\"b"`,
			[]string{"echo", `"a\"b"`},
		},
		{
			"mix quoted and unquoted",
			`cmd a\ b 'c d' "e f"`,
			[]string{"cmd", `a\ b`, `'c d'`, `"e f"`},
		},
		{
			"trailing dangling backslash kept",
			`cmd foo\`,
			[]string{"cmd", `foo\`},
		},
		{
			"unclosed single quote consumes rest",
			`cmd 'unclosed text`,
			[]string{"cmd", `'unclosed text`},
		},
		{
			"unclosed double quote consumes rest",
			`cmd "unclosed text`,
			[]string{"cmd", `"unclosed text`},
		},
		{
			"angle brackets pass through verbatim",
			`echo <space><enter>`,
			[]string{"echo", `<space><enter>`},
		},
		{
			"pipe passes through verbatim",
			`searchast a|b`,
			[]string{"searchast", `a|b`},
		},
		{
			"braces pass through verbatim",
			`echo {prompt}edit`,
			[]string{"echo", `{prompt}edit`},
		},
		{
			"misc shell metas pass through verbatim",
			`cmd a&b;c(d)e $VAR ~ * ? #frag`,
			[]string{"cmd", `a&b;c(d)e`, `$VAR`, `~`, `*`, `?`, `#frag`},
		},
		{
			"alias body with echo syntax stays one arg",
			`echo {prompt}searchast<space>locals.scm<enter>`,
			[]string{"echo", `{prompt}searchast<space>locals.scm<enter>`},
		},
	}

	for _, tc := range tcases {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.want, SplitCommandLine(tc.in))
		})
	}
}

func TestUnquoteToken(t *testing.T) {
	tcases := []struct {
		in, want string
	}{
		{"", ""},
		{"abc", "abc"},
		{`a\ b`, "a b"},
		{`a\\b`, `a\b`},
		{`'a b'`, "a b"},
		{`"a b"`, "a b"},
		{`"a\"b"`, `a"b`},
		{`"a\\b"`, `a\b`},
		{`'a\ b'`, `a\ b`},
		{`~/Unstable\ Build`, "~/Unstable Build"},
		// unclosed: lenient — strip the opener, keep the rest
		{`'unclosed`, "unclosed"},
		{`"unclosed`, "unclosed"},
		// trailing dangling backslash drops the trailing slash
		{`abc\`, "abc"},
		// angle brackets and pipes are literal at Layer 1
		{`<space>`, `<space>`},
		{`a|b`, `a|b`},
		{`{prompt}searchast<space>locals.scm<enter>`,
			`{prompt}searchast<space>locals.scm<enter>`},
	}
	for _, tc := range tcases {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.want, UnquoteToken(tc.in))
		})
	}
}

func TestShellQuote(t *testing.T) {
	tcases := []struct {
		desc, in, want string
	}{
		{"empty", "", `''`},
		{"plain", "abc", "abc"},
		{"with space", "a b", `'a b'`},
		{"with tab", "a\tb", "'a\tb'"},
		{"with backslash", `a\b`, `'a\b'`},
		{"with double quote", `a"b`, `'a"b'`},
		{"with single quote", `a'b`, `"a'b"`},
		{"angle brackets are literal", `<space>`, `<space>`},
		{"braces are literal", `{prompt}`, `{prompt}`},
	}
	for _, tc := range tcases {
		t.Run(tc.desc, func(t *testing.T) {
			got := ShellQuote(tc.in)
			assert.Equal(t, tc.want, got)
			// round-trip: SplitCommandLine + UnquoteToken yields original.
			toks := SplitCommandLine(got)
			if assert.Len(t, toks, 1) {
				assert.Equal(t, tc.in, UnquoteToken(toks[0]))
			}
		})
	}
}

func TestLastTokenIsIncomplete(t *testing.T) {
	tcases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"abc", false},
		{`abc\`, true},
		{`abc\\`, false},
		{`abc\\\`, true},
		{`'abc`, true},
		{`'abc'`, false},
		{`"abc`, true},
		{`"abc"`, false},
		{`"a\"b"`, false},
		{`"a\"b`, true},
		// dangling backslash inside double-quote that escapes the closing quote
		{`"abc\"`, true},
	}
	for _, tc := range tcases {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.want, LastTokenIsIncomplete(tc.in))
		})
	}
}

// FuzzShellQuote checks the central round-trip invariant: for any
// string s, ShellQuote(s) tokenises as a single argv element whose
// UnquoteToken value is s.
func FuzzShellQuote(f *testing.F) {
	for _, seed := range []string{
		"", "abc", "a b", "a\tb", `a\b`, `a"b`, `a'b`, `a'b"c\d e`,
		`<space>`, `{prompt}`, `a|b`, `a;b`, `~`, `*`, `?`, `#frag`,
		"\x00", "\xff\xfe", "α β", "🙂",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		quoted := ShellQuote(s)
		toks := SplitCommandLine(quoted)
		if len(toks) != 1 {
			t.Fatalf("ShellQuote(%q) = %q; SplitCommandLine returned %d tokens, want 1", s, quoted, len(toks))
		}
		if got := UnquoteToken(toks[0]); got != s {
			t.Fatalf("round-trip mismatch: ShellQuote(%q) = %q; UnquoteToken = %q", s, quoted, got)
		}
		// LastTokenIsIncomplete must also be false on a freshly
		// quoted value: ShellQuote always produces a closed
		// token.
		if LastTokenIsIncomplete(quoted) {
			t.Fatalf("ShellQuote(%q) = %q reports incomplete", s, quoted)
		}
	})
}

// FuzzSplitCommandLine checks that SplitCommandLine never panics, that
// every emitted token is a non-empty substring of the input, that the
// total byte count of tokens does not exceed the input length, and
// that each token re-splits to itself (token idempotency).
func FuzzSplitCommandLine(f *testing.F) {
	for _, seed := range []string{
		"", " ", "a", "a b", `a\ b`, `'a b'`, `"a b"`, `"a\"b"`,
		`a\`, `'unclosed`, `"unclosed`, `<space>`, "a|b{c}d<e>f",
		"   ", "\t\t", "a\tb", "\x00\x01", "α β γ", "🙂🙃",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		toks := SplitCommandLine(s)
		var total int
		for _, tok := range toks {
			if tok == "" {
				t.Fatalf("SplitCommandLine(%q) emitted empty token", s)
			}
			total += len(tok)
			// Each token re-tokenized in isolation must be a
			// single element equal to itself (token idempotency).
			again := SplitCommandLine(tok)
			if len(again) != 1 || again[0] != tok {
				t.Fatalf("token idempotency: SplitCommandLine(%q) -> %v; re-split %q -> %v",
					s, toks, tok, again)
			}
		}
		// Tokens are substrings of input minus separators; their
		// combined length cannot exceed len(s).
		if total > len(s) {
			t.Fatalf("SplitCommandLine(%q) -> %v; combined token bytes %d > input %d",
				s, toks, total, len(s))
		}
	})
}

// FuzzUnquoteToken checks that UnquoteToken never panics and never
// produces output longer than its input (Layer 1 only ever strips
// quoting/escape bytes, never adds any).
func FuzzUnquoteToken(f *testing.F) {
	for _, seed := range []string{
		"", "abc", `a\ b`, `a\\b`, `'a b'`, `"a b"`, `"a\"b"`,
		`'unclosed`, `"unclosed`, `abc\`, `<space>`, "α", "🙂",
		"\x00\xff", "''", `""`, `"\n"`, `"\\"`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got := UnquoteToken(s)
		if len(got) > len(s) {
			t.Fatalf("UnquoteToken(%q) = %q; output longer than input (%d > %d)",
				s, got, len(got), len(s))
		}
	})
}

// FuzzLastTokenIsIncomplete checks that LastTokenIsIncomplete never
// panics and that its result is consistent with SplitCommandLine: when
// it reports false, appending a separator and a fresh literal letter
// produces a new trailing token; when it reports true, that letter is
// absorbed into the in-progress token.
func FuzzLastTokenIsIncomplete(f *testing.F) {
	for _, seed := range []string{
		"", "abc", `abc\`, `abc\\`, `'abc`, `'abc'`, `"abc`, `"abc"`,
		`"a\"b"`, `"a\"b`, "a b", "<space>", "α", "🙂",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		incomplete := LastTokenIsIncomplete(s)
		// Appending " x" — a separator then a fresh literal letter
		// — must split into one extra token iff the buffer was not
		// incomplete.
		base := SplitCommandLine(s)
		extended := SplitCommandLine(s + " x")
		if incomplete {
			// Inside an open quote/escape, the separator and
			// the trailing letter are both consumed by the
			// current token; token count stays the same (and
			// the last token grows). Exception: an empty
			// buffer cannot be incomplete.
			if len(extended) != len(base) && !(len(base) == 0 && len(extended) == 1) {
				t.Fatalf("LastTokenIsIncomplete(%q) reported true but split count changed: base=%v extended=%v",
					s, base, extended)
			}
		} else {
			// Cleanly terminated: the new "x" must surface as
			// its own trailing token.
			if len(extended) != len(base)+1 {
				t.Fatalf("LastTokenIsIncomplete(%q) reported false but split count did not grow by one: base=%v extended=%v",
					s, base, extended)
			}
			if extended[len(extended)-1] != "x" {
				t.Fatalf("LastTokenIsIncomplete(%q) reported false but trailing token != %q: extended=%v",
					s, "x", extended)
			}
		}
	})
}
