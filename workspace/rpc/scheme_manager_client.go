package rpc

import (
	"context"
	"fmt"
	"io"
	"runtime"

	"github.com/ernestrc/blue/logging"
	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"

	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/workspace"
)

var _ workspace.SchemeManager = (*SchemeManagerClient)(nil)

// NewSchemeManager returns a workspace.SchemeManager RPC-based client over
// the given connection.
func NewSchemeManager(broker proto.MuxBroker, cc proto.MuxConn) *SchemeManagerClient {
	ret := new(SchemeManagerClient)
	ret.init(broker, cc)
	runtime.SetFinalizer(ret, func(c *SchemeManagerClient) { c.Close() })
	return ret
}

// SchemeManagerClient satisfies workspace.SchemeManager by calling a
// remote SchemeManager over a proto.MuxConn.
type SchemeManagerClient struct {
	broker    proto.MuxBroker
	client    ManagerClient
	cc        proto.MuxConn
	ctx       context.Context
	cancelCtx func()
}

func (c *SchemeManagerClient) init(broker proto.MuxBroker, cc proto.MuxConn) {
	c.broker = broker
	c.client = NewManagerClient(cc)
	c.ctx, c.cancelCtx = context.WithCancel(context.Background())
	c.cc = cc
}

func (c *SchemeManagerClient) log(level log.Level, msg string, args ...interface{}) {
	log.WithField(logging.KeyClass, "SchemeManagerClient").Logf(level, msg, args...)
}

func (c *SchemeManagerClient) serveProxyServer(scheme string, fn workspace.SchemeFunc) (
	*proxySchemeServerImpl, string, error,
) {
	var psrv *proxySchemeServerImpl
	ret, err := proto.AcceptAndServeChannel(c.ctx, c.broker,
		func(_ string, srv proto.MuxServer) {
			psrv = newProxySchemeServerImpl(c.broker, srv, scheme, fn)
			RegisterProxySchemeServer(srv.Registrar(), psrv)
		}, "proxy_scheme")
	if err != nil {
		return nil, "", err
	}
	return psrv, ret, nil
}

// RegisterScheme satisfies workspace.SchemeManager
func (c *SchemeManagerClient) RegisterScheme(
	scheme string, fn workspace.SchemeFunc,
) (err error) {
	var proxyID string

	c.log(log.TraceLevel, "RegisterScheme(%s)", scheme)
	defer c.log(log.TraceLevel, "RegisterScheme(%s, %s): %s", proxyID, scheme, err)

	ctx, cleanup := ctxWithTimeout(c.ctx)
	defer cleanup()

	var server *proxySchemeServerImpl
	server, proxyID, err = c.serveProxyServer(scheme, fn)
	if err != nil {
		err = fmt.Errorf("serveHandler: %w", err)
		return
	}

	req := RegisterSchemeRequest{ProxyId: proxyID, Scheme: scheme}
	_, err = c.client.RegisterScheme(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		_ = server.Close()
		return
	}

	return
}

// Close releases all resources associated with this instance.
func (c *SchemeManagerClient) Close() (ret error) {
	if c.cancelCtx == nil {
		return
	}
	c.cancelCtx()
	c.cancelCtx = nil
	if closer, ok := c.cc.(io.Closer); ok {
		if err := closer.Close(); err != nil {
			ret = multierr.Append(ret, err)
		}
	}
	runtime.SetFinalizer(c, nil)
	return
}
