package rpc

import (
	"context"
	"fmt"
	"io"
	"sync"

	multierr "github.com/ernestrc/go-multierror"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/workspace"
)

/* server side */

// if we ever implement a keep alive mechanism for servers it should not
// be added to this server, as clients are completely ephemeral and
// the "proxyId" is stored to connect to it, rather than keeping a connection open
type proxySchemeServerImpl struct {
	UnimplementedProxySchemeServer
	scheme string
	fn     workspace.SchemeFunc
	srv    proto.MuxServer
	broker proto.MuxBroker
	mu     sync.Mutex

	servers map[uint32]io.Closer
}

type proxySchemeResource struct {
	srv    proto.MuxServer
	scheme workspace.Scheme
	server *SchemeServerImpl
}

func (c proxySchemeResource) Close() (ret error) {
	c.srv.Stop()
	c.server.Stop()
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
	ret.servers = make(map[uint32]io.Closer)
	return ret
}

func (s *proxySchemeServerImpl) serveScheme(scheme workspace.Scheme) (uint32, error) {
	var server *SchemeServerImpl
	ret, srv, err := proto.AcceptAndServe(s.broker,
		func(_ uint32, srv proto.MuxServer) {
			server = NewSchemeServer(scheme, &s.mu)
			RegisterSchemeServer(srv.Registrar(), server)
		})

	s.mu.Lock()
	defer s.mu.Unlock()

	s.servers[ret] = proxySchemeResource{srv: srv, scheme: scheme, server: server}
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

	scheme, err := s.fn(cfg, uri)
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
	s.mu.Lock()
	defer s.mu.Unlock()

	s.srv.Stop()
	for _, s := range s.servers {
		if err := s.Close(); err != nil {
			ret = multierr.Append(ret, err)
		}
	}
	return nil
}

/* client-side */

func initializeSchemeThroughProxy(
	cfg config.Config, uri workspaceapi.URI,
	broker proto.MuxBroker, proxyID uint32,
) (workspace.Scheme, proto.MuxConn, error) {
	// once uri, and config is sent disconnect proxy client
	proxyConn, err := broker.Dial(proxyID)
	if err != nil {
		return nil, nil, err
	}
	defer proxyConn.Close()

	cfgJson := config.JSONFromConfig(cfg)
	data, err := cfgJson.MarshalText()
	if err != nil {
		return nil, nil, fmt.Errorf("could not marshal proxy config: %s", err)
	}
	proxyClient := NewProxySchemeClient(proxyConn)
	resp, err := proxyClient.InitializeProxy(context.Background(), &InitializeProxyRequest{
		ConfigJson: string(data),
		Uri:        uri.String(),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("could not initialize proxy: %s", err)
	}

	conn, err := broker.Dial(resp.GetTokenId())
	if err != nil {
		return nil, nil, err
	}
	return NewScheme(conn), conn, nil
}
