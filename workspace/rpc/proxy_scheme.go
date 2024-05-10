package rpc

import (
	"context"
	"fmt"
	"os"
	"sync"

	"unstable.build/go-tui/api/config"
	schemeapi "unstable.build/go-tui/api/scheme"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/proto"
)

/* extension side */

// if we ever implement a keep alive mechanism for servers it should not
// be added to this server, as clients are completely ephemeral and
// the "proxyId" is stored to connect to it, rather than keeping a connection open
type proxySchemeServerImpl struct {
	UnimplementedProxySchemeServer
	scheme string
	fn     schemeapi.SchemeFunc
	srv    proto.MuxServer
	broker proto.MuxBroker

	ctx       context.Context
	cancelCtx func()
}

func newProxySchemeServerImpl(
	ctx context.Context, broker proto.MuxBroker,
	srv proto.MuxServer, scheme string, fn schemeapi.SchemeFunc,
) *proxySchemeServerImpl {
	ret := new(proxySchemeServerImpl)
	ret.broker = broker
	ret.srv = srv
	ret.scheme = scheme
	ret.fn = fn
	ok := proto.IsContextWithWaitGroup(ctx)
	if !ok {
		ctx = proto.ContextWithWaitGroup(ctx, new(sync.WaitGroup))
	}
	ret.ctx, ret.cancelCtx = context.WithCancel(ctx)
	return ret
}

func (s *proxySchemeServerImpl) serveScheme(scheme schemeapi.Scheme) (string, error) {
	var srv proto.MuxServer
	ctxWg := proto.WaitGroupFromContext(s.ctx)
	// NOTE: scheme are usually served once for the lifecycle of the extension.
	// If this ever changes, we should ensure that when scheme is closed,
	// we stop the grpc server AND manage any cyclical references such that
	// the runtime finalizer of the client can run.
	ret, err := proto.AcceptAndServeChannel(s.ctx, s.broker,
		func(_ string, _srv proto.MuxServer) {
			ctxWg.Add(1)
			srv = _srv
			// this is client-side, so no need to pass a locker
			// since it will only be accesed through this server
			server := NewServer(scheme, new(sync.Mutex))
			RegisterSchemeServer(srv.Registrar(), server)
			RegisterFilesServer(srv.Registrar(), server)
		}, "scheme")
	if err == nil {
		go func(ctx context.Context) {
			defer ctxWg.Done()
			<-ctx.Done()
			srv.Stop()
		}(s.ctx)
	}
	return ret, err
}

func (s *proxySchemeServerImpl) InitializeProxy(
	ctx context.Context, req *InitializeProxyRequest,
) (*InitializeProxyResponse, error) {
	var cfg config.Config
	if req.GetConfigJson() != "" {
		cfg = &config.JSON{}
		err := cfg.(*config.JSON).UnmarshalText([]byte(req.GetConfigJson()))
		if err != nil {
			return nil, fmt.Errorf("could not unmarshal config: %s", err)
		}
	}

	uriStr := req.GetUri()
	uri, err := workspaceapi.ParseURI(uriStr)
	if err != nil {
		return nil, err
	}

	scheme, err := s.fn(ctx, cfg, uri)
	if err != nil {
		return nil, err
	}

	tokenID, err := s.serveScheme(scheme)
	if err != nil {
		return nil, err
	}

	return &InitializeProxyResponse{TokenId: tokenID}, nil
}

func (s *proxySchemeServerImpl) Close() (ret error) {
	if s.cancelCtx == nil {
		return
	}

	s.cancelCtx()
	s.cancelCtx = nil
	s.srv.Stop()
	return nil
}

/* host-side */

func initializeSchemeThroughProxy(
	ctx context.Context, cfg config.Config, uri workspaceapi.URI,
	broker proto.MuxBroker, proxyID string,
) (schemeapi.Scheme, error) {
	// once uri, and config is sent disconnect proxy client
	proxyConn, err := broker.DialChannel(ctx, proxyID, os.Args[0], "proxyScheme")
	if err != nil {
		return nil, err
	}
	defer proxyConn.Close()

	cfgJson := config.JSONFromConfig(cfg)
	data, err := cfgJson.MarshalText()
	if err != nil {
		return nil, fmt.Errorf("could not marshal proxy config: %s", err)
	}
	proxyClient := NewProxySchemeClient(proxyConn)
	resp, err := proxyClient.InitializeProxy(context.Background(), &InitializeProxyRequest{
		ConfigJson: string(data),
		Uri:        uri.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("could not initialize proxy: %s", err)
	}

	conn, err := broker.DialChannel(ctx, resp.GetTokenId(), os.Args[0], "proxyScheme")
	if err != nil {
		return nil, err
	}
	return NewClient(conn), nil
}
