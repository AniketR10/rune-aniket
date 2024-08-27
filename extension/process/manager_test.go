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
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"unstable.build/go-tui/api/config"
	"unstable.build/go-tui/component/notifications"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/extension/extensionrpc"
	"unstable.build/go-tui/extension/extensiontest"
	"unstable.build/go-tui/rpc"
)

type nopBroker struct {
	mu      sync.Mutex
	nextID  uint32
	_closed bool
}

func (b *nopBroker) Cleanup(uint32) error {
	return nil
}

func (b *nopBroker) NextId() uint32 {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	return b.nextID
}

func (b *nopBroker) Accept(ID uint32) (net.Listener, error) {
	return nil, nil
}

func (b *nopBroker) AcceptAndServe(
	ID uint32, srv func(opts []grpc.ServerOption) rpc.MuxServer,
) {
}

func (b *nopBroker) Dial(ID uint32) (
	conn rpc.MuxConn, err error,
) {
	return nil, nil
}

func (b *nopBroker) closed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b._closed
}

func (b *nopBroker) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b._closed {
		panic("called Close multiple times")
	}
	b._closed = true
	return nil
}

func newTestManager(grantor extension.Grantor, opts ...Option) (*Manager, *testGranteePbClient, rpc.MuxBroker) {
	m := new(Manager)

	opts = append([]Option{
		WithHandshakeTimeout(1000 * time.Millisecond),
		WithHealthTimeout(1000 * time.Millisecond),
		WithHealthRetries(0),
	}, opts...)
	mockpb := &testGranteePbClient{
		healthChan: make(chan struct{}),
		locker:     new(sync.Mutex),
	}

	m.builder = func(extensionID, path string, grantor extension.Grantor) (*granteeClient, error) {
		return newGranteeClient(m.broker, mockpb), nil
	}
	m.Init(grantor, opts...)
	m.broker = rpc.NewUnixGRPCBroker("", "", "")

	return m, mockpb, m.broker
}

func testRunAndWait(t *testing.T, mgr *Manager, pbClient *testGranteePbClient) {
	cfg := config.MapConfig(make(map[string]interface{}))
	err := mgr.Run("myId", "/here/is/my/extension", cfg)
	require.NoError(t, err)

	pbClient.healthChan <- struct{}{}
	close(pbClient.healthChan) // only test once
}

func assertShutdown(t *testing.T, pbClient *testGranteePbClient) string {
	<-pbClient.onShutdownChan
	sht, ok := pbClient.shutdown()
	assert.True(t, ok)
	return sht.Reason
}

