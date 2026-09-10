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

package timeshader

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestReversePlot(t *testing.T) {
	ts := reverse(&nopShader{})
	res := genPlot(ts, 1.0, 0.0)
	expect := `
xxx·······································································
···xx·····································································
·····xxx··································································
········x·································································
·········xx·······························································
···········xx·····························································
·············xxx··························································
················xx························································
··················xx······················································
····················xx····················································
······················xx··················································
························xx················································
··························xx··············································
····························xx············································
······························xxx·········································
·································xx·······································
···································xx·····································
·····································xx···································
·······································xx·································
·········································xx·······························
···········································xx·····························
·············································xx···························
···············································xxx························
··················································xx······················
····················································x·····················
·····················································xxx··················
························································xx················
··························································xx··············
····························································xx············
······························································xxx·········
·································································xx·······
···································································x······
····································································xx····
······································································xxx·
+++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++x
`
	assert.Equal(t, expect, res)
}

func TestReverse(t *testing.T) {
	sh := new(nopShader)
	ts := Reverse(sh)

	tsuite := []struct {
		name     string
		total    int
		frameIn  int
		frameOut int
	}{
		{
			name:     "first frame becomes last frame",
			total:    100,
			frameIn:  0,
			frameOut: 99,
		},
		{
			name:     "last frame becomes first",
			total:    100,
			frameIn:  99,
			frameOut: 0,
		},
		{
			name:     "midpoint is unchanged",
			total:    100,
			frameIn:  49,
			frameOut: 50,
		},
		{
			name:     "negative number does not panic",
			total:    100,
			frameIn:  -20,
			frameOut: 119, // strange results can stimulate creativity
		},
	}

	for _, tcase := range tsuite {
		defer sh.reset()
		ts.Shade(tcase.frameIn, tcase.total, [][]term.Cell{})
		require.True(t, sh.shadeCalled)
		assert.Equal(t, tcase.frameOut, sh.frame)
	}
}
