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

package timeshader

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestAbs(t *testing.T) {
	sh := new(nopShader)
	ts := Abs(sh)
	tsuite := []struct {
		name     string
		total    int
		frameIn  int
		frameOut int
	}{
		{
			name:     "positive frame stays positive",
			total:    101,
			frameIn:  22,
			frameOut: 22,
		},
		{
			name:     "negative frame becomes positive",
			total:    101,
			frameIn:  -22,
			frameOut: 22,
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			defer sh.reset()
			ts.Shade(tcase.frameIn, tcase.total, [][]term.Cell{})
			require.True(t, sh.shadeCalled)
			assert.Equal(t, tcase.frameOut, sh.frame)
		})
	}

}
