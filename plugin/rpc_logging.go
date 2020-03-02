package plugin

import (
	"context"

	"github.com/ernestrc/go-tui/proto"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type loggingGranteeServer struct {
	*log.Logger
	proto.GranteeServer
}

func (s *loggingGranteeServer) Permissions(
	ctx context.Context, in *proto.PermRequest,
) (*proto.PermResponse, error) {
	res, err := s.GranteeServer.Permissions(ctx, in)
	s.Tracef("GranteeServer.Permissions(%+v): (%+v, %v)", in, res, err)
	return res, err
}

func (s *loggingGranteeServer) OnGrant(
	ctx context.Context, in *proto.OnPermGrantRequest,
) (*proto.OnPermGrantResponse, error) {
	res, err := s.GranteeServer.OnGrant(ctx, in)
	s.Tracef("GranteeServer.OnGrant(%+v): (%+v, %v)", in, res, err)
	return res, err
}

func (s *loggingGranteeServer) Shutdown(
	ctx context.Context, in *proto.ShutdownRequest,
) (*proto.ShutdownResponse, error) {
	res, err := s.GranteeServer.Shutdown(ctx, in)
	s.Tracef("GranteeServer.Shutdown(%+v): (%+v, %v)", in, res, err)
	return res, err
}

func (s *loggingGranteeServer) Health(
	ctx context.Context, in *proto.HealthRequest,
) (*proto.HealthResponse, error) {
	res, err := s.GranteeServer.Health(ctx, in)
	s.Tracef("GranteeServer.Health(%+v): (%+v, %v)", in, res, err)
	return res, err
}

type loggingGranteeClient struct {
	*log.Logger
	proto.GranteeClient
}

func (c *loggingGranteeClient) Permissions(
	ctx context.Context, in *proto.PermRequest, opts ...grpc.CallOption,
) (*proto.PermResponse, error) {
	res, err := c.GranteeClient.Permissions(ctx, in, opts...)
	c.Tracef("GranteeClient.Permissions(%+v): (%+v, %v)", in, res, err)
	return res, err
}

func (c *loggingGranteeClient) OnGrant(
	ctx context.Context, in *proto.OnPermGrantRequest, opts ...grpc.CallOption,
) (*proto.OnPermGrantResponse, error) {
	res, err := c.GranteeClient.OnGrant(ctx, in, opts...)
	c.Tracef("GranteeClient.OnGrant(%+v): (%+v, %v)", in, res, err)
	return res, err
}

func (c *loggingGranteeClient) Shutdown(
	ctx context.Context, in *proto.ShutdownRequest, opts ...grpc.CallOption,
) (*proto.ShutdownResponse, error) {
	res, err := c.GranteeClient.Shutdown(ctx, in, opts...)
	c.Tracef("GranteeClient.Shutdown(%+v): (%+v, %v)", in, res, err)
	return res, err
}

func (c *loggingGranteeClient) Health(
	ctx context.Context, in *proto.HealthRequest, opts ...grpc.CallOption,
) (*proto.HealthResponse, error) {
	res, err := c.GranteeClient.Health(ctx, in, opts...)
	c.Tracef("GranteeClient.Health(%+v): (%+v, %v)", in, res, err)
	return res, err
}
