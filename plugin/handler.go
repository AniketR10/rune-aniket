package plugin

import (
	"context"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/proto"
	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
)

// HandlerPlugin satisfies plugin.GRPCPlugin
type HandlerPlugin struct {
	plugin.Plugin
	tui.Handler
}

// GRPCServer satisfies plugin.GRPCPlugin
func (p *HandlerPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	server := handler.NewServer(p.Handler)
	proto.RegisterHandlerServer(s, server)
	return nil
}

// GRPCClient satisfies plugin.GRPCPlugin
func (p *HandlerPlugin) GRPCClient(
	ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn,
) (interface{}, error) {
	return handler.NewClient(proto.NewHandlerClient(c)), nil
}
