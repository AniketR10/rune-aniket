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

package vtescanner

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
