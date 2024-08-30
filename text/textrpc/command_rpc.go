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

package textrpc

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/blue/logging"
	status "google.golang.org/grpc/status"
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/browser/browserrpc"
	"unstable.build/go-tui/rpc"
	termrpc "unstable.build/go-tui/term/termrpc"
	"unstable.build/go-tui/text"
)

var _ text.CommandHandler = (*commandClient)(nil)

const defaultClientTimeout = 5 * time.Second

type commandClient struct {
	conn rpc.MuxConn
	pb   CommandHandlerClient
	s    *Server
}

func newCommandClient(conn rpc.MuxConn, s *Server) *commandClient {
	ret := new(commandClient)
	ret.conn = conn
	ret.pb = NewCommandHandlerClient(conn)
	ret.s = s
	runtime.SetFinalizer(ret, func(c *commandClient) { c.Close() })
	return ret
}

func (c *commandClient) Complete(ctx context.Context, name string, args []string) (
	iterator.Iterator[string], string, error,
) {
	c.log(log.TraceLevel, "complete command: %s", name)

	ctx, cancelFn := context.WithTimeout(ctx, defaultClientTimeout)

	req := CompleteRequest{
		Name: name,
		Args: args,
	}

	// do not block waiting for I/O
	c.s.editor.Unlock()
	defer c.s.editor.Lock()

	client, err := c.pb.Complete(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		c.log(log.TraceLevel, "complete command %s error: %v", name, err)
		cancelFn()
		return nil, "", fmt.Errorf("complete command rpc: %w", err)
	}

	const neverReplaceArgs = ""
	return &completeClientIterator{
		cancelFn: cancelFn,
		parent:   c,
		client:   client,
	}, neverReplaceArgs, nil
}

func (c *commandClient) HandleCommand(ctx context.Context, cmd textapi.Command) error {
	c.log(log.TraceLevel, "handle command: %s", cmd.Name)

	ctx, cancelFn := context.WithTimeout(ctx, defaultClientTimeout)
	defer cancelFn()

	var cursorContent, cursorWindow termrpc.Coordinates
	cursorContent.FromModel(cmd.Cursor.Content)
	cursorWindow.FromModel(cmd.Cursor.Window)

	var uri *URI
	if cmd.URI != (workspaceapi.URI{}) {
		uri = NewURI(cmd.URI)
	}

	adapter := cmd.Window.(browser.Window)

	req := HandleCommandRequest{
		Name:          cmd.Name,
		Args:          cmd.Args,
		ResourceName:  uri,
		CursorContent: &cursorContent,
		CursorWindow:  &cursorWindow,
		WindowId:      adapter.ID(),
	}

	// do not block waiting for I/O
	c.s.editor.Unlock()
	defer c.s.editor.Lock()

	_, err := c.pb.HandleCommand(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		c.log(log.TraceLevel, "handle command %s error: %v", cmd.Name, err)
		if st, ok := status.FromError(err); ok {
			return errors.New(strings.Trim(st.Message(), "\n"))
		}
		return err
	}
	return nil
}
func (c *commandClient) log(level log.Level, msg string, args ...interface{}) {
	if !log.IsLevelEnabled(level) {
		return
	}
	log.WithFields(log.Fields{logging.KeyClass: "textrpc.commandClient"}).
		Logf(level, msg, args...)
}

func (c *commandClient) Close() error {
	err := c.conn.Close()
	c.log(log.TraceLevel, "close called: err=%v", err)
	runtime.SetFinalizer(c, nil)
	return err
}

type commandServer struct {
	UnimplementedCommandHandlerServer
	h       textapi.CommandHandler
	browser *browserrpc.Client
	// delay client finalizer until commandServer is GC'd
	c *Client
}

func newCommandServer(h textapi.CommandHandler, bc *browserrpc.Client, c *Client) *commandServer {
	ret := new(commandServer)
	ret.h = h
	ret.browser = bc
	ret.c = c
	return ret
}

func (s *commandServer) commandFromProto(
	e *textapi.Command, pe *HandleCommandRequest,
) (err error) {
	if pe.ResourceName != nil {
		e.URI, err = NewURIFromProto(pe.GetResourceName())
		if err != nil {
			return
		}
		e.Resource = Token{
			URI: e.URI,
		}
	}
	e.Cursor.Window = pe.GetCursorWindow().ToModel()
	e.Cursor.Content = pe.GetCursorContent().ToModel()
	e.Args = pe.GetArgs()
	e.Name = pe.GetName()
	e.Window, err = s.browser.DialWindow(uint64(pe.GetWindowId()))
	return err
}

func (s *commandServer) Complete(
	req *CompleteRequest, srv CommandHandler_CompleteServer,
) error {
	if req.Name == "" {
		return errors.New("invalid request: missing command name")
	}

	s.log(log.TraceLevel, "streaming completer's iterator for %s %v",
		req.Name, req.Args)

	defer s.log(log.TraceLevel, "done streaming completer's iterator for %s %v",
		req.Name, req.Args)

	completer, err := s.h.Complete(srv.Context(), req.Name, req.Args)
	if err != nil {
		return err
	}

	for {
		next, ok := completer.Next()
		if !ok {
			break
		}
		resp := CompleteResponse{Value: next}
		if err := srv.Send(&resp); err != nil {
			return fmt.Errorf("send completion msg: %w", err)
		}
	}

	resp := CompleteResponse{Done: true}
	if err := completer.Err(); err != nil {
		resp.Error = err.Error()
	}

	if err := srv.Send(&resp); err != nil {
		return fmt.Errorf("send last completion msg: %w", err)
	}

	return nil
}

func (s *commandServer) HandleCommand(
	ctx context.Context, req *HandleCommandRequest,
) (*HandleCommandResponse, error) {
	s.log(log.TraceLevel, "handle command %+v", req)

	var cmd textapi.Command
	err := s.commandFromProto(&cmd, req)
	if err != nil {
		return nil, err
	}

	if err := s.h.HandleCommand(ctx, cmd); err != nil {
		return nil, err
	}
	res := new(HandleCommandResponse)
	return res, nil
}

func (c *commandServer) log(level log.Level, msg string, args ...interface{}) {
	log.WithFields(log.Fields{logging.KeyClass: "textrpc.commandServer"}).
		Logf(level, msg, args...)
}

type completeClientIterator struct {
	parent   *commandClient
	client   CommandHandler_CompleteClient
	err      error
	cancelFn func()
}

func (c *completeClientIterator) Next() (string, bool) {
	res, err := c.client.Recv()
	if err != nil {
		c.err = err
		c.cancelFn()
		return "", false
	}
	if res.Done {
		c.cancelFn()
	}
	return res.Value, !res.Done
}

func (c *completeClientIterator) Err() error {
	return c.err
}
