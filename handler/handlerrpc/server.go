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

package handlerrpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync/atomic"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	"unstable.build/go-tui"
	"unstable.build/go-tui/term/termrpc"
)

type dimensions struct {
	width  int
	height int
}

// Server serves a tui.Handler implementation over GRPC.
type Server struct {
	UnimplementedHandlerServer
	// NOTE: do not synchronize access to handle
	// explicitly, and instead rely on clients to do it.
	// This is necessary because calls to Handle can call APIs
	// which might then trigger another method to be called, i.e. Close, which
	// creates a deadlock.
	handler    tui.Handler
	dimensions atomic.Value
}

// NewServer allocates storage for a new Server and initializes it.
func NewServer(handler tui.Handler) *Server {
	ret := new(Server)
	ret.Init(handler)
	return ret
}

// Init initializes this Server to serve handler.
func (s *Server) Init(handler tui.Handler) {
	s.handler = handler
	s.dimensions.Store(dimensions{})
}

// Draw is an RPC that handles request to an underlying Handler's
// Draw over RPC.
func (s *Server) Draw(ctx context.Context, in *DrawRequest) (
	*DrawResponse, error,
) {
	dim := s.dimensions.Load().(dimensions)
	if int(in.Width) != dim.width || int(in.Height) != dim.height {
		s.handler.Resize(int(in.Width), int(in.Height))
	}
	s.dimensions.Store(dimensions{width: int(in.Width), height: int(in.Height)})
	cursor, style, show := s.handler.Cursor()
	res := NewDrawResponse(ctx, s.handler, int(in.Width), int(in.Height))
	res.Cursor.Position.X = int32(cursor.X)
	res.Cursor.Position.Y = int32(cursor.Y)
	res.Cursor.Show = show
	res.Cursor.Style = int32(style)
	return res, nil
}

func (s *Server) log(level log.Level, msg string, args ...interface{}) {
	if !log.IsLevelEnabled(level) {
		return
	}
	log.WithField(logging.KeyClass, "handler.Server").Logf(level, msg, args...)
}

// Handle is an RPC that handles request to an underlying Handler's
// Handle over RPC.
func (s *Server) Handle(ctx context.Context, req *HandleRequest) (
	*HandleResponse, error,
) {
	pev := req.GetEvent()
	if pev == nil {
		return nil, errors.New("missing Event field in HandleRequest")
	}
	ev, err := pev.ToModel()
	if err != nil {
		return nil, err
	}

	s.log(log.TraceLevel, "handler.Server.Handle(%v)", ev)

	var exit, handled bool
	exit, handled = s.handler.Handle(ev)

	return &HandleResponse{
		Quit:    exit,
		Handled: handled,
	}, nil
}

// Man is an RPC that handles request to an underlying
// Handler's Man over RPC.
func (s *Server) Man(context.Context, *ManRequest) (
	*ManResponse, error,
) {
	man := s.handler.Man()
	protoMan := new(termrpc.Manual)
	err := protoMan.FromModel(man)
	if err != nil {
		return nil, err
	}
	return &ManResponse{Man: protoMan}, nil
}

// Close is an RPC that handles request to an underlying
// Handler's Close if it implements it, otherwise it ignores request.
func (s *Server) Close(ctx context.Context, req *CloseRequest) (
	*CloseResponse, error,
) {
	closer, ok := s.handler.(io.Closer)
	if ok {
		err := closer.Close()
		if err != nil {
			return nil, fmt.Errorf("error Close: %v", err)
		}
	}
	s.log(log.DebugLevel, "handler.Server.Close: %v", ok)
	return &CloseResponse{}, nil
}
