package utf8parser

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testReceiver struct {
	builder strings.Builder
	err     error
}

func (t *testReceiver) Codepoint(c rune) {
	t.builder.WriteRune(c)
}

func (t *testReceiver) InvalidSequence() {
	t.err = errors.New("invalid sequence")
}

func TestParser(t *testing.T) {
	receiver := &testReceiver{}
	parser := NewParser(receiver)
	data, err := os.ReadFile("UTF-8-demo.txt")
	require.NoError(t, err)

	for _, ch := range data {
		parser.Advance(ch)
	}

	require.NoError(t, receiver.err)
	assert.Equal(t, string(data), receiver.builder.String())
}
