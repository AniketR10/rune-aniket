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

package process

import (
	"context"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"unstable.build/go-tui/extension/extensionrpc"
)

type loggingGranteeServer struct {
	extensionrpc.GranteeServer
}

func (s *loggingGranteeServer) Permissions(
	ctx context.Context, in *extensionrpc.PermRequest,
) (*extensionrpc.PermResponse, error) {
	res, err := s.GranteeServer.Permissions(ctx, in)
	log.Tracef("GranteeServer.Permissions(%+v): (%+v, %v)", in, res, err)
	return res, err
}

func (s *loggingGranteeServer) OnGrant(
	ctx context.Context, in *extensionrpc.OnPermGrantRequest,
) (*extensionrpc.OnPermGrantResponse, error) {
	res, err := s.GranteeServer.OnGrant(ctx, in)
	log.Tracef("GranteeServer.OnGrant(%+v): (%+v, %v)", in, res, err)
	return res, err
}

func (s *loggingGranteeServer) Shutdown(
	ctx context.Context, in *extensionrpc.ShutdownRequest,
) (*extensionrpc.ShutdownResponse, error) {
	res, err := s.GranteeServer.Shutdown(ctx, in)
	log.Tracef("GranteeServer.Shutdown(%+v): (%+v, %v)", in, res, err)
	return res, err
}

func (s *loggingGranteeServer) Health(
	ctx context.Context, in *extensionrpc.HealthRequest,
) (*extensionrpc.HealthResponse, error) {
	res, err := s.GranteeServer.Health(ctx, in)
	log.Tracef("GranteeServer.Health(%+v): (%+v, %v)", in, res, err)
	return res, err
}

type loggingGranteeClient struct {
	extensionrpc.GranteeClient
}

func (c *loggingGranteeClient) Permissions(
	ctx context.Context, in *extensionrpc.PermRequest, opts ...grpc.CallOption,
) (*extensionrpc.PermResponse, error) {
	res, err := c.GranteeClient.Permissions(ctx, in, opts...)
	log.Tracef("GranteeClient.Permissions(%+v): (%+v, %v)", in, res, err)
	return res, err
}

func (c *loggingGranteeClient) OnGrant(
	ctx context.Context, in *extensionrpc.OnPermGrantRequest, opts ...grpc.CallOption,
) (*extensionrpc.OnPermGrantResponse, error) {
	res, err := c.GranteeClient.OnGrant(ctx, in, opts...)
	log.Tracef("GranteeClient.OnGrant(%+v): (%+v, %v)", in, res, err)
	return res, err
}

func (c *loggingGranteeClient) Shutdown(
	ctx context.Context, in *extensionrpc.ShutdownRequest, opts ...grpc.CallOption,
) (*extensionrpc.ShutdownResponse, error) {
	res, err := c.GranteeClient.Shutdown(ctx, in, opts...)
	log.Tracef("GranteeClient.Shutdown(%+v): (%+v, %v)", in, res, err)
	return res, err
}

func (c *loggingGranteeClient) Health(
	ctx context.Context, in *extensionrpc.HealthRequest, opts ...grpc.CallOption,
) (*extensionrpc.HealthResponse, error) {
	res, err := c.GranteeClient.Health(ctx, in, opts...)
	log.Tracef("GranteeClient.Health(%+v): (%+v, %v)", in, res, err)
	return res, err
}
