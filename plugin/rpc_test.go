package plugin

import (
	"context"
	"errors"
	"math"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"unstable.build/go-tui/config"
	pluginpb "unstable.build/go-tui/plugin/rpc"
	"unstable.build/go-tui/proto"
)

type testGranteePbClient struct {
	locker             sync.Locker
	err                error
	fixturePermissions []*pluginpb.Permission
	sleepPermissions   time.Duration
	healthChan         chan struct{}
	onShutdownChan     chan struct{}

	_permissions *pluginpb.PermRequest
	_onGrant     *pluginpb.OnPermGrantRequest
	_shutdown    *pluginpb.ShutdownRequest
	_health      *pluginpb.HealthRequest
}

func (c *testGranteePbClient) permissions() (pluginpb.PermRequest, bool) {
	c.locker.Lock()
	defer c.locker.Unlock()
	if c._permissions == nil {
		return pluginpb.PermRequest{}, false
	}
	return *c._permissions, true
}
func (c *testGranteePbClient) onGrant() (pluginpb.OnPermGrantRequest, bool) {
	c.locker.Lock()
	defer c.locker.Unlock()
	if c._onGrant == nil {
		return pluginpb.OnPermGrantRequest{}, false
	}
	return *c._onGrant, true
}
func (c *testGranteePbClient) shutdown() (pluginpb.ShutdownRequest, bool) {
	c.locker.Lock()
	defer c.locker.Unlock()
	if c._shutdown == nil {
		return pluginpb.ShutdownRequest{}, false

	}
	return *c._shutdown, true
}
func (c *testGranteePbClient) health() (pluginpb.HealthRequest, bool) {
	c.locker.Lock()
	defer c.locker.Unlock()
	if c._health == nil {
		return pluginpb.HealthRequest{}, false

	}
	return *c._health, true
}

func (c *testGranteePbClient) Permissions(
	ctx context.Context, in *pluginpb.PermRequest, opts ...grpc.CallOption,
) (*pluginpb.PermResponse, error) {
	c.locker.Lock()
	c._permissions = in
	c.locker.Unlock()
	permissions := new(pluginpb.PermResponse)
	if c.err != nil {
		return nil, c.err
	}
	permissions.Perms = c.fixturePermissions
	time.Sleep(c.sleepPermissions)
	return permissions, nil
}

func (c *testGranteePbClient) OnGrant(
	ctx context.Context, in *pluginpb.OnPermGrantRequest, opts ...grpc.CallOption,
) (*pluginpb.OnPermGrantResponse, error) {
	c.locker.Lock()
	c._onGrant = in
	c.locker.Unlock()
	if c.err != nil {
		return nil, c.err
	}
	return new(pluginpb.OnPermGrantResponse), nil
}

func (c *testGranteePbClient) Shutdown(
	ctx context.Context, in *pluginpb.ShutdownRequest, opts ...grpc.CallOption,
) (*pluginpb.ShutdownResponse, error) {
	if c.onShutdownChan != nil {
		go func(ch chan struct{}) {
			c.locker.Lock()
			c._shutdown = in
			c.locker.Unlock()
			ch <- struct{}{}
		}(c.onShutdownChan)
	}
	if c.err != nil {
		return nil, c.err
	}
	return new(pluginpb.ShutdownResponse), nil
}

func (c *testGranteePbClient) Health(
	ctx context.Context, in *pluginpb.HealthRequest, opts ...grpc.CallOption,
) (*pluginpb.HealthResponse, error) {
	if c.err != nil {
		return nil, c.err
	}
	if c.healthChan != nil {
		// wait for test harness signal to respond
		_, ok := <-c.healthChan
		// if not closed, then hold
		if ok {
			c._health = in
		}
	}
	return new(pluginpb.HealthResponse), nil
}

