package rpc

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ernestrc/blue/retry"
	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/workspace"
	workspacetest "unstable.build/go-tui/workspace/test"
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

func TestSchemeManagerClientServerScheme(t *testing.T) {
	testSchemeClientServer(t, func(t *testing.T, ctrl *gomock.Controller) (
		workspace.Scheme, *workspacetest.MockScheme, func(),
	) {
		uri, err := workspace.ParseURI("test:///tmp")
		require.NoError(t, err)
		cfg := config.NopConfig()
		manager := workspace.NewManager(cfg)
		broker := proto.NewDialBroker()
		mockScheme := workspacetest.NewMockScheme(ctrl)

		// host-side
		srv := NewSchemeManagerServer(broker, manager, new(sync.Mutex))
		conn, closeFn := doSetupSchemeManagerClientServerTest(t, srv)

		// plugin-side
		managerClient := NewSchemeManager(broker, conn)
		require.NoError(t, managerClient.RegisterScheme("test", func(_cfg config.Config, _uri workspace.URI) (workspace.Scheme, error) {
			assert.Equal(t, uri, _uri)
			return mockScheme, nil
		}))

		// wait for RegisterScheme on host side
		var schemeFn workspace.SchemeFunc
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err = retry.Retry(ctx, retry.ExponentialStrategy(10*time.Millisecond, 1*time.Second),
			func(context.Context) (bool, error) {
				var err error
				schemeFn, err = manager.Scheme(uri)
				return true, err
			})
		require.NoError(t, err)

		client, err := schemeFn(cfg, uri)
		require.NoError(t, err)

		return client, mockScheme, func() {
			closeFn()
			conn.Close()
			manager.Close()
		}
	})
}
