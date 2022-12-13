package rpc

import (
	"context"
	"runtime"

	"unstable.build/go-tui/term"
	"unstable.build/go-tui/workspace"
)

type clientView struct {
	uri    workspace.URI
	client *Client
}

func (r clientView) RawCells() ([][]term.Cell, error) {
	ctx := context.Background()
	req := RawCellsRequest{ResourceName: NewURI(r.uri)}

	res, err := r.client.ed.RawCells(ctx, &req)
	runtime.KeepAlive(r.client)
	if err != nil {
		return nil, err
	}

	return RawCellsResponseToBuffer(res).RawCells(), nil
}
