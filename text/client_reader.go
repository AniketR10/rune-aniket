package text

import (
	"context"

	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
)

type clientReader struct {
	handlerID uint32
	client    *Client
}

func (r clientReader) RawCells() ([][]term.Cell, error) {
	ctx := context.Background()
	req := proto.RawCellsRequest{HandlerId: r.handlerID}

	res, err := r.client.ed.RawCells(ctx, &req)
	if err != nil {
		return nil, err
	}

	return proto.RawCellsResponseToBuffer(res).RawCells(), nil
}
