package plugin

import (
	"context"

	"github.com/ernestrc/go-tui/proto"
	"github.com/hashicorp/go-plugin"
)

type granteeServer struct {
	req       []Permission
	broker    proto.MuxBroker
	grantee   Grantee
	connected bool
}

func newGranteeServer(
	broker proto.MuxBroker, grantee Grantee, req []Permission,
) proto.GranteeServer {
	ret := new(granteeServer)
	ret.broker = broker
	ret.grantee = grantee
	ret.req = req
	return ret
}

func (s *granteeServer) Permissions(context.Context, *proto.PermRequest) (
	*proto.PermResponse, error,
) {
	resp := new(proto.PermResponse)
	for _, perm := range s.req {
		resp.Perms = append(resp.Perms, &proto.Permission{Id: string(perm)})
	}

	if !s.connected {
		s.connected = true
		s.grantee.OnConnected(s.broker)
	}

	return resp, nil
}

func (s *granteeServer) OnGrant(ctx context.Context, req *proto.OnPermGrantRequest) (
	*proto.OnPermGrantResponse, error,
) {
	/* only trigger OnPermission* for permissions that were actually requested */

	for _, denied := range req.Denied {
		for _, requested := range s.req {
			if string(requested) == denied.Id {
				s.grantee.OnPermissionDenied(Permission(denied.Id))
			}
		}
	}
	for _, granted := range req.Granted {
		for _, requested := range s.req {
			if string(requested) == granted.Id {
				s.grantee.OnPermissionGranted(granted.GrantId, Permission(granted.Id))
			}
		}
	}

	return new(proto.OnPermGrantResponse), nil
}

func (s *granteeServer) Shutdown(context.Context, *proto.ShutdownRequest) (
	*proto.ShutdownResponse, error,
) {
	if err := s.grantee.OnShutdown(); err != nil {
		return nil, err
	}
	return new(proto.ShutdownResponse), nil
}

func (s *granteeServer) Health(context.Context, *proto.HealthRequest) (
	*proto.HealthResponse, error,
) {
	if err := s.grantee.Health(); err != nil {
		return nil, err
	}
	return new(proto.HealthResponse), nil
}

type granteeClient struct {
	mBroker proto.MuxBroker
	client  proto.GranteeClient

	pClient *plugin.Client
}

func newGranteeClient(
	broker proto.MuxBroker, client proto.GranteeClient,
) *granteeClient {
	ret := new(granteeClient)
	ret.client = client
	ret.mBroker = broker
	return ret
}

func (c *granteeClient) broker() proto.MuxBroker {
	return c.mBroker
}

func (c *granteeClient) permissions(ctx context.Context) (
	[]*proto.Permission, error,
) {
	req := proto.PermRequest{}

	resp, err := c.client.Permissions(ctx, &req)
	if err != nil {
		return nil, err
	}
	return resp.GetPerms(), nil
}

func (c *granteeClient) sendGrants(
	ctx context.Context,
	denied []*proto.Permission,
	granted []*proto.PermissionGrant,
) error {
	req := new(proto.OnPermGrantRequest)

	for _, dn := range denied {
		req.Denied = append(req.Denied, dn)
	}

	for _, gr := range granted {
		req.Granted = append(req.Granted, gr)
	}

	_, err := c.client.OnGrant(ctx, req)
	return err
}

func (c *granteeClient) health(ctx context.Context) error {
	req := proto.HealthRequest{}
	_, err := c.client.Health(ctx, &req)
	return err
}

func (c *granteeClient) bindPluginClient(pc *plugin.Client) {
	if c.pClient != nil {
		panic("trying to bind to two clients")
	}
	c.pClient = pc
}

func (c *granteeClient) shutdown(reason string) error {
	if c.pClient != nil {
		defer func() {
			c.pClient.Kill()
			c.pClient = nil
		}()
	}
	ctx := context.Background()
	req := proto.ShutdownRequest{Reason: reason}
	_, err := c.client.Shutdown(ctx, &req)

	if broker := c.broker(); broker != nil {
		brokerErr := broker.Close()
		if brokerErr != nil {
			return brokerErr
		}
	}
	return err
}
