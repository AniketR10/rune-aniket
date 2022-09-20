package text

import (
	"context"

	"github.com/ernestrc/go-tui/term"
	textpb "github.com/ernestrc/go-tui/text/rpc"
)

type clientView struct {
	handlerID uint32
	client    *Client
}

func (r clientView) RawCells() ([][]term.Cell, error) {
	ctx := context.Background()
	req := textpb.RawCellsRequest{HandlerId: r.handlerID}

	res, err := r.client.ed.RawCells(ctx, &req)
	if err != nil {
		return nil, err
	}

	return textpb.RawCellsResponseToBuffer(res).RawCells(), nil
}
