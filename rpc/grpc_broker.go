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
package rpc

import (
	context "context"
	fmt "fmt"
	"net"
	"os"
	"sync"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"unstable.build/go-tui/util"
)

// implements MuxBroker
type grpcBroker struct {
	dataDir     string
	pkg         string
	version     string
	unixSockets sync.Map
}

// NewUnixGRPCBroker provides brokerage by using unix socket listeners
// and grpc connections.
func NewUnixGRPCBroker(dataDir, pkg, version string) MuxBroker {
	ret := new(grpcBroker)
	ret.dataDir = dataDir
	ret.pkg = pkg
	ret.version = version
	return ret
}

func (t *grpcBroker) NewChannel(tags ...string) (MuxServer, error) {
	ret, err := util.TempUnixListenerTags(t.dataDir, tags...)
	if err != nil {
		return nil, fmt.Errorf("temp unix listener")
	}
	t.unixSockets.Store(ret.Addr().String(), struct{}{})

	var opts []grpc.ServerOption
	if t.dataDir != "" {
		const shouldPanic = true
		opts = append(opts, grpc.ChainUnaryInterceptor(
			UnaryReportRecoveryInterceptor(t.dataDir, t.pkg, t.version, shouldPanic),
		))
		opts = append(opts, grpc.ChainStreamInterceptor(
			StreamReportRecoveryInterceptor(t.dataDir, t.pkg, t.version, shouldPanic),
		))
	}
	return GRPCServer(ret, opts...), nil
}

func (t *grpcBroker) DialChannel(ctx context.Context, address string, tags ...string) (
	conn MuxConn, err error,
) {
	opts := []grpc.DialOption{
		grpc.WithStatsHandler(nil),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithContextDialer(
			func(ctx context.Context, _ string) (net.Conn, error) {
				addr, err := net.ResolveUnixAddr("unix", address)
				if err != nil {
					return nil, err
				}
				var d net.Dialer
				return d.DialContext(ctx, addr.Network(), addr.String())
			},
		)}
	conn, err = grpc.DialContext(ctx, "", opts...)
	if err != nil {
		return
	}

	if log.IsLevelEnabled(log.TraceLevel) {
		conn = newLoggingConn(address, conn, tags...)
	}
	return
}

func (t *grpcBroker) Close() (ret error) {
	t.unixSockets.Range(func(key, value any) bool {
		// might be redundant if clients clean up correctly
		_ = os.Remove(key.(string))
		return true
	})
	return nil
}
