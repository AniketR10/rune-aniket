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

package extensionproc

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
	"unstable.build/go-tui/api/config"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/extension/extensionrpc"
	"unstable.build/go-tui/rpc"
)

type testGranteePbClient struct {
	locker             sync.Locker
	err                error
	fixturePermissions []*extensionrpc.Permission
	sleepPermissions   time.Duration
	healthChan         chan struct{}
	onShutdownChan     chan struct{}

	_permissions *extensionrpc.PermRequest
	_onGrant     *extensionrpc.OnPermGrantRequest
	_shutdown    *extensionrpc.ShutdownRequest
	_health      *extensionrpc.HealthRequest
}

func (c *testGranteePbClient) permissions() (extensionrpc.PermRequest, bool) {
	c.locker.Lock()
	defer c.locker.Unlock()
	if c._permissions == nil {
		return extensionrpc.PermRequest{}, false
	}
	return *c._permissions, true
}
func (c *testGranteePbClient) onGrant() (extensionrpc.OnPermGrantRequest, bool) {
	c.locker.Lock()
	defer c.locker.Unlock()
	if c._onGrant == nil {
		return extensionrpc.OnPermGrantRequest{}, false
	}
	return *c._onGrant, true
}
func (c *testGranteePbClient) shutdown() (extensionrpc.ShutdownRequest, bool) {
	c.locker.Lock()
	defer c.locker.Unlock()
	if c._shutdown == nil {
		return extensionrpc.ShutdownRequest{}, false

	}
	return *c._shutdown, true
}
func (c *testGranteePbClient) health() (extensionrpc.HealthRequest, bool) {
	c.locker.Lock()
	defer c.locker.Unlock()
	if c._health == nil {
		return extensionrpc.HealthRequest{}, false

	}
	return *c._health, true
}

func (c *testGranteePbClient) Permissions(
	ctx context.Context, in *extensionrpc.PermRequest, opts ...grpc.CallOption,
) (*extensionrpc.PermResponse, error) {
	c.locker.Lock()
	c._permissions = in
	c.locker.Unlock()
	permissions := new(extensionrpc.PermResponse)
	if c.err != nil {
		return nil, c.err
	}
	permissions.Perms = c.fixturePermissions
	time.Sleep(c.sleepPermissions)
	return permissions, nil
}

func (c *testGranteePbClient) OnGrant(
	ctx context.Context, in *extensionrpc.OnPermGrantRequest, opts ...grpc.CallOption,
) (*extensionrpc.OnPermGrantResponse, error) {
	c.locker.Lock()
	c._onGrant = in
	c.locker.Unlock()
	if c.err != nil {
		return nil, c.err
	}
	return new(extensionrpc.OnPermGrantResponse), nil
}

func (c *testGranteePbClient) Shutdown(
	ctx context.Context, in *extensionrpc.ShutdownRequest, opts ...grpc.CallOption,
) (*extensionrpc.ShutdownResponse, error) {
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
	return new(extensionrpc.ShutdownResponse), nil
}

func (c *testGranteePbClient) Health(
	ctx context.Context, in *extensionrpc.HealthRequest, opts ...grpc.CallOption,
) (*extensionrpc.HealthResponse, error) {
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
	return new(extensionrpc.HealthResponse), nil
}

