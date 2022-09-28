package rpc

import (
	"context"

	"unstable.build/go-tui/term"
)

type clientView struct {
	handlerID uint32
	client    *Client
}

func (r clientView) RawCells() ([][]term.Cell, error) {
	ctx := context.Background()
	req := RawCellsRequest{HandlerId: r.handlerID}

	res, err := r.client.ed.RawCells(ctx, &req)
	if err != nil {
		return nil, err
	}

	return RawCellsResponseToBuffer(res).RawCells(), nil
}
