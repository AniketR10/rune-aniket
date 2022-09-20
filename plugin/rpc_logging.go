package plugin

import (
	"context"

	pluginpb "github.com/ernestrc/go-tui/plugin/rpc"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type loggingGranteeServer struct {
	*log.Logger
	pluginpb.GranteeServer
}

func (s *loggingGranteeServer) Permissions(
	ctx context.Context, in *pluginpb.PermRequest,
) (*pluginpb.PermResponse, error) {
	res, err := s.GranteeServer.Permissions(ctx, in)
	s.Tracef("GranteeServer.Permissions(%+v): (%+v, %v)", in, res, err)
	return res, err
}

func (s *loggingGranteeServer) OnGrant(
	ctx context.Context, in *pluginpb.OnPermGrantRequest,
) (*pluginpb.OnPermGrantResponse, error) {
	res, err := s.GranteeServer.OnGrant(ctx, in)
	s.Tracef("GranteeServer.OnGrant(%+v): (%+v, %v)", in, res, err)
	return res, err
}

func (s *loggingGranteeServer) Shutdown(
	ctx context.Context, in *pluginpb.ShutdownRequest,
) (*pluginpb.ShutdownResponse, error) {
	res, err := s.GranteeServer.Shutdown(ctx, in)
	s.Tracef("GranteeServer.Shutdown(%+v): (%+v, %v)", in, res, err)
	return res, err
}

func (s *loggingGranteeServer) Health(
	ctx context.Context, in *pluginpb.HealthRequest,
) (*pluginpb.HealthResponse, error) {
	res, err := s.GranteeServer.Health(ctx, in)
	s.Tracef("GranteeServer.Health(%+v): (%+v, %v)", in, res, err)
	return res, err
}

type loggingGranteeClient struct {
	*log.Logger
	pluginpb.GranteeClient
}

func (c *loggingGranteeClient) Permissions(
	ctx context.Context, in *pluginpb.PermRequest, opts ...grpc.CallOption,
) (*pluginpb.PermResponse, error) {
	res, err := c.GranteeClient.Permissions(ctx, in, opts...)
	c.Tracef("GranteeClient.Permissions(%+v): (%+v, %v)", in, res, err)
	return res, err
}

func (c *loggingGranteeClient) OnGrant(
	ctx context.Context, in *pluginpb.OnPermGrantRequest, opts ...grpc.CallOption,
) (*pluginpb.OnPermGrantResponse, error) {
	res, err := c.GranteeClient.OnGrant(ctx, in, opts...)
	c.Tracef("GranteeClient.OnGrant(%+v): (%+v, %v)", in, res, err)
	return res, err
}

func (c *loggingGranteeClient) Shutdown(
	ctx context.Context, in *pluginpb.ShutdownRequest, opts ...grpc.CallOption,
) (*pluginpb.ShutdownResponse, error) {
	res, err := c.GranteeClient.Shutdown(ctx, in, opts...)
	c.Tracef("GranteeClient.Shutdown(%+v): (%+v, %v)", in, res, err)
	return res, err
}

func (c *loggingGranteeClient) Health(
	ctx context.Context, in *pluginpb.HealthRequest, opts ...grpc.CallOption,
) (*pluginpb.HealthResponse, error) {
	res, err := c.GranteeClient.Health(ctx, in, opts...)
	c.Tracef("GranteeClient.Health(%+v): (%+v, %v)", in, res, err)
	return res, err
}