func TestUnitClient(t *testing.T) {
	t.Run("permissions sends permissions", func(t *testing.T) {
		mockpbClient := &testGranteePbClient{locker: new(sync.Mutex)}
		mockpbClient.fixturePermissions =
			[]*extensionrpc.Permission{{Id: "ballz"}}

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

		grant := &extensionrpc.PermissionGrant{Id: "shits", Address: "1234"}
		granted := map[string]*extensionrpc.PermissionGrant{"poopers": grant}
		denied := []*extensionrpc.Permission{{Id: "poops"}}

		err := client.sendGrants(context.Background(), denied, granted)
		require.NoError(t, err)

		onGrant, ok := mockpbClient.onGrant()
		require.True(t, ok)
		assert.Equal(t, onGrant.GetDenied(), denied)
		assert.Equal(t, []*extensionrpc.PermissionGrant{grant}, onGrant.GetGranted())
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
	onGrant, onDenied    []extension.Permission
	onHealth, onShutdown bool
	onShutdownFn         func()
}

func (g *granteeMock) Connected(ctx context.Context, b rpc.MuxBroker, cfg config.Config) error {
	g.cfgs = append(g.cfgs, cfg)
	g.onConnected++
	return nil
}
func (g *granteeMock) PermissionGranted(ctx context.Context, grants []extension.Grant) error {
	for _, grant := range grants {
		g.onGrant = append(g.onGrant, grant.Permission)
	}
	return nil
}
func (g *granteeMock) PermissionDenied(ctx context.Context, perms []extension.Permission) error {
	g.onDenied = append(g.onDenied, perms...)
	return nil
}
func (g *granteeMock) Shutdown(ctx context.Context, reason string) error {
	if g.err != nil {
		return g.err
	}
	if g.onShutdown {
		return errors.New("called shutdown twice")
	}
	g.onShutdown = true
	if g.onShutdownFn != nil {
		g.onShutdownFn()
	}
	return nil
}
func (g *granteeMock) Health(ctx context.Context) error {
	if g.err != nil {
		return g.err
	}
	g.onHealth = true
	return nil
}

func setupIntTest(
	t *testing.T, granteeMock extension.Grantee, perms []extension.Permission,
) (client *granteeClient, closeFn func()) {
	lis, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	server := newGranteeServer(grpcServer, rpc.NewUnixGRPCBroker("", "", ""),
		granteeMock, perms, time.Duration(0))
	extensionrpc.RegisterGranteeServer(grpcServer, server)

	go grpcServer.Serve(lis)

	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
	require.NoError(t, err)

	client = newGranteeClient(nil, extensionrpc.NewGranteeClient(conn))
	closeFn = func() {
		client.shutdown("test harness")
		grpcServer.Stop()
	}
	return
}

func TestIntegrationExtensionClientServer(t *testing.T) {
	t.Run("permissions request triggers Grantee OnConnected", func(t *testing.T) {
		grantee := granteeMock{}
		perms := []extension.Permission{extension.Permission("write"), extension.Permission("read")}
		client, closeFn := setupIntTest(t, &grantee, perms)
		defer closeFn()

		protoPerms, err := client.permissions(context.Background(), nil)
		require.NoError(t, err)

		assert.Equal(t, []*extensionrpc.Permission{
			{Id: "write"}, {Id: "read"},
		}, protoPerms)
		assert.Equal(t, 1, grantee.onConnected)
	})

	t.Run("permissions request extension config passes onto grantee", func(t *testing.T) {
		grantee := granteeMock{}
		perms := []extension.Permission{extension.Permission("wasup")}
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
		perms := []extension.Permission{extension.Permission("append")}
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
		perms := []extension.Permission{extension.Permission("append"), extension.Permission("read")}
		client, closeFn := setupIntTest(t, &grantee, perms)
		defer closeFn()

		denied := []*extensionrpc.Permission{{Id: "append"}}
		granted := map[string]*extensionrpc.PermissionGrant{"read": {Id: "read", Address: "1"}}
		err := client.sendGrants(context.Background(), denied, granted)
		require.NoError(t, err)

		assert.Equal(t, []extension.Permission{extension.Permission("read")}, grantee.onGrant)
		assert.Equal(t, []extension.Permission{extension.Permission("append")}, grantee.onDenied)
	})

	t.Run("sendGrants request DOES NOT trigger Grantee OnPermissionGranted/Denied for permissions that were not requested", func(t *testing.T) {
		grantee := granteeMock{}
		perms := []extension.Permission{extension.Permission("write")}
		client, closeFn := setupIntTest(t, &grantee, perms)
		defer closeFn()

		denied := []*extensionrpc.Permission{{Id: "garbage"}}
		granted := map[string]*extensionrpc.PermissionGrant{
			"write": {Id: "write", Address: "1"},
			"trash": {Id: "trash", Address: "2"},
		}
		err := client.sendGrants(context.Background(), denied, granted)
		require.NoError(t, err)

		assert.Equal(t, []extension.Permission{extension.Permission("write")}, grantee.onGrant)
		assert.Equal(t, []extension.Permission(nil), grantee.onDenied)
	})

	t.Run("sendGrants request DOES NOT pass multiple grants of the same permission", func(t *testing.T) {
		grantee := granteeMock{}
		perms := []extension.Permission{extension.Permission("write"), extension.Permission("write")}
		client, closeFn := setupIntTest(t, &grantee, perms)
		defer closeFn()

		denied := []*extensionrpc.Permission{{Id: "garbage"}}
		granted := map[string]*extensionrpc.PermissionGrant{
			"write": {Id: "write", Address: "1"},
		}
		err := client.sendGrants(context.Background(), denied, granted)
		require.NoError(t, err)

		assert.Equal(t, []extension.Permission{extension.Permission("write")}, grantee.onGrant)
		assert.Equal(t, []extension.Permission(nil), grantee.onDenied)
	})

	t.Run("health request triggers Grantee Health", func(t *testing.T) {
		grantee := granteeMock{}
		perms := []extension.Permission{extension.Permission("append")}
		client, closeFn := setupIntTest(t, &grantee, perms)
		defer closeFn()

		err := client.health(context.Background())
		require.NoError(t, err)

		assert.True(t, grantee.onHealth)
	})

	t.Run("health path bubbles up error returned by Grantee", func(t *testing.T) {
		myErr := errors.New("oopsie daisy")
		grantee := granteeMock{err: myErr}
		perms := []extension.Permission{extension.Permission("append")}
		client, closeFn := setupIntTest(t, &grantee, perms)
		defer closeFn()

		err := client.health(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "oopsie daisy")
	})

	t.Run("Close request triggers Grantee OnShutdown", func(t *testing.T) {
		var wg sync.WaitGroup
		grantee := granteeMock{onShutdownFn: wg.Done}
		perms := []extension.Permission{extension.Permission("append")}
		client, closeFn := setupIntTest(t, &grantee, perms)
		defer closeFn()

		wg.Add(1)
		err := client.shutdown("sut")
		require.NoError(t, err)

		wg.Wait()
		assert.True(t, grantee.onShutdown)
	})

	t.Run("Close path bubbles up error returned by Grantee", func(t *testing.T) {
		myErr := errors.New("oh bollocks")
		grantee := granteeMock{err: myErr}
		perms := []extension.Permission{extension.Permission("append")}
		client, closeFn := setupIntTest(t, &grantee, perms)
		defer closeFn()

		err := client.shutdown("sut")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "oh bollocks")
	})
}
