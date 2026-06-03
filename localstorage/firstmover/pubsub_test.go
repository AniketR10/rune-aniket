// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2024 Unstable Build, All Rights Reserved.

package firstmover

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/docmarshal/doctoml"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"github.com/unstablebuild/rune-go-sdk/retry"
)

func TestPubSub(t *testing.T) {
	t.Run("Close called more than once doesn't block or panic", func(t *testing.T) {
		svc, _ := makeLeaderFollowerPair(t, 0)
		assert.NotPanics(t, func() {
			require.NoError(t, svc.Close())
			require.NoError(t, svc.Close())
			require.NoError(t, svc.Close())
		})
	})

	t.Run("leader subscribe more than once returns an error", func(t *testing.T) {
		leader, followers := makeLeaderFollowerPair(t, 1)
		require.NoError(t, leader.Subscribe(context.Background(), "coffee"))
		require.Error(t, leader.Subscribe(context.Background(), "coffee"))
		cleanupNodes(t, append([]*Service{leader}, followers...)...)
	})

	t.Run("follower subscribe more than once returns an error", func(t *testing.T) {
		leader, followers := makeLeaderFollowerPair(t, 1)
		require.NoError(t, followers[0].Subscribe(context.Background(), "coffee"))
		require.Error(t, followers[0].Subscribe(context.Background(), "coffee"))
		cleanupNodes(t, append([]*Service{leader}, followers...)...)
	})

	t.Run("nodes receive ErrMessageTooLarge when publishing oversized messages", func(t *testing.T) {
		leader, followers := makeLeaderFollowerPair(t, 1)
		ctx := context.Background()
		err := leader.Publish(ctx, "1234", make([]byte, leader.cfg.MaxMessageSize*2))
		require.Equal(t, ErrMessageTooLarge, err)
		err = followers[0].Publish(ctx, "1234", make([]byte, leader.cfg.MaxMessageSize*2))
		require.Equal(t, ErrMessageTooLarge, err)
		cleanupNodes(t, append([]*Service{leader}, followers...)...)
	})

	t.Run("leader publishes and follower receives", func(t *testing.T) {
		leader, followers := makeLeaderFollowerPair(t, 1)
		publishAndReceive(t, leader, followers[0], 1)
		cleanupNodes(t, append([]*Service{leader}, followers...)...)
	})

	t.Run("follower publishes and leader receives", func(t *testing.T) {
		leader, followers := makeLeaderFollowerPair(t, 1)
		publishAndReceive(t, followers[0], leader, 1)
		cleanupNodes(t, append([]*Service{leader}, followers...)...)
	})

	t.Run("client re-connect", func(t *testing.T) {
		leader, followers := makeLeaderFollowerPair(t, 2)
		topic := "1234"
		ctx := context.Background()
		require.NoError(t, followers[0].Subscribe(ctx, topic))
		require.NoError(t, followers[1].Subscribe(ctx, topic))
		require.NoError(t, leader.Close())
		require.NoError(t, followers[1].Publish(context.Background(), topic, []byte("block")))
		data, err := followers[0].Receive(context.Background(), topic)
		require.NoError(t, err)
		assert.Equal(t, "block", string(data))
		cleanupNodes(t, followers...)
	})

	t.Run("receive continues after leader handoff", func(t *testing.T) {
		leader, followers := makeLeaderFollowerPair(t, 1)
		follower := followers[0]
		topic := "1234"
		ctx := context.Background()
		require.NoError(t, follower.Subscribe(ctx, topic))
		require.NoError(t, leader.Close())
		require.NoError(t, follower.Publish(ctx, topic, []byte("block")))
		data, err := follower.Receive(ctx, topic)
		require.NoError(t, err)
		assert.Equal(t, "block", string(data))
		cleanupNodes(t, follower)
	})

	t.Run("extreme concurrency of leaders and followers", func(t *testing.T) {
		const n, m = 40, 20
		cfg := testConfig()
		lockFile := makeTempLockFile(t)
		backend := storagestub.NewInMemoryService()
		instances := make([]*Service, 0, n)
		for i := 0; i < n-1; i++ {
			instance := New(factoryFor(backend), lockFile, cfg)
			instance.retryStrategy = retry.CombinedStrategy(
				retry.SequentialStrategy(2*time.Millisecond),
				retry.LimitStrategy(m*2),
			)
			_ = instance.Get(context.Background(), lockFile, nil)
			instances = append(instances, instance)
		}
		instance1 := instances[len(instances)-1]
		instance2 := instances[len(instances)-2]
		defer cleanupNodes(t, instances...)

		go func() {
			for i := 0; i < n-2; i++ {
				time.Sleep(cfg.DialTimeout + cfg.ConnectRetryCadence)
				for _, instance := range instances {
					if instance.IsLeader() && instance != instance1 && instance != instance2 {
						_ = instance.Close()
						break
					}
				}
			}
		}()
		publishAndReceive(t, instance1, instance2, m)
	})
}

