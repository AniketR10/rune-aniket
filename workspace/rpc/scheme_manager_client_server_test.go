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
	"context"
	"net"
	"sync"
	"testing"
	"time"

	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/retry"
	"google.golang.org/grpc"
	"unstable.build/go-tui/api/config"
	schemeapi "unstable.build/go-tui/api/scheme"
	schemetest "unstable.build/go-tui/api/scheme/test"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/rpc"
	"unstable.build/go-tui/workspace"
	"unstable.build/go-tui/workspace/test"
)

func doSetupSchemeManagerClientServerTest(
	t *testing.T, s *SchemeManagerServer,
) (conn *grpc.ClientConn, closeFn func()) {
	lis, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	RegisterManagerServer(grpcServer, s)

	go grpcServer.Serve(lis)

	conn, err = grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
	require.NoError(t, err)

	closeFn = func() {
		grpcServer.Stop()
		lis.Close()
	}
	return
}
func setupProxyUnitTest(t *testing.T, ctrl *gomock.Controller) (
	schemeapi.Scheme, *schemetest.MockScheme, func(*testing.T),
) {
	mockScheme := schemetest.NewMockScheme(ctrl)
	client, closeFn := setupProxyTest(t, mockScheme)
	return client, mockScheme, closeFn
}

func setupProxyTest(t *testing.T, mockScheme schemeapi.Scheme) (
	schemeapi.Scheme, func(*testing.T),
) {
	uri, err := workspaceapi.ParseURI("test:///tmp")
	require.NoError(t, err)
	cfg := config.NopConfig()
	manager := workspace.NewManager(cfg)
	broker := rpc.NewUnixGRPCBroker("", "", "")

	// host-side
	srv := NewSchemeManagerServer(broker, manager, new(sync.Mutex))
	conn, closeFn := doSetupSchemeManagerClientServerTest(t, srv)

	// extension-side
	managerClient := NewSchemeManager(context.Background(), broker, conn)
	require.NoError(t, managerClient.RegisterScheme("test",
		func(_ context.Context, _cfg config.Config, _uri workspaceapi.URI) (
			schemeapi.Scheme, error,
		) {
			assert.Equal(t, uri, _uri)
			return mockScheme, nil
		}))

	// wait for RegisterScheme on host side
	var schemeFn schemeapi.SchemeFunc
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = retry.Retry(ctx, retry.ExponentialStrategy(10*time.Millisecond, 1*time.Second),
		func(context.Context) (bool, error) {
			var err error
			schemeFn, err = manager.Scheme(uri)
			return true, err
		})
	require.NoError(t, err)

	client, err := schemeFn(context.Background(), cfg, uri)
	require.NoError(t, err)

	return client, func(t *testing.T) {
		closeFn()
		require.NoError(t, managerClient.Close())
		require.NoError(t, manager.Close())
	}
}

func TestSchemeManagerClientServerSchemeSuiteIntegration(t *testing.T) {
	test.TestWorkspaceSchemeFiles(t, func(t *testing.T) schemeapi.Scheme {
		memURI, err := workspaceapi.ParseURI("memory:///tmp")
		require.NoError(t, err)
		scheme, err := workspace.NewMemoryScheme(context.Background(), config.NopConfig(), memURI)
		require.NoError(t, err)
		client, closeFn := setupProxyTest(t, scheme)
		t.Cleanup(func() { closeFn(t) })
		return client
	})

	t.Run("surfaces schemeapi.ErrSchemeAlreadyRegistered", func(t *testing.T) {
		uri, err := workspaceapi.ParseURI("test:///tmp")
		require.NoError(t, err)
		cfg := config.NopConfig()
		manager := workspace.NewManager(cfg)
		broker := rpc.NewUnixGRPCBroker("", "", "")

		srv := NewSchemeManagerServer(broker, manager, new(sync.Mutex))
		conn, closeFn := doSetupSchemeManagerClientServerTest(t, srv)
		defer closeFn()

		managerClient := NewSchemeManager(context.Background(), broker, conn)
		require.NoError(t, managerClient.RegisterScheme("test",
			func(ctx context.Context, cfg config.Config, _uri workspaceapi.URI) (
				schemeapi.Scheme, error,
			) {
				return workspace.NewMemoryScheme(ctx, cfg, uri)
			}))

		require.Equal(t, schemeapi.ErrSchemeAlreadyRegistered, managerClient.RegisterScheme("test",
			func(ctx context.Context, cfg config.Config, _uri workspaceapi.URI) (
				schemeapi.Scheme, error,
			) {
				return workspace.NewMemoryScheme(ctx, cfg, uri)
			}))
	})

	t.Run("unregisters created schemes upon Close", func(t *testing.T) {
		uri, err := workspaceapi.ParseURI("test:///tmp")
		require.NoError(t, err)
		cfg := config.NopConfig()
		manager := workspace.NewManager(cfg)
		broker := rpc.NewUnixGRPCBroker("", "", "")

		srv := NewSchemeManagerServer(broker, manager, new(sync.Mutex))
		conn, closeFn := doSetupSchemeManagerClientServerTest(t, srv)
		defer closeFn()
		managerClient := NewSchemeManager(context.Background(), broker, conn)
		require.NoError(t, managerClient.RegisterScheme("test",
			func(ctx context.Context, cfg config.Config, _uri workspaceapi.URI) (
				schemeapi.Scheme, error,
			) {
				memURI, err := workspaceapi.ParseURI("memory:///tmp")
				if err != nil {
					return nil, err
				}
				return workspace.NewMemoryScheme(ctx, cfg, memURI)
			}))

		// wait for RegisterScheme on host side
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err = retry.Retry(ctx, retry.ExponentialStrategy(10*time.Millisecond, 1*time.Second),
			func(context.Context) (bool, error) {
				var err error
				_, err = manager.Scheme(uri)
				return true, err
			})
		require.NoError(t, err)

		require.NoError(t, srv.Close())
		_, err = manager.Scheme(uri)
		require.Error(t, err)

		require.NoError(t, managerClient.Close())
		require.NoError(t, manager.Close())
	})
}