func TestUnitClient(t *testing.T) {
	t.Run("permissions sends permissions", func(t *testing.T) {
		mockpbClient := &testGranteePbClient{locker: new(sync.Mutex)}
		mockpbClient.fixturePermissions =
			[]*pluginpb.Permission{&pluginpb.Permission{Id: "ballz"}}

		client := newGranteeClient(nil, mockpbClient)

		perms, err := client.permissions(context.Background(), nil)
		require.NoError(t, err)
		assert.Equal(t, mockpbClient.fixturePermissions, perms)
	})

	t.Run("permissions bubbles up error", func(t *testing.T) {
		myErr := errors.New("Hubble was perfect")
		mockpbClient := &testGranteePbClient{err: myErr, locker: new(sync.Mutex)}
		client := newGranteeClient(nil, mockpbClient)

		_, err := client.permissions(context.Background(), nil)
		require.Equal(t, myErr, err)
	})

	t.Run("sendGrants sends grants", func(t *testing.T) {
		mockpbClient := &testGranteePbClient{locker: new(sync.Mutex)}
		client := newGranteeClient(nil, mockpbClient)

		grant := &pluginpb.PermissionGrant{Id: "shits", Address: "1234"}
		granted := map[string]*pluginpb.PermissionGrant{"poopers": grant}
		denied := []*pluginpb.Permission{{Id: "poops"}}

		err := client.sendGrants(context.Background(), denied, granted)
		require.NoError(t, err)

		onGrant, ok := mockpbClient.onGrant()
		require.True(t, ok)
		assert.Equal(t, onGrant.GetDenied(), denied)
		assert.Equal(t, []*pluginpb.PermissionGrant{grant}, onGrant.GetGranted())
	})

	t.Run("sendGrants bubbles up error", func(t *testing.T) {
		myErr := errors.New("Hubble was perfect")
		mockpbClient := &testGranteePbClient{err: myErr, locker: new(sync.Mutex)}
		client := newGranteeClient(nil, mockpbClient)

		err := client.sendGrants(context.Background(), nil, nil)
		require.Equal(t, myErr, err)
	})

	t.Run("health send a health request", func(t *testing.T) {
		mockpbClient := &testGranteePbClient{locker: new(sync.Mutex)}
		client := newGranteeClient(nil, mockpbClient)

		err := client.health(context.Background())
		require.NoError(t, err)
		require.NotNil(t, mockpbClient.health)
	})

	t.Run("health bubbles up error", func(t *testing.T) {
		myErr := errors.New("Atza is perfect")
		mockpbClient := &testGranteePbClient{err: myErr, locker: new(sync.Mutex)}
		client := newGranteeClient(nil, mockpbClient)

		err := client.health(context.Background())
		require.Equal(t, myErr, err)
	})

	t.Run("Close send a shutdown request", func(t *testing.T) {
		mockpbClient := &testGranteePbClient{locker: new(sync.Mutex)}
		client := newGranteeClient(nil, mockpbClient)

		err := client.shutdown("")
		require.NoError(t, err)
		require.NotNil(t, mockpbClient.shutdown)
	})

	t.Run("Close bubbles up shutdown error", func(t *testing.T) {
		myErr := errors.New("Atza is perfect")
		mockpbClient := &testGranteePbClient{err: myErr, locker: new(sync.Mutex)}
		client := newGranteeClient(nil, mockpbClient)

		err := client.shutdown("")
		require.Equal(t, myErr, err)
	})
}

type granteeMock struct {
	err                  error
	onConnected          int
	cfgs                 []config.Config
	onGrant, onDenied    []Permission
	onHealth, onShutdown bool
}

func (g *granteeMock) Connected(b proto.MuxBroker, cfg config.Config) {
	g.cfgs = append(g.cfgs, cfg)
	g.onConnected++
}
func (g *granteeMock) PermissionGranted(grants []Grant) {
	for _, grant := range grants {
		g.onGrant = append(g.onGrant, grant.Permission)
	}
}
func (g *granteeMock) PermissionDenied(perms []Permission) {
	g.onDenied = append(g.onDenied, perms...)
}
func (g *granteeMock) Shutdown(reason string) error {
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
	server := newGranteeServer(grpcServer, proto.NewUnixGRPCBroker(""),
		granteeMock, perms, time.Duration(0))
	pluginpb.RegisterGranteeServer(grpcServer, server)

	go grpcServer.Serve(lis)

	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
	require.NoError(t, err)

	client = newGranteeClient(nil, pluginpb.NewGranteeClient(conn))
	closeFn = func() {
		client.shutdown("test harness")
		grpcServer.Stop()
	}
	return
}

