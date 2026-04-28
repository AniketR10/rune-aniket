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
	"testing"

	"github.com/stretchr/testify/assert"

	shsyntax "mvdan.cc/sh/v3/syntax"
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
			`workspacenew ~/Unstable\ Build`,
			[]string{"workspacenew", `~/Unstable\ Build`},
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
	}

	p := shsyntax.NewParser()
	for _, tc := range tcases {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.want, SplitCommandLine(p, tc.in))
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
	}
	p := shsyntax.NewParser()
	for _, tc := range tcases {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.want, UnquoteToken(p, tc.in))
		})
	}
}
