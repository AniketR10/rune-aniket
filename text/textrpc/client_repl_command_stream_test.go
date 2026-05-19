// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.

package textrpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/term/termrpc"
)

func TestResponsiveFromProtoRows_PreservesPerCellAttributes(t *testing.T) {
	width := 4
	attrs := []term.Attributes{
		{Fg: term.ColorFuchsia, Attrs: term.AttrBold},
		{Fg: term.ColorGreen},
		{Fg: term.ColorGray},
		{Fg: term.ColorRed, Bg: term.ColorBlack},
	}
	runes := []rune{'a', 'b', 'c', 'd'}

	row := &termrpc.CellRow{Cells: make([]*termrpc.Cell, width)}
	for i := range width {
		var pc termrpc.Cell
		pc.FromModel(term.Cell{Attributes: attrs[i], Ch: runes[i], Width: 1})
		row.Cells[i] = &pc
	}

	r := responsiveFromProtoRows([]*termrpc.CellRow{row})
	r.Resize(width, 1)
	w := term.NewStringWriter(width, 1)
	require.NoError(t, w.Clear(term.Attributes{}))
	r.Draw(w)

	for x := range width {
		got := w.Cells()[x]
		assert.Equal(t, runes[x], got.Ch, "cell %d rune", x)
		assert.Equal(t, attrs[x], got.Attributes,
			"cell %d attributes should round-trip per-cell; "+
				"if they collapse to the zero value the client side "+
				"is dropping per-cell colors", x)
	}
}
