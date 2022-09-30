package rpc

import (
	"errors"
	"fmt"
	"io"

	"github.com/ernestrc/blue/logging"
	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/workspace"
)

// NewSchemeManager returns a workspace.SchemeManager RPC-based client over
// the given connection.
func NewSchemeManager(broker proto.MuxBroker, cc proto.MuxConn) workspace.SchemeManager {
	ret := new(schemeManagerClient)
	ret.init(broker, cc)
	return ret
}

type schemeManagerClient struct {
	broker proto.MuxBroker
	client ManagerClient

	schemes map[string]workspace.SchemeFunc
	servers map[uint32]io.Closer
}

func (c *schemeManagerClient) init(broker proto.MuxBroker, cc proto.MuxConn) {
	c.broker = broker
	c.client = NewManagerClient(cc)
	c.schemes = make(map[string]workspace.SchemeFunc)
	c.servers = make(map[uint32]io.Closer)
}

func (c *schemeManagerClient) log(level log.Level, msg string, args ...interface{}) {
	debug.StandardLogger().
		WithField(logging.KeyClass, "schemeManagerClient").Logf(level, msg, args...)
}

func (c *schemeManagerClient) serveProxyServer(scheme string, fn workspace.SchemeFunc) (
	*proxySchemeServerImpl, uint32, error,
) {
	var psrv *proxySchemeServerImpl
	ret, _, err := proto.AcceptAndServe(c.broker,
		func(proxyID uint32, srv proto.MuxServer) {
			psrv = newProxySchemeServerImpl(c.broker, srv, scheme, fn)
			RegisterProxySchemeServer(srv.GRPC(), psrv)
		})
	if err != nil {
		return nil, 0, err
	}
	return psrv, ret, nil
}

func (c *schemeManagerClient) RegisterScheme(scheme string, fn workspace.SchemeFunc) (err error) {
	var proxyID uint32

	c.log(log.TraceLevel, "RegisterScheme(%s)", scheme)
	defer c.log(log.TraceLevel, "RegisterScheme(%d, %s): %s", proxyID, scheme, err)

	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	_, ok := c.schemes[scheme]
	if ok {
		err = errors.New("scheme already registered")
		return
	}

	var server *proxySchemeServerImpl
	server, proxyID, err = c.serveProxyServer(scheme, fn)
	if err != nil {
		err = fmt.Errorf("serveHandler: %w", err)
		return
	}

	req := RegisterSchemeRequest{ProxyId: proxyID, Scheme: scheme}
	_, err = c.client.RegisterScheme(ctx, &req)
	if err != nil {
		_ = server.Close()
		return
	}

	c.servers[proxyID] = server
	c.schemes[scheme] = fn
	return
}

func (c *schemeManagerClient) Close() (ret error) {
	for _, s := range c.servers {
		if err := s.Close(); err != nil {
			ret = multierr.Append(ret, err)
		}
	}
	c.servers = nil
	return
}
