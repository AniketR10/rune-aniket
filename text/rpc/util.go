package rpc

import (
	"context"

	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
	termpb "unstable.build/go-tui/term/rpc"
)

// NewEditRequests converts a buf into an EditRequest.
func NewEditRequest(file workspaceapi.URI, buf *cell.Buffer) EditRequest {
	return EditRequest{
		Buffer:       rawCellsToProtoCells(buf.RawCells()),
		ResourceName: NewURI(file),
	}
}

// EditRequestToBuffer converts an EditRequest into a cell.Buffer
func EditRequestToBuffer(in *EditRequest) *cell.Buffer {
	return rowsToBuffer(in.GetBuffer())
}

// NewURIFromProto maps rpc.URI into a workspaceapi.URI.
func NewURIFromProto(u *URI) (workspaceapi.URI, error) {
	return workspaceapi.ParseURI(u.GetUri())
}

// NewURI maps a workspaceapi.URI into a rpc.URI.
func NewURI(u workspaceapi.URI) *URI {
	return &URI{Uri: u.String()}
}

// RawCellsResponseToBuffer converts an EditRequest into a cell.Buffer
func RawCellsResponseToBuffer(in *RawCellsResponse) *cell.Buffer {
	return rowsToBuffer(in.GetRows())
}

// NewRawCellsResponse converts a buf into an RawCellsResponse.
func NewRawCellsResponse(cells [][]term.Cell) *RawCellsResponse {
	return &RawCellsResponse{Rows: rawCellsToProtoCells(cells)}
}

func rowsToBuffer(in []*termpb.CellRow) *cell.Buffer {
	var maxWidth int
	for _, row := range in {
		if len(row.Cells) > maxWidth {
			maxWidth = len(row.Cells)
		}
	}

	var w cell.BufferWriter
	w.Init(context.Background(), maxWidth, len(in))

	for y, rows := range in {
		for x, cell := range rows.Cells {
			w.SetCell(term.Coordinates{X: x, Y: y}, cell.ToModel())
		}
	}

	ret := new(cell.Buffer)
	w.ToBuffer(ret)
	return ret
}

func rawCellsToProtoCells(cells [][]term.Cell) []*termpb.CellRow {
	var size int
	for _, row := range cells {
		size += len(row)
	}

	// these slabs reduce allocations from ~N (=num cells)
	// to 4 which reduces this function's ns/op from 60 to 80%
	rows := make([]*termpb.CellRow, len(cells))
	protoCellRowSlabPtr := make([]termpb.CellRow, len(cells))
	cellRowSlab := make([]*termpb.Cell, size)
	cellRowSlabIdx := 0
	cellSlabPtr := make([]termpb.Cell, size)
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
