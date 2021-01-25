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

func (w clientWriter) Insert(
	at term.Coordinates, str string,
) (from, to term.Coordinates, err error) {
	ctx := context.Background()

	var protoAt proto.Coordinates
	protoAt.FromModel(at)
	req := proto.InsertRequest{
		HandlerId: w.handlerID,
		At:        &protoAt,
		Str:       str,
	}
	res, err := w.client.ed.Insert(ctx, &req)
	if err != nil {
		return from, to, err
	}

	from = res.GetFrom().ToModel()
	to = res.GetTo().ToModel()
	return
}

func (w clientWriter) Delete(
	from, to term.Coordinates,
) (start, end term.Coordinates, str string, err error) {
	ctx := context.Background()
	var protoTo, protoFrom proto.Coordinates
	protoFrom.FromModel(from)
	protoTo.FromModel(to)

	req := proto.DeleteRequest{
		HandlerId: w.handlerID,
		From:      &protoFrom,
		To:        &protoTo,
	}
	res, err := w.client.ed.Delete(ctx, &req)
	if err != nil {
		return start, end, str, err
	}

	start = res.GetStart().ToModel()
	end = res.GetEnd().ToModel()
	str = res.GetStr()
	return
}
