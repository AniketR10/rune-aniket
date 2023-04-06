package plugin

import (
	"context"
	"fmt"
	"sync"
	"time"

	multierr "github.com/ernestrc/go-multierror"
	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
	"unstable.build/go-tui/config"
	pluginpb "unstable.build/go-tui/plugin/rpc"
	"unstable.build/go-tui/proto"
)

const (
	defDurationGracefulShutClient = 1 * time.Second
)

type granteeServer struct {
	pluginpb.UnimplementedGranteeServer
	mu        sync.Mutex
	req       []Permission
	broker    proto.MuxBroker
	grantee   Grantee
	connected bool
	keepAlive chan struct{}
	srv       *grpc.Server
	ctx       context.Context
	cancelCtx func()

	durationGracefulShut time.Duration
	keepAliveTimeout     time.Duration
}

func newGranteeServer(
	s *grpc.Server, broker proto.MuxBroker,
	grantee Grantee, req []Permission,
	keepAlive time.Duration,
) pluginpb.GranteeServer {
	ret := new(granteeServer)
	ret.broker = broker
	ret.grantee = grantee
	ret.req = req
	ret.srv = s
	ret.ctx, ret.cancelCtx = context.WithCancel(context.Background())
	if keepAlive != time.Duration(0) {
		ret.keepAlive = make(chan struct{})
		ret.keepAliveTimeout = keepAlive * 2
	}
	return ret
}

func forceStopTimer(timer *time.Timer) {
	if timer.Stop() {
		return
	}

	select {
	case <-timer.C:
	default:
	}
}

func (s *granteeServer) monitorKeepAlive() {
	t := time.NewTimer(s.keepAliveTimeout)

	s.mu.Lock()
	ch := s.keepAlive
	s.mu.Unlock()

	for {
		select {
		case <-t.C:
			ctx, cancel := context.WithCancel(context.Background())
			s.doShutdown(ctx, "lost connectivity to host: failed to send a health check in time")
			cancel()
		case <-ch:
			forceStopTimer(t)
			t.Reset(s.keepAliveTimeout)
		}
	}
}

func (s *granteeServer) Permissions(ctx context.Context, req *pluginpb.PermRequest) (
	*pluginpb.PermResponse, error,
) {
	resp := new(pluginpb.PermResponse)
	for _, perm := range s.req {
		resp.Perms = append(resp.Perms, &pluginpb.Permission{Id: string(perm)})
	}

	// Note: this is how Manager sends the config
	// if that ever changes, this code will break
	var cfg config.JSON
	protocfg := req.GetConfig()
	if protocfg == nil {
		cfg = config.JSONFromMap(make(map[string]interface{}))
	} else {
		err := cfg.UnmarshalText(protocfg)
		if err != nil {
			return nil, fmt.Errorf("could not decode incoming plugin config: %v", err)
		}
	}

	s.mu.Lock()
	connected := s.connected
	s.mu.Unlock()

	if !connected {
		s.grantee.Connected(s.broker, cfg)
		s.mu.Lock()
		defer s.mu.Unlock()
		s.connected = true
		if s.keepAlive != nil {
			go s.monitorKeepAlive()
		}
	}

	return resp, nil
}

func (s *granteeServer) OnGrant(ctx context.Context, req *pluginpb.OnPermGrantRequest) (
	*pluginpb.OnPermGrantResponse, error,
) {
	/* only trigger OnPermission* for permissions that were actually requested */

	var denied []Permission
	var granted []Grant
	for _, den := range req.Denied {
		for _, requested := range s.req {
			if string(requested) == den.Id {
				denied = append(denied, Permission(den.Id))
				break
			}
		}
	}
	for _, gr := range req.Granted {
		for _, requested := range s.req {
			if string(requested) == gr.Id {
				granted = append(granted, Grant{
					Token:      gr.Address,
					Permission: Permission(gr.Id),
					Context:    s.ctx,
				})
				break
			}
		}
	}

	if len(granted) != 0 {
		s.grantee.PermissionGranted(granted)
	}
	if len(denied) != 0 {
		s.grantee.PermissionDenied(denied)
	}

	return new(pluginpb.OnPermGrantResponse), nil
}

func (s *granteeServer) doShutdown(ctx context.Context, reason string) (ret error) {
	// idempotent
	s.mu.Lock()
	keepAlive := s.keepAlive
	s.keepAlive = nil
	s.mu.Unlock()

	if keepAlive != nil {
		close(keepAlive)
	}

	defer s.cancelCtx()

	if err := s.grantee.Shutdown(reason); err != nil {
		ret = multierr.Append(ret, err)
	}

	// if this request to shutdown is coming from the wire
	// then cleanup server resources only when ctx is done
	go func() {
		<-ctx.Done()
		s.srv.GracefulStop()
		if err := s.broker.Close(); err != nil {
			ret = multierr.Append(ret, err)
		}
	}()
	return ret
}

func (s *granteeServer) Shutdown(ctx context.Context, in *pluginpb.ShutdownRequest) (
	*pluginpb.ShutdownResponse, error,
) {
	err := s.doShutdown(ctx, in.GetReason())
	if err != nil {
		return nil, err
	}
	return new(pluginpb.ShutdownResponse), nil
}

func (s *granteeServer) Health(context.Context, *pluginpb.HealthRequest) (
	*pluginpb.HealthResponse, error,
) {
	if err := s.grantee.Health(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.keepAlive != nil {
		select {
		case s.keepAlive <- struct{}{}:
		}
	}
	return new(pluginpb.HealthResponse), nil
}

type granteeClient struct {
	mBroker proto.MuxBroker
	client  pluginpb.GranteeClient

	pClient *plugin.Client
}

func newGranteeClient(
	broker proto.MuxBroker, client pluginpb.GranteeClient,
) *granteeClient {
	ret := new(granteeClient)
	ret.client = client
	ret.mBroker = broker
	return ret
}

func (c *granteeClient) broker() proto.MuxBroker {
	return c.mBroker
}

func (c *granteeClient) permissions(ctx context.Context, cfg config.Config) (
	perms []*pluginpb.Permission, err error,
) {
	var req pluginpb.PermRequest
	if cfg == nil {
		req.Config = []byte("{}")
	} else {
		jcfg := config.JSONFromConfig(cfg)
		req.Config, err = jcfg.MarshalText()
		if err != nil {
			err = fmt.Errorf("could not marshal config: %v", err)
			return
		}
	}

	resp, err := c.client.Permissions(ctx, &req)
	if err != nil {
		return nil, err
	}
	return resp.GetPerms(), nil
}

func (c *granteeClient) sendGrants(
	ctx context.Context,
	denied []*pluginpb.Permission,
	granted map[string]*pluginpb.PermissionGrant,
) error {
	req := new(pluginpb.OnPermGrantRequest)

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
	req := pluginpb.HealthRequest{}
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
	ctx, cancelFn := context.WithTimeout(context.Background(), defDurationGracefulShutClient)
	defer cancelFn()

	req := pluginpb.ShutdownRequest{Reason: reason}
	_, err := c.client.Shutdown(ctx, &req)
	return err
}
