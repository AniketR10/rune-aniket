package rpc

import (
	"context"
	"runtime"

	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/browser"
	browserpb "unstable.build/go-tui/browser/rpc"
	"unstable.build/go-tui/proto"
	termpb "unstable.build/go-tui/term/rpc"
	"unstable.build/go-tui/text"
)

var _ text.CommandHandler = (*commandClient)(nil)

type commandClient struct {
	conn proto.MuxConn
	pb   CommandHandlerClient
	s    *Server
}

func newCommandClient(conn proto.MuxConn, s *Server) *commandClient {
	ret := new(commandClient)
	ret.conn = conn
	ret.pb = NewCommandHandlerClient(conn)
	ret.s = s
	runtime.SetFinalizer(ret, func(c *commandClient) { c.Close() })
	return ret
}

func (c *commandClient) HandleCommand(ctx context.Context, cmd textapi.Command) (
	bool, error,
) {

	ctx, cancelFn := context.WithTimeout(ctx, defaultClientTimeout)
	defer cancelFn()

	var cursorContent, cursorWindow termpb.Coordinates
	cursorContent.FromModel(cmd.Cursor.Content)
	cursorWindow.FromModel(cmd.Cursor.Window)

	var uri *URI
	if cmd.URI != (workspaceapi.URI{}) {
		uri = NewURI(cmd.URI)
	}

	adapter := cmd.Window.(browser.WindowToAPIWindow)
	channelID, err := c.s.browser.ServeWindow(adapter.Win)
	if err != nil {
		return false, err
	}

	req := HandleCommandRequest{
		Name:            cmd.Name,
		Args:            cmd.Args,
		ResourceName:    uri,
		CursorContent:   &cursorContent,
		CursorWindow:    &cursorWindow,
		WindowId:        adapter.Win.ID(),
		WindowChannelId: channelID,
	}

	// do not block waiting for I/O
	c.s.editor.Unlock()
	defer c.s.editor.Lock()

	resp, err := c.pb.HandleCommand(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return false, err
	}
	return resp.GetExit(), nil
}

func (c *commandClient) Close() error {
	err := c.conn.Close()
	runtime.SetFinalizer(c, nil)
	return err
}

type commandServer struct {
	UnimplementedCommandHandlerServer
	h       textapi.CommandHandler
	browser *browserpb.Client
}

func newCommandServer(h textapi.CommandHandler, bc *browserpb.Client) *commandServer {
	ret := new(commandServer)
	ret.h = h
	ret.browser = bc
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
	e.Window, err = s.browser.DialWindow(pe.GetWindowChannelId(), uint64(pe.GetWindowId()))
	return err
}

func (s *commandServer) HandleCommand(
	ctx context.Context, req *HandleCommandRequest,
) (*HandleCommandResponse, error) {
	var cmd textapi.Command
	err := s.commandFromProto(&cmd, req)
	if err != nil {
		return nil, err
	}

	exit, err := s.h.HandleCommand(ctx, cmd)
	if err != nil {
		return nil, err
	}
	res := new(HandleCommandResponse)
	res.Exit = exit
	return res, nil
}
