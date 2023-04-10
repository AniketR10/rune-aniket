package rpc

import (
	"context"
	"fmt"
	"os"
	"sync"

	multierr "github.com/ernestrc/go-multierror"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/workspace"
)

/* plugin side */

// if we ever implement a keep alive mechanism for servers it should not
// be added to this server, as clients are completely ephemeral and
// the "proxyId" is stored to connect to it, rather than keeping a connection open
type proxySchemeServerImpl struct {
	UnimplementedProxySchemeServer
	scheme string
	fn     workspace.SchemeFunc
	srv    proto.MuxServer
	broker proto.MuxBroker

	ctx       context.Context
	cancelCtx func()
}

type proxySchemeResource struct {
	server *Server
	scheme workspace.Scheme
}

func (c proxySchemeResource) Close() (ret error) {
	if err := c.server.Stop(); err != nil {
		ret = multierr.Append(ret, err)
	}
	if err := c.scheme.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	return ret
}

func newProxySchemeServerImpl(
	broker proto.MuxBroker, srv proto.MuxServer,
	scheme string, fn workspace.SchemeFunc,
) *proxySchemeServerImpl {
	ret := new(proxySchemeServerImpl)
	ret.broker = broker
	ret.srv = srv
	ret.scheme = scheme
	ret.fn = fn
	ret.ctx, ret.cancelCtx = context.WithCancel(context.Background())
	return ret
}

func (s *proxySchemeServerImpl) serveScheme(scheme workspace.Scheme) (string, error) {
	ret, err := proto.AcceptAndServeChannel(s.ctx, s.broker,
		func(_ string, srv proto.MuxServer) {
			// this is client-side, so no need to pass a locker
			// since it will only be accesed through this server
			server := NewServer(scheme, new(sync.Mutex))
			RegisterSchemeServer(srv.Registrar(), server)
			RegisterFilesServer(srv.Registrar(), server)
		}, "scheme")
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
	cfg config.Config, uri workspaceapi.URI,
	broker proto.MuxBroker, proxyID string,
) (workspace.Scheme, error) {
	// once uri, and config is sent disconnect proxy client
	proxyConn, err := broker.DialChannel(proxyID, os.Args[0], "proxyScheme")
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

	conn, err := broker.DialChannel(resp.GetTokenId())
	if err != nil {
		return nil, err
	}
	return NewClient(conn), nil
}
