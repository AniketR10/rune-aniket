// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.
package rpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"unstable.build/go-tui/component"
	termpb "unstable.build/go-tui/term/rpc"
)

func TestNewDrawResponse(t *testing.T) {
	tcase := []struct {
		in  string
		out *DrawResponse
	}{
		{
			in: "a",
			out: &DrawResponse{
				Rows: []*termpb.CellRow{
					{Cells: []*termpb.Cell{&zeroCell, &zeroCell, &zeroCell, &zeroCell, &zeroCell}},
					{Cells: []*termpb.Cell{&zeroCell, &zeroCell, &zeroCell, &zeroCell, &zeroCell}},
					{Cells: []*termpb.Cell{
						&zeroCell,
						&zeroCell,
						{Character: 'a', Width: 1},
						&zeroCell,
						&zeroCell,
					}},
					{Cells: []*termpb.Cell{&zeroCell, &zeroCell, &zeroCell, &zeroCell, &zeroCell}},
					{Cells: []*termpb.Cell{&zeroCell, &zeroCell, &zeroCell, &zeroCell, &zeroCell}},
				},
				Cursor: &DrawResponse_Cursor{Position: &termpb.Coordinates{}},
			},
		},
		{
			in: "aaaaaa\naaaaaa\naaaaaa",
			out: &DrawResponse{
				Rows: []*termpb.CellRow{
					{Cells: []*termpb.Cell{&zeroCell, &zeroCell, &zeroCell, &zeroCell, &zeroCell}},
					{Cells: []*termpb.Cell{
						{Character: 'a', Width: 1}, {Character: 'a', Width: 1}, {Character: 'a', Width: 1},
						{Character: 'a', Width: 1}, {Character: 'a', Width: 1},
					}},
					{Cells: []*termpb.Cell{
						{Character: 'a', Width: 1}, {Character: 'a', Width: 1}, {Character: 'a', Width: 1},
						{Character: 'a', Width: 1}, {Character: 'a', Width: 1},
					}},
					{Cells: []*termpb.Cell{
						{Character: 'a', Width: 1}, {Character: 'a', Width: 1}, {Character: 'a', Width: 1},
						{Character: 'a', Width: 1}, {Character: 'a', Width: 1},
					}},
					{Cells: []*termpb.Cell{&zeroCell, &zeroCell, &zeroCell, &zeroCell, &zeroCell}},
				},
				Cursor: &DrawResponse_Cursor{Position: &termpb.Coordinates{}},
			},
		},
		{
			in: "👨‍👧‍👦",
			out: &DrawResponse{
				Rows: []*termpb.CellRow{
					{Cells: []*termpb.Cell{&zeroCell, &zeroCell, &zeroCell, &zeroCell, &zeroCell}},
					{Cells: []*termpb.Cell{&zeroCell, &zeroCell, &zeroCell, &zeroCell, &zeroCell}},
					{Cells: []*termpb.Cell{
						&zeroCell,
						{
							Character: '👨',
							Combining: []uint32{uint32('‍'), uint32('👧'), uint32('‍'), uint32('👦')},
							Width:     2,
						},
						&zeroCell,
						&zeroCell,
						&zeroCell,
					}},
					{Cells: []*termpb.Cell{&zeroCell, &zeroCell, &zeroCell, &zeroCell, &zeroCell}},
					{Cells: []*termpb.Cell{&zeroCell, &zeroCell, &zeroCell, &zeroCell, &zeroCell}},
				},
				Cursor: &DrawResponse_Cursor{Position: &termpb.Coordinates{}},
			},
		},
	}

	for _, tcase := range tcase {
		comp := component.NewStringWithConfig(tcase.in,
			component.StringConfig{Alignment: component.SpanAlignmentCentered})
		comp.Resize(5, 5)
		res := NewDrawResponse(context.Background(), comp, 5, 5)
		assert.Equal(t, tcase.out, res)
	}
}

func benchmarkDrawResponse(b *testing.B, width, height int) {
	var str string
	for i := 0; i < width; i++ {
		str += "fjkelwjflk\njflw\njfklewfkjlkew\n"
	}
	comp := component.NewStringWithConfig(str,
		component.StringConfig{Alignment: component.SpanAlignmentCentered})
	comp.Resize(width, height)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewDrawResponse(context.Background(), comp, width, height)
	}
}

func BenchmarkDrawResponseTiny(b *testing.B) {
	benchmarkDrawResponse(b, 5, 5)
}
func BenchmarkDrawResponseSmall(b *testing.B) {
	benchmarkDrawResponse(b, 50, 50)
}
func BenchmarkDrawResponseMedium(b *testing.B) {
	benchmarkDrawResponse(b, 500, 500)
}
func BenchmarkDrawResponseBig(b *testing.B) {
	benchmarkDrawResponse(b, 5000, 5000)
}
