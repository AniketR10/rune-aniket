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

package rpc

import (
	"context"
	"io"

	"google.golang.org/grpc"
	"unstable.build/go-tui"
	"unstable.build/go-tui/term"
	termpb "unstable.build/go-tui/term/rpc"
)

type mockHandlerClient struct {
	handledCh chan term.Event
	quitCh    chan struct{}
	rpcError  error
	remote    tui.Handler
}

func (c *mockHandlerClient) Draw(
	ctx context.Context, in *DrawRequest, opts ...grpc.CallOption,
) (*DrawResponse, error) {
	if c.rpcError != nil {
		return nil, c.rpcError
	}
	width, height := int(in.GetWidth()), int(in.GetHeight())
	c.remote.Resize(width, height)
	pos, style, show := c.remote.Cursor()

	resp := NewDrawResponse(context.Background(), c.remote, width, height)
	resp.Cursor = &DrawResponse_Cursor{
		Position: &termpb.Coordinates{
			X: int32(pos.X),
			Y: int32(pos.Y),
		},
		Show:  show,
		Style: int32(style),
	}
	return resp, nil
}

func (c *mockHandlerClient) Handle(
	ctx context.Context, in *HandleRequest, opts ...grpc.CallOption,
) (*HandleResponse, error) {
	if c.rpcError != nil {
		return nil, c.rpcError
	}

	resp := new(HandleResponse)
	ev, err := in.GetEvent().ToModel()
	if err != nil {
		return nil, err
	}

	// mimic server
	if ev.Type == term.EventInterrupt {
		return resp, err
	}

	resp.Quit, resp.Handled = c.remote.Handle(ev)
	if c.handledCh != nil {
		select {
		case c.handledCh <- ev:
		case <-c.quitCh:
		}
	}

	return resp, nil
}

func (c *mockHandlerClient) Man(
	ctx context.Context, in *ManRequest, opts ...grpc.CallOption,
) (*ManResponse, error) {
	if c.rpcError != nil {
		return nil, c.rpcError
	}

	man := c.remote.Man()
	protoMan := termpb.Manual{}
	protoMan.FromModel(man)

	return &ManResponse{Man: &protoMan}, nil
}

func (c *mockHandlerClient) Close(
	ctx context.Context, in *CloseRequest, opts ...grpc.CallOption,
) (*CloseResponse, error) {
	if c.rpcError != nil {
		return nil, c.rpcError
	}
	if closer, ok := c.remote.(io.Closer); ok {
		err := closer.Close()
		if err != nil {
			return nil, err
		}
	}
	return &CloseResponse{}, nil
}
