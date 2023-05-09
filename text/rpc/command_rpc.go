package rpc

import (
	"context"
	"runtime"
	"time"

	"github.com/ernestrc/blue/logging"
	log "github.com/sirupsen/logrus"
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/browser"
	browserpb "unstable.build/go-tui/browser/rpc"
	"unstable.build/go-tui/proto"
	termpb "unstable.build/go-tui/term/rpc"
	"unstable.build/go-tui/text"
)

var _ text.CommandHandler = (*commandClient)(nil)

const defaultClientTimeout = 2 * time.Second

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
	c.log(log.TraceLevel, "handle command: %s", cmd.Name)

	ctx, cancelFn := context.WithTimeout(ctx, defaultClientTimeout)
	defer cancelFn()

	var cursorContent, cursorWindow termpb.Coordinates
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

	resp, err := c.pb.HandleCommand(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		c.log(log.TraceLevel, "handle command %s error: %v", cmd.Name, err)
		return false, err
	}
	return resp.GetExit(), nil
}
func (c *commandClient) log(level log.Level, msg string, args ...interface{}) {
	log.WithFields(log.Fields{logging.KeyClass: "textpb.commandClient"}).
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
	browser *browserpb.Client
	// delay client finalizer until commandServer is GC'd
	c *Client
}

func newCommandServer(h textapi.CommandHandler, bc *browserpb.Client, c *Client) *commandServer {
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
