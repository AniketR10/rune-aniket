package ide

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"unstable.build/go-tui/term"
)

func TestMacroParse(t *testing.T) {
	suite := []struct {
		input          string
		expectedOutput []macroKey
		expectedErr    error
	}{
		{},
		{
			input:       "><",
			expectedErr: errors.New("invalid escape sequence: unescaped, starting '>' character"),
		},
		{
			input:          "a",
			expectedOutput: []macroKey{{KeyComb: term.KeyComb{Ch: 'a'}}},
		},
		{
			input:          "<space>",
			expectedOutput: []macroKey{{KeyComb: term.KeyComb{Key: term.KeySpace}}},
		},
		{
			input: "a{WAIT}",
			expectedErr: errors.New("invalid instruction: {WAIT}"),
		},
		{
			input: "a{{WAIT}",
			expectedErr: errors.New("invalid instruction: {{WAIT}"),
		},
		{
			input: "a{}WAIT}",
			expectedErr: errors.New("invalid instruction: {}"),
		},
		{
			input: "a<{wait}space>",
			expectedErr: errors.New("unterminated key: '<' found but no matching '>' found"),
		},
		{
			input: "a{wait}",
			expectedOutput: []macroKey{
				{KeyComb: term.KeyComb{Ch: 'a'}},
				{instructWait: true},
			},
		},
		{
			input: "{wait}a",
			expectedOutput: []macroKey{
				{instructWait: true},
				{KeyComb: term.KeyComb{Ch: 'a'}},
			},
		},
		{
			input: "<space>{wait}",
			expectedOutput: []macroKey{
				{KeyComb: term.KeyComb{Key: term.KeySpace}},
				{instructWait: true},
			},
		},
		{
			input: "{wait}<space>",
			expectedOutput: []macroKey{
				{instructWait: true},
				{KeyComb: term.KeyComb{Key: term.KeySpace}},
			},
		},
		{
			input: "<space>{wait}<space>",
			expectedOutput: []macroKey{
				{KeyComb: term.KeyComb{Key: term.KeySpace}},
				{instructWait: true},
				{KeyComb: term.KeyComb{Key: term.KeySpace}},
			},
		},
		{
			input: "{wait}<space>{wait}",
			expectedOutput: []macroKey{
				{instructWait: true},
				{KeyComb: term.KeyComb{Key: term.KeySpace}},
				{instructWait: true},
			},
		},
		{
			input: "<space>{wait}<space>{wait}",
			expectedOutput: []macroKey{
				{KeyComb: term.KeyComb{Key: term.KeySpace}},
				{instructWait: true},
				{KeyComb: term.KeyComb{Key: term.KeySpace}},
				{instructWait: true},
			},
		},
		{
			input: "{wait}<space>{wait}<space>",
			expectedOutput: []macroKey{
				{instructWait: true},
				{KeyComb: term.KeyComb{Key: term.KeySpace}},
				{instructWait: true},
				{KeyComb: term.KeyComb{Key: term.KeySpace}},
			},
		},
	}

	for i, test := range suite {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			actualOutput, actualErr := parseMacroKeys(test.input)
			assert.Equal(t, test.expectedOutput, actualOutput)
			assert.Equal(t, test.expectedErr, actualErr)
		})
	}
}
