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

func TestAcceleratePlot(t *testing.T) {
	ts := accelerate(&nopShader{})
	res := genPlot(ts, 1.0, 0.0)
	expect := `
·········································································x
·······································································xx·
······································································x···
··········································································
····································································xx····
···································································x······
··································································x·······
·································································x········
·······························································xx·········
······························································x···········
·····························································x············
···························································xx·············
··········································································
·························································xx···············
························································x·················
······················································xx··················
·····················································x····················
···················································xx·····················
··················································x·······················
················································xx························
··············································xx··························
·············································x····························
··········································xxx·····························
·········································x································
·······································xx·································
·····································xx···································
···································xx·····································
································xxx·······································
······························xx··········································
···························xxx············································
·······················xxxx···············································
·····················xx···················································
················xxxxx·····················································
··········xxxxxx··························································
xxxxxxxxxx++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
`
	assert.Equal(t, expect, res)
}

func TestAccelerate(t *testing.T) {
	sh := new(nopShader)
	ts := Accelerate(sh)

	tsuite := []struct {
		name     string
		total    int
		frameIn  int
		frameOut int
	}{
		{
			name:     "first frame is unchanged",
			total:    100,
			frameIn:  0,
			frameOut: 0,
		},
		{
			name:     "last frame is unchanged",
			total:    100,
			frameIn:  99,
			frameOut: 99,
		},
		{
			name:     "midpoint is at 70 per cent",
			total:    100,
			frameIn:  70,
			frameOut: 49,
		},
		{
			name:     "negative number do not retain sign", // it's quadratic! neg * neg = pos
			total:    100,
			frameIn:  -70,
			frameOut: 49,
		},
	}

	for _, tcase := range tsuite {
		defer sh.reset()
		ts.Shade(tcase.frameIn, tcase.total, [][]term.Cell{})
		require.True(t, sh.shadeCalled)
		assert.Equal(t, tcase.frameOut, sh.frame)
	}
}
