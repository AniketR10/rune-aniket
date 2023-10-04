package process

import (
	"context"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	extensionpb "unstable.build/go-tui/extension/rpc"
)

type loggingGranteeServer struct {
	extensionpb.GranteeServer
}

func (s *loggingGranteeServer) Permissions(
	ctx context.Context, in *extensionpb.PermRequest,
) (*extensionpb.PermResponse, error) {
	res, err := s.GranteeServer.Permissions(ctx, in)
	log.Tracef("GranteeServer.Permissions(%+v): (%+v, %v)", in, res, err)
	return res, err
}

func (s *loggingGranteeServer) OnGrant(
	ctx context.Context, in *extensionpb.OnPermGrantRequest,
) (*extensionpb.OnPermGrantResponse, error) {
	res, err := s.GranteeServer.OnGrant(ctx, in)
	log.Tracef("GranteeServer.OnGrant(%+v): (%+v, %v)", in, res, err)
	return res, err
}

func (s *loggingGranteeServer) Shutdown(
	ctx context.Context, in *extensionpb.ShutdownRequest,
) (*extensionpb.ShutdownResponse, error) {
	res, err := s.GranteeServer.Shutdown(ctx, in)
	log.Tracef("GranteeServer.Shutdown(%+v): (%+v, %v)", in, res, err)
	return res, err
}

func (s *loggingGranteeServer) Health(
	ctx context.Context, in *extensionpb.HealthRequest,
) (*extensionpb.HealthResponse, error) {
	res, err := s.GranteeServer.Health(ctx, in)
	log.Tracef("GranteeServer.Health(%+v): (%+v, %v)", in, res, err)
	return res, err
}

type loggingGranteeClient struct {
	extensionpb.GranteeClient
}

func (c *loggingGranteeClient) Permissions(
	ctx context.Context, in *extensionpb.PermRequest, opts ...grpc.CallOption,
) (*extensionpb.PermResponse, error) {
	res, err := c.GranteeClient.Permissions(ctx, in, opts...)
	log.Tracef("GranteeClient.Permissions(%+v): (%+v, %v)", in, res, err)
	return res, err
}

func (c *loggingGranteeClient) OnGrant(
	ctx context.Context, in *extensionpb.OnPermGrantRequest, opts ...grpc.CallOption,
) (*extensionpb.OnPermGrantResponse, error) {
	res, err := c.GranteeClient.OnGrant(ctx, in, opts...)
	log.Tracef("GranteeClient.OnGrant(%+v): (%+v, %v)", in, res, err)
	return res, err
}

func (c *loggingGranteeClient) Shutdown(
	ctx context.Context, in *extensionpb.ShutdownRequest, opts ...grpc.CallOption,
) (*extensionpb.ShutdownResponse, error) {
	res, err := c.GranteeClient.Shutdown(ctx, in, opts...)
	log.Tracef("GranteeClient.Shutdown(%+v): (%+v, %v)", in, res, err)
	return res, err
}

func (c *loggingGranteeClient) Health(
	ctx context.Context, in *extensionpb.HealthRequest, opts ...grpc.CallOption,
) (*extensionpb.HealthResponse, error) {
	res, err := c.GranteeClient.Health(ctx, in, opts...)
	log.Tracef("GranteeClient.Health(%+v): (%+v, %v)", in, res, err)
	return res, err
}
