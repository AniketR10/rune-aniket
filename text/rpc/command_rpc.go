package rpc

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/proto"
	termpb "unstable.build/go-tui/term/rpc"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/workspace"
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
	return ret
}

func (c *commandClient) HandleCommand(ctx context.Context, cmd text.Command) (
	bool, error,
) {
	var resourceID uint32
	var uri *URI
	// only overwrite resource if this event is for a particular resource
	if cmd.URI != (workspace.URI{}) {
		brokerID, ok := c.s.uriToID[cmd.URI.String()]
		if !ok {
			err := fmt.Errorf("could not handle command %v: handler with resource name %q not found",
				cmd.Name, cmd.URI.String())
			c.s.tryLog(log.WarnLevel, "%s", err)
			return false, err
		}
		resourceID = brokerID
		uri = NewURI(cmd.URI)
	}

	ctx, cancelFn := context.WithTimeout(ctx, defaultClientTimeout)
	defer cancelFn()

	var content, window termpb.Coordinates
	content.FromModel(cmd.Cursor.Content)
	window.FromModel(cmd.Cursor.Window)

	req := HandleCommandRequest{
		Name:         cmd.Name,
		Args:         cmd.Args,
		ResourceName: uri,
		ResourceId:   resourceID,
		Content:      &content,
		Window:       &window,
	}

	// do not block waiting for I/O
	c.s.editor.Unlock()
	defer c.s.editor.Lock()

	resp, err := c.pb.HandleCommand(ctx, &req)
	if err != nil {
		return false, err
	}
	return resp.GetExit(), nil
}

func (c *commandClient) Close() error {
	return c.conn.Close()
}

type commandServer struct {
	UnimplementedCommandHandlerServer
	h text.CommandHandler
}

func newCommandServer(h text.CommandHandler) *commandServer {
	ret := new(commandServer)
	ret.h = h
	return ret
}

func (s *commandServer) HandleCommand(
	ctx context.Context, req *HandleCommandRequest,
) (*HandleCommandResponse, error) {
	var cmd text.Command
	err := commandFromProto(&cmd, req)
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
