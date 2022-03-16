package editor

import (
	"context"

	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
)

type clientWriter struct {
	handlerID uint32
	client    *Client
}

func (w clientWriter) Update(
	start, end term.Coordinates, str string,
) (from, to term.Coordinates, old string, err error) {
	ctx := context.Background()

	var protoStart, protoEnd proto.Coordinates
	protoStart.FromModel(start)
	protoEnd.FromModel(end)
	req := proto.UpdateRequest{
		HandlerId: w.handlerID,
		Start:     &protoStart,
		End:       &protoEnd,
		Str:       str,
	}
	res, err := w.client.ed.Update(ctx, &req)
	if err != nil {
		return from, to, "", err
	}

	from = res.GetFrom().ToModel()
	to = res.GetTo().ToModel()
	old = res.GetOld()
	return
}
