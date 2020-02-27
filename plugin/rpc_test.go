package plugin

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/ernestrc/go-tui/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type testGranteePbClient struct {
	err                error
	fixturePermissions []*proto.Permission

	permissions *proto.PermRequest
	onGrant     *proto.OnPermGrantRequest
	shutdown    *proto.ShutdownRequest
	health      *proto.HealthRequest
}

func (c *testGranteePbClient) Permissions(
	ctx context.Context, in *proto.PermRequest, opts ...grpc.CallOption,
) (*proto.PermResponse, error) {
	c.permissions = in
	permissions := new(proto.PermResponse)
	if c.err != nil {
		return nil, c.err
	}
	permissions.Perms = c.fixturePermissions
	return permissions, nil
}

func (c *testGranteePbClient) OnGrant(
	ctx context.Context, in *proto.OnPermGrantRequest, opts ...grpc.CallOption,
) (*proto.OnPermGrantResponse, error) {
	c.onGrant = in
	if c.err != nil {
		return nil, c.err
	}
	return new(proto.OnPermGrantResponse), nil
}

func (c *testGranteePbClient) Shutdown(
	ctx context.Context, in *proto.ShutdownRequest, opts ...grpc.CallOption,
) (*proto.ShutdownResponse, error) {
	c.shutdown = in
	if c.err != nil {
		return nil, c.err
	}
	return new(proto.ShutdownResponse), nil
}

func (c *testGranteePbClient) Health(
	ctx context.Context, in *proto.HealthRequest, opts ...grpc.CallOption,
) (*proto.HealthResponse, error) {
	c.health = in
	if c.err != nil {
		return nil, c.err
	}
	return new(proto.HealthResponse), nil
}

func TestUnitClient(t *testing.T) {
	t.Run("permissions sends permissions", func(t *testing.T) {
		mockpbClient := &testGranteePbClient{}
		mockpbClient.fixturePermissions =
			[]*proto.Permission{&proto.Permission{Id: "ballz"}}

		client := newGranteeClient(nil, mockpbClient)

		perms, err := client.permissions(context.Background())
		require.NoError(t, err)
		assert.Equal(t, mockpbClient.fixturePermissions, perms)
	})

	t.Run("permissions bubbles up error", func(t *testing.T) {
		myErr := errors.New("Hubble was perfect")
		mockpbClient := &testGranteePbClient{err: myErr}
		client := newGranteeClient(nil, mockpbClient)

		_, err := client.permissions(context.Background())
		require.Equal(t, myErr, err)
	})

	t.Run("sendGrants sends grants", func(t *testing.T) {
		mockpbClient := &testGranteePbClient{}
		client := newGranteeClient(nil, mockpbClient)

		granted := []*proto.PermissionGrant{&proto.PermissionGrant{Id: "shits", GrantId: uint32(1234)}}
		denied := []*proto.Permission{&proto.Permission{Id: "poops"}}

		err := client.sendGrants(context.Background(), denied, granted)
		require.NoError(t, err)

		assert.Equal(t, mockpbClient.onGrant.GetDenied(), denied)
		assert.Equal(t, mockpbClient.onGrant.GetGranted(), granted)
	})

	t.Run("sendGrants bubbles up error", func(t *testing.T) {
		myErr := errors.New("Hubble was perfect")
		mockpbClient := &testGranteePbClient{err: myErr}
		client := newGranteeClient(nil, mockpbClient)

		err := client.sendGrants(context.Background(), nil, nil)
		require.Equal(t, myErr, err)
	})

	t.Run("health send a health request", func(t *testing.T) {
		mockpbClient := &testGranteePbClient{}
		client := newGranteeClient(nil, mockpbClient)

		err := client.health(context.Background())
		require.NoError(t, err)
		require.NotNil(t, mockpbClient.health)
	})

	t.Run("health bubbles up error", func(t *testing.T) {
		myErr := errors.New("Atza is perfect")
		mockpbClient := &testGranteePbClient{err: myErr}
		client := newGranteeClient(nil, mockpbClient)

		err := client.health(context.Background())
		require.Equal(t, myErr, err)
	})

	t.Run("Close send a shutdown request", func(t *testing.T) {
		mockpbClient := &testGranteePbClient{}
		client := newGranteeClient(nil, mockpbClient)

		err := client.Close()
		require.NoError(t, err)
		require.NotNil(t, mockpbClient.shutdown)
	})

	t.Run("Close bubbles up shutdown error", func(t *testing.T) {
		myErr := errors.New("Atza is perfect")
		mockpbClient := &testGranteePbClient{err: myErr}
		client := newGranteeClient(nil, mockpbClient)

		err := client.Close()
		require.Equal(t, myErr, err)
	})
}

type granteeMock struct {
	err                  error
	onConnected          int
	onGrant, onDenied    []Permission
	onHealth, onShutdown bool
}

