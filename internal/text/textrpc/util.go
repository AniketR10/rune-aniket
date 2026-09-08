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

package textrpc

import (
	"github.com/unstablebuild/rune-go-sdk/api/textapi/textrpc"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/term/termrpc"
	"unstable.build/rune/internal/cell"
)

// NewEditRequest converts a buf into an EditRequest.
func NewEditRequest(
	file workspaceapi.URI, buf *cell.Buffer, readOnly, recovered bool,
) textrpc.EditRequest {
	return textrpc.EditRequest{
		Buffer:       rawCellsToProtoCells(buf.RawCells()),
		ResourceName: NewURI(file),
		ReadOnly:     readOnly,
		Recovered:    recovered,
	}
}

// EditRequestToBuffer converts an EditRequest into a cell.Buffer
func EditRequestToBuffer(in *textrpc.EditRequest) *cell.Buffer {
	return rowsToBuffer(in.GetBuffer())
}

// NewURIFromProto maps rpc.URI into a workspaceapi.URI.
func NewURIFromProto(u *textrpc.URI) (workspaceapi.URI, error) {
	return workspaceapi.ParseURI(u.GetUri())
}

// NewURI maps a workspaceapi.URI into a rpc.URI.
func NewURI(u workspaceapi.URI) *textrpc.URI {
	return &textrpc.URI{Uri: u.String()}
}

// RawCellsResponseToBuffer converts an EditRequest into a cell.Buffer
func RawCellsResponseToBuffer(in *textrpc.RawCellsResponse) *cell.Buffer {
	return rowsToBuffer(in.GetRows())
}

// NewRawCellsResponse converts a buf into an RawCellsResponse.
func NewRawCellsResponse(cells [][]term.Cell) *textrpc.RawCellsResponse {
	return &textrpc.RawCellsResponse{Rows: rawCellsToProtoCells(cells)}
}

func rowsToBuffer(in []*termrpc.CellRow) *cell.Buffer {
	var total int
	for _, row := range in {
		total += len(row.Cells)
	}

	// Rows are carved exact-size (cap==len) out of one contiguous slab:
	// a height x maxWidth rectangle here would be retained for the
	// lifetime of the buffer, which is quadratic-ish waste when a single
	// long line meets many short ones.
	rows := make([][]term.Cell, len(in))
	slab := make([]term.Cell, total)
	var off int
	for y, row := range in {
		n := len(row.Cells)
		if n == 0 {
			continue
		}
		r := slab[off : off+n : off+n]
		off += n
		for x, c := range row.Cells {
			r[x] = c.ToModel()
		}
		rows[y] = r
	}
	return cell.AdoptCellsToBuffer(rows)
}

func rawCellsToProtoCells(cells [][]term.Cell) []*termrpc.CellRow {
	var size int
	for _, row := range cells {
		size += len(row)
	}

	// these slabs reduce allocations from ~N (=num cells)
	// to 4 which reduces this function's ns/op from 60 to 80%
	rows := make([]*termrpc.CellRow, len(cells))
	protoCellRowSlabPtr := make([]termrpc.CellRow, len(cells))
	cellRowSlab := make([]*termrpc.Cell, size)
	cellRowSlabIdx := 0
	cellSlabPtr := make([]termrpc.Cell, size)
	cellSlabPtrIdx := 0

	for y, row := range cells {
		cells := cellRowSlab[cellRowSlabIdx : cellRowSlabIdx+len(row)]
		cellRowSlabIdx += len(row)
		for x, cell := range row {
			c := &cellSlabPtr[cellSlabPtrIdx]
			cellSlabPtrIdx++
			c.FromModel(cell)
			cells[x] = c
		}
		protoCellRowSlabPtr[y].Cells = cells
		rows[y] = &protoCellRowSlabPtr[y]
	}

	return rows
}
