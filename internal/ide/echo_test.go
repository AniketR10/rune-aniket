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

package ide

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestEchoParse(t *testing.T) {
	suite := []struct {
		input          string
		expectedOutput []echoKey
		expectedErr    error
	}{
		{},
		{
			input:       "><",
			expectedErr: errors.New("invalid escape sequence: unescaped, starting '>' character"),
		},
		{
			input:          "a",
			expectedOutput: []echoKey{{KeyComb: term.KeyComb{Ch: 'a'}}},
		},
		{
			input:          "<space>",
			expectedOutput: []echoKey{{KeyComb: term.KeyComb{Key: term.KeySpace}}},
		},
		{
			input:       "a{WAIT}",
			expectedErr: errors.New("invalid instruction: {WAIT}"),
		},
		{
			input:       "a{{WAIT}",
			expectedErr: errors.New("invalid instruction: {{WAIT}"),
		},
		{
			input:       "a{}WAIT}",
			expectedErr: errors.New("invalid instruction: {}"),
		},
		{
			input:       "a<{wait}space>",
			expectedErr: errors.New("unterminated key: '<' found but no matching '>' found"),
		},
		{
			input: "a{wait}",
			expectedOutput: []echoKey{
				{KeyComb: term.KeyComb{Ch: 'a'}},
				{instructWait: true},
			},
		},
		{
			input: "{wait}a",
			expectedOutput: []echoKey{
				{instructWait: true},
				{KeyComb: term.KeyComb{Ch: 'a'}},
			},
		},
		{
			input: "<space>{wait}",
			expectedOutput: []echoKey{
				{KeyComb: term.KeyComb{Key: term.KeySpace}},
				{instructWait: true},
			},
		},
		{
			input: "{wait}<space>",
			expectedOutput: []echoKey{
				{instructWait: true},
				{KeyComb: term.KeyComb{Key: term.KeySpace}},
			},
		},
		{
			input: "<space>{wait}<space>",
			expectedOutput: []echoKey{
				{KeyComb: term.KeyComb{Key: term.KeySpace}},
				{instructWait: true},
				{KeyComb: term.KeyComb{Key: term.KeySpace}},
			},
		},
		{
			input: "{wait}<space>{wait}",
			expectedOutput: []echoKey{
				{instructWait: true},
				{KeyComb: term.KeyComb{Key: term.KeySpace}},
				{instructWait: true},
			},
		},
		{
			input: "<space>{wait}<space>{wait}",
			expectedOutput: []echoKey{
				{KeyComb: term.KeyComb{Key: term.KeySpace}},
				{instructWait: true},
				{KeyComb: term.KeyComb{Key: term.KeySpace}},
				{instructWait: true},
			},
		},
		{
			input: "{wait}<space>{wait}<space>",
			expectedOutput: []echoKey{
				{instructWait: true},
				{KeyComb: term.KeyComb{Key: term.KeySpace}},
				{instructWait: true},
				{KeyComb: term.KeyComb{Key: term.KeySpace}},
			},
		},
		{
			input: "{prompt}<space>{prompt}<space>",
			expectedOutput: []echoKey{
				{instructPrompt: true},
				{KeyComb: term.KeyComb{Key: term.KeySpace}},
				{instructPrompt: true},
				{KeyComb: term.KeyComb{Key: term.KeySpace}},
			},
		},
		{
			input:       "{register}",
			expectedErr: errors.New("missing register ID after {register}"),
		},
		{
			input: "a{register}b<space>",
			expectedOutput: []echoKey{
				{KeyComb: term.KeyComb{Ch: 'a'}},
				{instructReg: "b"},
				{KeyComb: term.KeyComb{Key: term.KeySpace}},
			},
		},
	}

	for i, test := range suite {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			t.Parallel()

			actualOutput, actualErr := parseEchoKeys(test.input)
			assert.Equal(t, test.expectedOutput, actualOutput)
			assert.Equal(t, test.expectedErr, actualErr)
		})
	}
}