func (g *granteeMock) OnConnected(b proto.MuxBroker) {
	g.onConnected++
}
func (g *granteeMock) OnPermissionGranted(perm Permission) {
	g.onGrant = append(g.onGrant, perm)
}
func (g *granteeMock) OnPermissionDenied(perm Permission) {
	g.onDenied = append(g.onDenied, perm)
}
func (g *granteeMock) OnShutdown() error {
	if g.err != nil {
		return g.err
	}
	if g.onShutdown {
		return errors.New("called shutdown twice")
	}
	g.onShutdown = true
	return nil
}
func (g *granteeMock) Health() error {
	if g.err != nil {
		return g.err
	}
	g.onHealth = true
	return nil
}

func setupIntTest(
	t *testing.T, granteeMock Grantee, perms []Permission,
) (client *granteeClient, closeFn func()) {
	lis, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	server := newGranteeServer(nil, granteeMock, perms)
	proto.RegisterGranteeServer(grpcServer, server)

	go grpcServer.Serve(lis)

	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
	require.NoError(t, err)

	client = newGranteeClient(nil, proto.NewGranteeClient(conn))
	closeFn = func() {
		client.Close()
		grpcServer.Stop()
	}
	return
}

// TODO test that OnShutdown error does bubble up all the way to the client?
// TODO test that Health error does bubble up all the way to the client?
// TODO test after bind close closes plugin.Client
func TestIntegrationPluginClientServer(t *testing.T) {
	t.Run("permissions request triggers Grantee OnConnected", func(t *testing.T) {
		grantee := granteeMock{}
		perms := []Permission{Permission("write"), Permission("read")}
		client, closeFn := setupIntTest(t, &grantee, perms)
		defer closeFn()

		protoPerms, err := client.permissions(context.Background())
		require.NoError(t, err)

		assert.Equal(t, []*proto.Permission{
			&proto.Permission{Id: "write"}, &proto.Permission{Id: "read"},
		}, protoPerms)
		assert.Equal(t, 1, grantee.onConnected)
	})

	t.Run("permissions request twice does not trigger Grantee OnConnected twice", func(t *testing.T) {
		grantee := granteeMock{}
		perms := []Permission{Permission("append")}
		client, closeFn := setupIntTest(t, &grantee, perms)
		defer closeFn()

		_, err := client.permissions(context.Background())
		require.NoError(t, err)
		_, err = client.permissions(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 1, grantee.onConnected)
	})

	t.Run("sendGrants request triggers Grantee OnPermissionGranted/Denied", func(t *testing.T) {
		grantee := granteeMock{}
		perms := []Permission{Permission("append"), Permission("read")}
		client, closeFn := setupIntTest(t, &grantee, perms)
		defer closeFn()

		denied := []*proto.Permission{&proto.Permission{Id: "append"}}
		granted := []*proto.PermissionGrant{&proto.PermissionGrant{Id: "read", GrantId: 1}}
		err := client.sendGrants(context.Background(), denied, granted)
		require.NoError(t, err)

		assert.Equal(t, []Permission{Permission("read")}, grantee.onGrant)
		assert.Equal(t, []Permission{Permission("append")}, grantee.onDenied)
	})

	t.Run("sendGrants request DOES NOT trigger Grantee OnPermissionGranted/Denied for permissions that were not requested", func(t *testing.T) {
		grantee := granteeMock{}
		perms := []Permission{Permission("write")}
		client, closeFn := setupIntTest(t, &grantee, perms)
		defer closeFn()

		denied := []*proto.Permission{&proto.Permission{Id: "garbage"}}
		granted := []*proto.PermissionGrant{
			&proto.PermissionGrant{Id: "write", GrantId: 1},
			&proto.PermissionGrant{Id: "trash", GrantId: 2},
		}
		err := client.sendGrants(context.Background(), denied, granted)
		require.NoError(t, err)

		assert.Equal(t, []Permission{Permission("write")}, grantee.onGrant)
		assert.Equal(t, []Permission(nil), grantee.onDenied)
	})

	t.Run("health request triggers Grantee Health", func(t *testing.T) {
		grantee := granteeMock{}
		perms := []Permission{Permission("append")}
		client, closeFn := setupIntTest(t, &grantee, perms)
		defer closeFn()

		err := client.health(context.Background())
		require.NoError(t, err)

		assert.True(t, grantee.onHealth)
	})

	t.Run("health path bubbles up error returned by Grantee", func(t *testing.T) {
		myErr := errors.New("oopsie daisy")
		grantee := granteeMock{err: myErr}
		perms := []Permission{Permission("append")}
		client, closeFn := setupIntTest(t, &grantee, perms)
		defer closeFn()

		err := client.health(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "oopsie daisy")
	})

	t.Run("Close request triggers Grantee OnShutdown", func(t *testing.T) {
		grantee := granteeMock{}
		perms := []Permission{Permission("append")}
		client, closeFn := setupIntTest(t, &grantee, perms)
		defer closeFn()

		err := client.Close()
		require.NoError(t, err)

		assert.True(t, grantee.onShutdown)
	})

	t.Run("Close path bubbles up error returned by Grantee", func(t *testing.T) {
		myErr := errors.New("oh bollocks")
		grantee := granteeMock{err: myErr}
		perms := []Permission{Permission("append")}
		client, closeFn := setupIntTest(t, &grantee, perms)
		defer closeFn()

		err := client.Close()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "oh bollocks")
	})
}
