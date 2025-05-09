// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package workspacerpc

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

	"unstable.build/go-tui/api/schemeapi"
	"unstable.build/go-tui/rpc"
)

var _ schemeapi.SchemeManager = (*SchemeManagerClient)(nil)

// NewSchemeManager returns a schemeapi.SchemeManager RPC-based client over
// the given connection.
func NewSchemeManager(
	ctx context.Context, broker rpc.MuxBroker, cc rpc.MuxConn,
) *SchemeManagerClient {
	ret := new(SchemeManagerClient)
	ret.init(ctx, broker, cc)
	runtime.SetFinalizer(ret, func(c *SchemeManagerClient) { c.Close() })
	return ret
}

// SchemeManagerClient satisfies schemeapi.SchemeManager by calling a
// remote SchemeManager over a rpc.MuxConn.
type SchemeManagerClient struct {
	broker    rpc.MuxBroker
	client    ManagerClient
	cc        rpc.MuxConn
	ctx       context.Context
	cancelCtx func()
}

func (c *SchemeManagerClient) init(
	ctx context.Context, broker rpc.MuxBroker, cc rpc.MuxConn,
) {
	c.broker = broker
	c.client = NewManagerClient(cc)
	ok := rpc.IsContextWithWaitGroup(ctx)
	if !ok {
		ctx = rpc.ContextWithWaitGroup(ctx, new(sync.WaitGroup))
	}
	c.ctx, c.cancelCtx = context.WithCancel(ctx)
	c.cc = cc
}

func (c *SchemeManagerClient) log(level log.Level, msg string, args ...interface{}) {
	if !log.IsLevelEnabled(level) {
		return
	}
	log.WithField(logging.KeyClass, "SchemeManagerClient").Logf(level, msg, args...)
}

func (c *SchemeManagerClient) serveProxyServer(scheme string, fn schemeapi.SchemeFunc) (
	*proxySchemeServerImpl, string, error,
) {
	var srv rpc.MuxServer
	var psrv *proxySchemeServerImpl
	ctxWg := rpc.WaitGroupFromContext(c.ctx)
	ret, err := rpc.AcceptAndServeChannel(c.ctx, c.broker,
		func(_ string, _srv rpc.MuxServer) {
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