func cleanupNodes(t *testing.T, nodes ...*Service) {
	t.Helper()
	for _, node := range nodes {
		if node != nil {
			assert.NoError(t, node.Close())
		}
	}
}

func makeLeaderFollowerPair(t *testing.T, nfollowers int) (*Service, []*Service) {
	return makeLeaderFollowerPairLockFileListen(t, nfollowers, "")
}

func makeLeaderFollowerPairLockFileListen(
	t *testing.T, nfollowers int, lockFileListen string,
) (*Service, []*Service) {
	lockFile := makeTempLockFile(t)
	cfg := testConfig()
	cfg.Marshaler = doctoml.Marshaler()
	svc := storagestub.NewInMemoryServiceWithMarshaler(cfg.Marshaler)
	leader := New(factoryFor(svc), lockFile, cfg)
	var temp testStruct
	err := leader.Get(context.Background(), lockFile, &temp)
	require.Equal(t, storageapi.ErrNotFound, err)
	var followers []*Service
	for i := 0; i < nfollowers; i++ {
		follower := new(Service)
		follower.lockFileListen = lockFileListen
		follower.Init(factoryFor(svc), lockFile, cfg)
		followers = append(followers, follower)
	}
	return leader, followers
}

func publishAndReceive(t *testing.T, sender, receiver *Service, n int) {
	ctx := context.Background()
	var done, ready sync.WaitGroup
	actualMsg := make([][]byte, n)
	actualErr := make([]error, n)
	done.Add(n)
	ready.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer done.Done()
			actualErr[i] = receiver.Subscribe(ctx, strconv.Itoa(i))
			ready.Done()
			if actualErr[i] != nil {
				return
			}
			actualMsg[i], actualErr[i] = receiver.Receive(ctx, strconv.Itoa(i))
		}(i)
	}
	ready.Wait()
	for i := 0; i < n; i++ {
		err := sender.Publish(ctx, strconv.Itoa(i), []byte(strconv.Itoa(i)))
		require.NoError(t, err)
	}
	done.Wait()
	for i := 0; i < n; i++ {
		require.NoError(t, actualErr[i], i)
		assert.Equal(t, strconv.Itoa(i), string(actualMsg[i]), i)
	}
}

func TestPubSubPublishMatrix(t *testing.T) {
	suite := []struct {
		indexPub int
		n        int
	}{
		{0, 2}, {1, 2}, {0, 3}, {1, 3}, {2, 3},
	}
	for _, tc := range suite {
		desc := fmt.Sprintf("%d node is able to publish to the rest of nodes (n=%d)", tc.indexPub, tc.n)
		t.Run(desc, func(t *testing.T) {
			leader, followers := makeLeaderFollowerPair(t, tc.n)
			nodes := append(followers, leader)
			ctx := context.Background()
			topic := "1234"
			publisher := nodes[tc.indexPub]
			rest := make([]*Service, 0, len(nodes)-1)
			rest = append(rest, nodes[:tc.indexPub]...)
			rest = append(rest, nodes[tc.indexPub+1:]...)
			for _, node := range rest {
				require.NoError(t, node.Subscribe(ctx, topic))
			}
			require.NoError(t, publisher.Publish(ctx, topic, []byte("block")))
			for _, node := range rest {
				data, err := node.Receive(ctx, topic)
				require.NoError(t, err)
				assert.Equal(t, "block", string(data))
			}
			cleanupNodes(t, nodes...)
		})
	}
}
