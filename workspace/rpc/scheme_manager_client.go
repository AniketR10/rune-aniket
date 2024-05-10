package rpc

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"sync"

	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"

	schemeapi "unstable.build/go-tui/api/scheme"
	"unstable.build/go-tui/proto"
)

var _ schemeapi.SchemeManager = (*SchemeManagerClient)(nil)

// NewSchemeManager returns a schemeapi.SchemeManager RPC-based client over
// the given connection.
func NewSchemeManager(
	ctx context.Context, broker proto.MuxBroker, cc proto.MuxConn,
) *SchemeManagerClient {
	ret := new(SchemeManagerClient)
	ret.init(ctx, broker, cc)
	runtime.SetFinalizer(ret, func(c *SchemeManagerClient) { c.Close() })
	return ret
}

// SchemeManagerClient satisfies schemeapi.SchemeManager by calling a
// remote SchemeManager over a proto.MuxConn.
type SchemeManagerClient struct {
	broker    proto.MuxBroker
	client    ManagerClient
	cc        proto.MuxConn
	ctx       context.Context
	cancelCtx func()
}

func (c *SchemeManagerClient) init(
	ctx context.Context, broker proto.MuxBroker, cc proto.MuxConn,
) {
	c.broker = broker
	c.client = NewManagerClient(cc)
	ok := proto.IsContextWithWaitGroup(ctx)
	if !ok {
		ctx = proto.ContextWithWaitGroup(ctx, new(sync.WaitGroup))
	}
	c.ctx, c.cancelCtx = context.WithCancel(ctx)
	c.cc = cc
}

func (c *SchemeManagerClient) log(level log.Level, msg string, args ...interface{}) {
	log.WithField(logging.KeyClass, "SchemeManagerClient").Logf(level, msg, args...)
}

func (c *SchemeManagerClient) serveProxyServer(scheme string, fn schemeapi.SchemeFunc) (
	*proxySchemeServerImpl, string, error,
) {
	var srv proto.MuxServer
	var psrv *proxySchemeServerImpl
	ctxWg := proto.WaitGroupFromContext(c.ctx)
	ret, err := proto.AcceptAndServeChannel(c.ctx, c.broker,
		func(_ string, _srv proto.MuxServer) {
			ctxWg.Add(1)
			srv = _srv
			psrv = newProxySchemeServerImpl(c.ctx, c.broker, srv, scheme, fn)
			RegisterProxySchemeServer(srv.Registrar(), psrv)
		}, "proxy_scheme")
	if err != nil {
		return nil, "", err
	}
	go func(ctx context.Context) {
		defer ctxWg.Done()
		<-ctx.Done()
		srv.Stop()
	}(c.ctx)
	return psrv, ret, nil
}

// RegisterScheme satisfies schemeapi.SchemeManager
func (c *SchemeManagerClient) RegisterScheme(
	scheme string, fn schemeapi.SchemeFunc,
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
		if status.Code(err) == codes.AlreadyExists {
			err = schemeapi.ErrSchemeAlreadyRegistered
		}
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
