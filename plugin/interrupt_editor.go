package plugin

import (
	"context"

	"github.com/ernestrc/go-tui/proto"
)

// this structure wraps a editor.Editor to
// provide interrupt on write requests coming from the wire
type interruptEditor struct {
	proto.EditorServer
	interruptDraw func()
}

func interruptEditorServer(srv proto.EditorServer, interruptDraw func()) proto.EditorServer {
	return &interruptEditor{EditorServer: srv, interruptDraw: interruptDraw}
}

func (e *interruptEditor) Edit(ctx context.Context, req *proto.EditRequest) (
	*proto.EditResponse, error,
) {
	res, err := e.EditorServer.Edit(ctx, req)
	e.interruptDraw()
	return res, err
}

func (e *interruptEditor) Subscribe(ctx context.Context, req *proto.EditorSubscribeRequest) (
	*proto.EditorSubscribeResponse, error,
) {
	res, err := e.EditorServer.Subscribe(ctx, req)
	e.interruptDraw()
	return res, err

}

func (e *interruptEditor) Register(ctx context.Context, req *proto.RegisterCommandRequest) (
	*proto.RegisterCommandResponse, error,
) {
	res, err := e.EditorServer.Register(ctx, req)
	e.interruptDraw()
	return res, err

}

func (e *interruptEditor) SetLocationList(ctx context.Context, req *proto.SetLocationListRequest) (
	*proto.SetLocationListResponse, error,
) {
	res, err := e.EditorServer.SetLocationList(ctx, req)
	e.interruptDraw()
	return res, err

}
func (e *interruptEditor) MoveToNextLocation(ctx context.Context, req *proto.MoveToLocationRequest) (
	*proto.MoveToLocationResponse, error,
) {
	res, err := e.EditorServer.MoveToNextLocation(ctx, req)
	e.interruptDraw()
	return res, err

}
func (e *interruptEditor) MoveToPrevLocation(ctx context.Context, req *proto.MoveToLocationRequest) (
	*proto.MoveToLocationResponse, error,
) {
	res, err := e.EditorServer.MoveToPrevLocation(ctx, req)
	e.interruptDraw()
	return res, err

}
func (e *interruptEditor) Insert(ctx context.Context, req *proto.InsertRequest) (
	*proto.InsertResponse, error,
) {
	res, err := e.EditorServer.Insert(ctx, req)
	e.interruptDraw()
	return res, err

}
func (e *interruptEditor) Delete(ctx context.Context, req *proto.DeleteRequest) (
	*proto.DeleteResponse, error,
) {
	res, err := e.EditorServer.Delete(ctx, req)
	e.interruptDraw()
	return res, err

}
