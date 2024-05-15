package process

import (
	"context"
	"fmt"
	"sync"
	"time"

	multierr "github.com/ernestrc/go-multierror"
	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
	"unstable.build/go-tui/api/config"
	"unstable.build/go-tui/extension"
	extensionpb "unstable.build/go-tui/extension/rpc"
	"unstable.build/go-tui/rpc"
)

const (
	defDurationGracefulShutClient = 5 * time.Second
)

type granteeServer struct {
	extensionpb.UnimplementedGranteeServer
	mu        sync.Mutex
	req       []extension.Permission
	broker    rpc.MuxBroker
	grantee   extension.Grantee
	connected bool
	keepAlive chan struct{}
	srv       *grpc.Server
	ctx       context.Context
	cancelCtx func()
	closeWg   sync.WaitGroup

	keepAliveTimeout     time.Duration
}

func newGranteeServer(
	s *grpc.Server, broker rpc.MuxBroker,
	grantee extension.Grantee, req []extension.Permission,
	keepAlive time.Duration,
) extensionpb.GranteeServer {
	ret := new(granteeServer)
	ret.broker = broker
	ret.grantee = grantee
	ret.req = req
	ret.srv = s
	ret.ctx, ret.cancelCtx = context.WithCancel(context.Background())
	ret.ctx = rpc.ContextWithWaitGroup(ret.ctx, &ret.closeWg)
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
			ctx, cancel := context.WithTimeout(s.ctx, s.keepAliveTimeout)
			defer cancel()
			defer forceStopTimer(t)
			_ = s.doShutdown(ctx, "lost connectivity to host: failed to send a health check in time")
			return
		case <-ch:
			forceStopTimer(t)
			t.Reset(s.keepAliveTimeout)
		}
	}
}

func (s *granteeServer) Permissions(ctx context.Context, req *extensionpb.PermRequest) (
	*extensionpb.PermResponse, error,
) {
	resp := new(extensionpb.PermResponse)
	for _, perm := range s.req {
		resp.Perms = append(resp.Perms, &extensionpb.Permission{Id: string(perm)})
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
			return nil, fmt.Errorf("could not decode incoming extension config: %v", err)
		}
	}

	s.mu.Lock()
	connected := s.connected
	s.mu.Unlock()

	if !connected {
		if err := s.grantee.Connected(ctx, s.broker, cfg); err != nil {
			return nil, fmt.Errorf("grantee connected: %w", err)
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		s.connected = true
		if s.keepAlive != nil {
			go s.monitorKeepAlive()
		}
	}

	return resp, nil
}

func (s *granteeServer) OnGrant(ctx context.Context, req *extensionpb.OnPermGrantRequest) (
	*extensionpb.OnPermGrantResponse, error,
) {
	/* only trigger OnPermission* for permissions that were actually requested */

	var denied []extension.Permission
	var granted []extension.Grant
	for _, den := range req.Denied {
		for _, requested := range s.req {
			if string(requested) == den.Id {
				denied = append(denied, extension.Permission(den.Id))
				break
			}
		}
	}
	for _, gr := range req.Granted {
		for _, requested := range s.req {
			if string(requested) == gr.Id {
				granted = append(granted, extension.Grant{
					Token:      gr.Address,
					Permission: extension.Permission(gr.Id),
					Context:    s.ctx,
				})
				break
			}
		}
	}

	if len(granted) != 0 {
		if err := s.grantee.PermissionGranted(ctx, granted); err != nil {
			return nil, fmt.Errorf("grantee permissions granted: %w", err)
		}
	}
	if len(denied) != 0 {
		if err := s.grantee.PermissionDenied(ctx, denied); err != nil {
			return nil, fmt.Errorf("grantee permissions denied: %w", err)
		}
	}

	return new(extensionpb.OnPermGrantResponse), nil
}

func (s *granteeServer) doShutdown(ctx context.Context, reason string) (ret error) {
	s.mu.Lock()
	keepAlive := s.keepAlive
	s.keepAlive = nil
	cancelCtx := s.cancelCtx
	s.cancelCtx = nil
	s.mu.Unlock()

	// idempotent
	if keepAlive != nil {
		close(keepAlive)
	}

	if err := s.broker.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}

	// NOTE: protect against extensions with poor synchronization which could block
	// indefinetely, then grantee server would timeout and send a kill signal.
	// This would be fine, except that we use unix sockets that need to be
	// cleaned up.
	t := time.After(defDurationGracefulShutClient / 3)
	done := make(chan struct{})
	go func() {
		if err := s.grantee.Shutdown(ctx, reason); err != nil {
			ret = multierr.Append(ret, err)
		}
		done <- struct{}{}
	}()

	select {
	case <-t:
	case <-done:
	}
	// wait for all resources to close
	if cancelCtx != nil {
		cancelCtx()
	}
	s.closeWg.Wait()

	// complete rpc without blocking
	go s.srv.GracefulStop()

	return ret
}

func (s *granteeServer) Shutdown(ctx context.Context, in *extensionpb.ShutdownRequest) (
	*extensionpb.ShutdownResponse, error,
) {
	err := s.doShutdown(ctx, in.GetReason())
	if err != nil {
		return nil, err
	}
	return new(extensionpb.ShutdownResponse), nil
}

func (s *granteeServer) Health(ctx context.Context, req *extensionpb.HealthRequest) (
	*extensionpb.HealthResponse, error,
) {
	if err := s.grantee.Health(ctx); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.keepAlive != nil {
		s.keepAlive <- struct{}{}
	}
	return new(extensionpb.HealthResponse), nil
}

type granteeClient struct {
	mBroker rpc.MuxBroker
	client  extensionpb.GranteeClient

	pClient *goplugin.Client
}

func newGranteeClient(
	broker rpc.MuxBroker, client extensionpb.GranteeClient,
) *granteeClient {
	ret := new(granteeClient)
	ret.client = client
	ret.mBroker = broker
	return ret
}

func (c *granteeClient) permissions(ctx context.Context, cfg config.Config) (
	perms []*extensionpb.Permission, err error,
) {
	var req extensionpb.PermRequest
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
	denied []*extensionpb.Permission,
	granted map[string]*extensionpb.PermissionGrant,
) error {
	req := new(extensionpb.OnPermGrantRequest)

	req.Denied = append(req.Denied, denied...)

	for _, gr := range granted {
		req.Granted = append(req.Granted, gr)
	}

	_, err := c.client.OnGrant(ctx, req)
	return err
}

func (c *granteeClient) health(ctx context.Context) error {
	req := extensionpb.HealthRequest{}
	_, err := c.client.Health(ctx, &req)
	return err
}

func (c *granteeClient) bindExtensionClient(pc *goplugin.Client) {
	if c.pClient != nil {
		panic("trying to bind to two clients")
	}
	c.pClient = pc
}

func (c *granteeClient) shutdown(reason string) error {
	if c.pClient != nil {
		// shutdown is handled by grantee server by closing all resources
		// gracefully and waiting for all to complete before exiting.
		// This is to ensure that the process exits, whether gracefully or not
		defer func() {
			c.pClient.Kill()
			c.pClient = nil
		}()
	}
	ctx, cancelFn := context.WithTimeout(context.Background(),
		defDurationGracefulShutClient)
	defer cancelFn()

	req := extensionpb.ShutdownRequest{Reason: reason}
	_, err := c.client.Shutdown(ctx, &req)
	return err
}
