// Copyright (C) 2017-2026 Unstable Build, LLC
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