func TestIntegrationPluginClientServer(t *testing.T) {
	t.Run("permissions request triggers Grantee OnConnected", func(t *testing.T) {
		grantee := granteeMock{}
		perms := []Permission{Permission("write"), Permission("read")}
		client, closeFn := setupIntTest(t, &grantee, perms)
		defer closeFn()

		protoPerms, err := client.permissions(context.Background(), nil)
		require.NoError(t, err)

		assert.Equal(t, []*pluginpb.Permission{
			&pluginpb.Permission{Id: "write"}, &pluginpb.Permission{Id: "read"},
		}, protoPerms)
		assert.Equal(t, 1, grantee.onConnected)
	})

	t.Run("permissions request plugin config passes onto grantee", func(t *testing.T) {
		grantee := granteeMock{}
		perms := []Permission{Permission("wasup")}
		client, closeFn := setupIntTest(t, &grantee, perms)
		defer closeFn()

		in := map[string]interface{}{
			"viz":    true,
			"hubble": 1,
			"sonicd": "more",
			"longboard": map[string]interface{}{
				"raven": math.MaxFloat64,
			},
		}
		_, err := client.permissions(context.Background(), config.MapConfig(in))
		require.NoError(t, err)

		require.Len(t, grantee.cfgs, 1)

		out := grantee.cfgs[0]
		viz, err := out.GetBool("viz")
		assert.NoError(t, err)
		assert.True(t, viz)

		sonicd, err := out.GetString("sonicd")
		assert.NoError(t, err)
		assert.Equal(t, "more", sonicd)

		longboard, err := out.GetConfig("longboard")
		require.NoError(t, err)

		raven, err := longboard.GetFloat("raven")
		assert.NoError(t, err)
		assert.Equal(t, math.MaxFloat64, raven)

		hubble, err := out.GetInt("hubble")
		assert.NoError(t, err)
		assert.Equal(t, 1, hubble)
	})

	t.Run("permissions request twice does not trigger Grantee OnConnected twice", func(t *testing.T) {
		grantee := granteeMock{}
		perms := []Permission{Permission("append")}
		client, closeFn := setupIntTest(t, &grantee, perms)
		defer closeFn()

		_, err := client.permissions(context.Background(), nil)
		require.NoError(t, err)
		_, err = client.permissions(context.Background(), nil)
		require.NoError(t, err)
		assert.Equal(t, 1, grantee.onConnected)
	})

	t.Run("sendGrants request triggers Grantee OnPermissionGranted/Denied", func(t *testing.T) {
		grantee := granteeMock{}
		perms := []Permission{Permission("append"), Permission("read")}
		client, closeFn := setupIntTest(t, &grantee, perms)
		defer closeFn()

		denied := []*pluginpb.Permission{&pluginpb.Permission{Id: "append"}}
		granted := map[string]*pluginpb.PermissionGrant{"read": &pluginpb.PermissionGrant{Id: "read", Address: "1"}}
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

		denied := []*pluginpb.Permission{&pluginpb.Permission{Id: "garbage"}}
		granted := map[string]*pluginpb.PermissionGrant{
			"write": &pluginpb.PermissionGrant{Id: "write", Address: "1"},
			"trash": &pluginpb.PermissionGrant{Id: "trash", Address: "2"},
		}
		err := client.sendGrants(context.Background(), denied, granted)
		require.NoError(t, err)

		assert.Equal(t, []Permission{Permission("write")}, grantee.onGrant)
		assert.Equal(t, []Permission(nil), grantee.onDenied)
	})

	t.Run("sendGrants request DOES NOT pass multiple grants of the same permission", func(t *testing.T) {
		grantee := granteeMock{}
		perms := []Permission{Permission("write"), Permission("write")}
		client, closeFn := setupIntTest(t, &grantee, perms)
		defer closeFn()

		denied := []*pluginpb.Permission{&pluginpb.Permission{Id: "garbage"}}
		granted := map[string]*pluginpb.PermissionGrant{
			"write": &pluginpb.PermissionGrant{Id: "write", Address: "1"},
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

		err := client.shutdown("sut")
		require.NoError(t, err)

		assert.True(t, grantee.onShutdown)
	})

	t.Run("Close path bubbles up error returned by Grantee", func(t *testing.T) {
		myErr := errors.New("oh bollocks")
		grantee := granteeMock{err: myErr}
		perms := []Permission{Permission("append")}
		client, closeFn := setupIntTest(t, &grantee, perms)
		defer closeFn()

		err := client.shutdown("sut")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "oh bollocks")
	})
}
