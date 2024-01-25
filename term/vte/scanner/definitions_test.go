package scanner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefinitions(t *testing.T) {
	tests := []struct {
		inData         uint8
		expectedState  State
		expectedAction Action
	}{
		{0xee, SosPmApcString, Unhook},
		{0x0f, Utf8, None},
		{0xff, Utf8, BeginUtf8},
		{0xee, SosPmApcString, Unhook},
		{0x0f, Utf8, None},
		{0xff, Utf8, BeginUtf8},
	}

	for _, test := range tests {
		state, action := unpack(test.inData)
		assert.Equal(t, test.expectedState, state)
		assert.Equal(t, test.expectedAction, action)
	}
}
