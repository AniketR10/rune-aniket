package rpc

import (
	"context"

	"github.com/ernestrc/go-tui/term"
	termpb "github.com/ernestrc/go-tui/term/rpc"
)

type clientWriter struct {
	handlerID uint32
	client    *Client
}

func (w clientWriter) Edit(
	start, end term.Coordinates, str string,
) (from, to term.Coordinates, old string, err error) {
	ctx := context.Background()

	var protoStart, protoEnd termpb.Coordinates
	protoStart.FromModel(start)
	protoEnd.FromModel(end)
	req := EditCellRequest{
		HandlerId: w.handlerID,
		Start:     &protoStart,
		End:       &protoEnd,
		Str:       str,
	}
	res, err := w.client.ed.EditCell(ctx, &req)
	if err != nil {
		return from, to, "", err
	}

	from = res.GetFrom().ToModel()
	to = res.GetTo().ToModel()
	old = res.GetOld()
	return
}
