package rpc

import (
	"context"
	"runtime"

	"unstable.build/go-tui/term"
	termpb "unstable.build/go-tui/term/rpc"
	"unstable.build/go-tui/workspace"
)

type clientWriter struct {
	uri    workspace.URI
	client *Client
}

func (w clientWriter) Edit(
	start, end term.Coordinates, str string,
) (from, to term.Coordinates, old string, err error) {
	ctx := context.Background()

	var protoStart, protoEnd termpb.Coordinates
	protoStart.FromModel(start)
	protoEnd.FromModel(end)
	req := EditCellRequest{
		ResourceName: NewURI(w.uri),
		Start:        &protoStart,
		End:          &protoEnd,
		Str:          str,
	}
	res, err := w.client.ed.EditCell(ctx, &req)
	runtime.KeepAlive(w.client)
	if err != nil {
		return from, to, "", err
	}

	from = res.GetFrom().ToModel()
	to = res.GetTo().ToModel()
	old = res.GetOld()
	return
}