func TestManagerRun(t *testing.T) {
	t.Run("should start extension and proceed to handshake", func(t *testing.T) {
		grantor := extensiontest.MockGrantor{}
		mgr, pbClient, _ := newTestManager(&grantor)
		defer mgr.Close()

		pbClient.fixturePermissions = []*extensionrpc.Permission{{Id: "read"}, {Id: "read"}}
		testRunAndWait(t, mgr, pbClient)

		assert.NotNil(t, pbClient.permissions)
		require.NotNil(t, pbClient.onGrant)

		grant, ok := pbClient.onGrant()
		require.True(t, ok)

		assert.Len(t, grant.Denied, 0)
		require.Len(t, grant.Granted, 1)

		require.Len(t, grant.Granted, 1)
		require.Equal(t, "read", grant.Granted[0].Id)
		require.NotZero(t, grant.Granted[0].Address)

		srvs := grantor.Servers()
		require.Len(t, srvs, 1)

		// verify that shutdown was not called
		_, ok = pbClient.shutdown()
		require.False(t, ok)

		stat, ok := mgr.Stat("myId")
		require.True(t, ok)
		require.NotZero(t, stat.ActiveAt)
		require.Len(t, stat.Errors, 0)

		stats := mgr.Stats()
		status, ok := stats["myId"]
		require.True(t, ok)
		require.NotZero(t, status.ActiveAt)
		require.Len(t, status.Errors, 0)
	})

	t.Run("Stop should call a extension's shutdown", func(t *testing.T) {
		mgr, pbClient, _ := newTestManager(&extensiontest.MockGrantor{})
		defer mgr.Close()

		pbClient.onShutdownChan = make(chan struct{})

		testRunAndWait(t, mgr, pbClient)
		require.NoError(t, mgr.Stop("myId"))

		assertShutdown(t, pbClient)

		// should return error because it had been already stopped
		require.Error(t, mgr.Stop("myId"))
	})

	t.Run("double start extension with same ID should error", func(t *testing.T) {
		mgr, _, _ := newTestManager(&extensiontest.MockGrantor{})
		defer mgr.Close()

		err := mgr.Run("red", "myPath", nil)
		require.NoError(t, err)

		err = mgr.Run("red", "myPath", nil)
		require.Error(t, err)
	})

	t.Run("should shutdown extension if errors upon call to get permissions", func(t *testing.T) {
		mgr, pbClient, _ := newTestManager(&extensiontest.MockGrantor{})
		defer mgr.Close()

		pbClient.fixturePermissions =
			[]*extensionrpc.Permission{{Id: "read"}}
		pbClient.err = errors.New("woopsie")
		pbClient.onShutdownChan = make(chan struct{})

		err := mgr.Run("blue", "/here/is/my/extension", nil)
		require.NoError(t, err)

		reason := assertShutdown(t, pbClient)
		assert.Contains(t, reason, "handshake error")
	})

	t.Run("should shutdown extension if fails to respond to handshake in time", func(t *testing.T) {
		mgr, pbClient, _ := newTestManager(&extensiontest.MockGrantor{})
		defer mgr.Close()

		pbClient.fixturePermissions =
			[]*extensionrpc.Permission{{Id: "read"}}
		pbClient.sleepPermissions = 2 * time.Second
		pbClient.onShutdownChan = make(chan struct{})

		err := mgr.Run("blue", "/here/is/my/extension", nil)
		require.NoError(t, err)

		reason := assertShutdown(t, pbClient)
		assert.Contains(t, reason, "handshake error")
	})

	t.Run("should shutdown extension if fails to respond to first health requests in time", func(t *testing.T) {
		mgr, pbClient, _ := newTestManager(&extensiontest.MockGrantor{})
		defer mgr.Close()

		pbClient.fixturePermissions =
			[]*extensionrpc.Permission{{Id: "read"}}
		pbClient.onShutdownChan = make(chan struct{})

		err := mgr.Run("green", "/here/is/my/extension", nil)
		require.NoError(t, err)

		time.Sleep(mgr.config.handshakeTimeout + 50*time.Millisecond)

		reason := assertShutdown(t, pbClient)
		assert.Contains(t, reason, "health check")
	})

	t.Run("should shutdown extension if fails to respond to subsequent health checks", func(t *testing.T) {
		notifications := testNotifications{}
		mgr, pbClient, _ := newTestManager(&extensiontest.MockGrantor{},
			WithHealthTimeout(250*time.Millisecond), WithHealthRetries(3),
			WithNotifications(&notifications))
		defer mgr.Close()

		pbClient.fixturePermissions =
			[]*extensionrpc.Permission{{Id: "read"}}
		pbClient.onShutdownChan = make(chan struct{})

		err := mgr.Run("yellow", "/here/is/my/extension", nil)
		require.NoError(t, err)

		// respond 2 times correctly
		pbClient.healthChan <- struct{}{}
		pbClient.healthChan <- struct{}{}
		defer close(pbClient.healthChan)

		// exhaust retries
		sleepyTime := 250*3*time.Millisecond + (50 * time.Millisecond)
		time.Sleep(sleepyTime)

		reason := assertShutdown(t, pbClient)
		assert.Contains(t, reason, "exhausted health check retries")

		// check errors in status
		stat, ok := mgr.Stat("yellow")
		assert.True(t, ok)
		require.Len(t, stat.Errors, mgr.config.healthRetries+1, mgr.clients)
		assert.Contains(t, stat.Errors[0].Error(), "deadline")

		notifications.mu.Lock()
		defer notifications.mu.Unlock()

		require.NotZero(t, notifications.msg)
		assert.Equal(t, "extension 'yellow' error: health check: context deadline exceeded", notifications.msg[0])
	})

	t.Run("should wait for extension Shutdown before returning from a call to Close", func(t *testing.T) {
		mgr, pbClient, _ := newTestManager(&extensiontest.MockGrantor{})

		pbClient.fixturePermissions = []*extensionrpc.Permission{{Id: "read"}}
		pbClient.onShutdownChan = make(chan struct{})

		extensionIDs := []string{"green", "blue"}
		for _, id := range extensionIDs {
			err := mgr.Run(id, "/here/is/my/extension", nil)
			require.NoError(t, err)
		}

		time.Sleep(mgr.config.handshakeTimeout + 50*time.Millisecond)

		var wg sync.WaitGroup
		wg.Add(len(extensionIDs))
		go func() {
			for range extensionIDs {
				<-pbClient.onShutdownChan
				_, ok := pbClient.shutdown()
				assert.True(t, ok)
				wg.Done()
			}
		}()
		wg.Wait()

		require.NoError(t, mgr.Close())
	})

	t.Run("Close should not hold resource mutex while waiting for extension shutdown", func(t *testing.T) {
		var rmu sync.Mutex
		mgr, pbClient, _ := newTestManager(&extensiontest.MockGrantor{}, WithLocker(&rmu))

		pbClient.fixturePermissions = []*extensionrpc.Permission{{Id: "read"}}
		pbClient.onShutdownChan = make(chan struct{})
		pbClient.locker = &rmu

		extensionIDs := []string{"green", "blue"}
		for _, id := range extensionIDs {
			err := mgr.Run(id, "/here/is/my/extension", nil)
			require.NoError(t, err)
		}

		time.Sleep(mgr.config.handshakeTimeout + 50*time.Millisecond)

		var wg sync.WaitGroup
		wg.Add(len(extensionIDs))
		go func() {
			for range extensionIDs {
				<-pbClient.onShutdownChan
				_, ok := pbClient.shutdown()
				assert.True(t, ok)
				wg.Done()
			}
		}()
		wg.Wait()

		require.NoError(t, mgr.Close())
	})
}

type testNotifications struct {
	mu  sync.Mutex
	msg []string
}

func (n *testNotifications) Notify(level notifications.Level, msg string, args ...interface{}) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.msg = append(n.msg, fmt.Sprintf(msg, args...))
	return nil
}

func (n *testNotifications) NotifyOnce(level notifications.Level, msg string, args ...interface{}) error {
	return n.Notify(level, msg, args...)
}
