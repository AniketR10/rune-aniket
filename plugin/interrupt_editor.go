package plugin

import (
	"context"

	textpb "github.com/ernestrc/go-tui/text/rpc"
)

// this structure wraps a text.Editor to
// provide interrupt on write requests coming from the wire
type interruptEditor struct {
	textpb.EditorServer
	interruptDraw func()
}

func interruptEditorServer(srv textpb.EditorServer, interruptDraw func()) textpb.EditorServer {
	return &interruptEditor{EditorServer: srv, interruptDraw: interruptDraw}
}

func (e *interruptEditor) Edit(ctx context.Context, req *textpb.EditRequest) (
	*textpb.EditResponse, error,
) {
	res, err := e.EditorServer.Edit(ctx, req)
	e.interruptDraw()
	return res, err
}

func (e *interruptEditor) Subscribe(ctx context.Context, req *textpb.EditorSubscribeRequest) (
	*textpb.EditorSubscribeResponse, error,
) {
	res, err := e.EditorServer.Subscribe(ctx, req)
	e.interruptDraw()
	return res, err

}

func (e *interruptEditor) Register(ctx context.Context, req *textpb.RegisterCommandRequest) (
	*textpb.RegisterCommandResponse, error,
) {
	res, err := e.EditorServer.Register(ctx, req)
	e.interruptDraw()
	return res, err

}

func (e *interruptEditor) SetLocationList(ctx context.Context, req *textpb.SetLocationListRequest) (
	*textpb.SetLocationListResponse, error,
) {
	res, err := e.EditorServer.SetLocationList(ctx, req)
	e.interruptDraw()
	return res, err

}
func (e *interruptEditor) MoveToNextLocation(ctx context.Context, req *textpb.MoveToLocationRequest) (
	*textpb.MoveToLocationResponse, error,
) {
	res, err := e.EditorServer.MoveToNextLocation(ctx, req)
	e.interruptDraw()
	return res, err

}
func (e *interruptEditor) MoveToPrevLocation(ctx context.Context, req *textpb.MoveToLocationRequest) (
	*textpb.MoveToLocationResponse, error,
) {
	res, err := e.EditorServer.MoveToPrevLocation(ctx, req)
	e.interruptDraw()
	return res, err

}
func (e *interruptEditor) EditCell(ctx context.Context, req *textpb.EditCellRequest) (
	*textpb.EditCellResponse, error,
) {
	res, err := e.EditorServer.EditCell(ctx, req)
	e.interruptDraw()
	return res, err

}
