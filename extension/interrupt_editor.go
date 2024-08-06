// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package extension

import (
	"context"

	textpb "unstable.build/go-tui/text/rpc"
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

func (e *interruptEditor) Subscribe(stream textpb.Editor_SubscribeServer) error {
	return e.EditorServer.Subscribe(stream)
}

func (e *interruptEditor) Register(ctx context.Context, req *textpb.RegisterCommandRequest) (
	*textpb.RegisterCommandResponse, error,
) {
	res, err := e.EditorServer.Register(ctx, req)
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
