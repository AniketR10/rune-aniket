// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2024 Unstable Build, All Rights Reserved.

package firstmover

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	multierror "github.com/ernestrc/go-multierror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/document/docmarshal"
	"github.com/unstablebuild/blue/document/docmarshal/docbson"
	"github.com/unstablebuild/blue/document/docmarshal/docjson"
	"github.com/unstablebuild/blue/document/docmarshal/doctoml"
	"github.com/unstablebuild/blue/document/doctest"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"unstable.build/go-tui/localstorage/bluestore"
)

func TestDefaultConfiguration(t *testing.T) {
	cfg := DefaultConfig()
	maxFollowFailures := int(cfg.TimeToCoup / (cfg.DialTimeout + cfg.ConnectRetryCadence))
	assert.Greater(t, maxFollowFailures, 1)
}

func TestServiceIntegration(t *testing.T) {
	for name, _marshaler := range map[string]docmarshal.Marshaler{
		"bson": docbson.Marshaler(),
		"json": docjson.Marshaler(),
		"toml": doctoml.Marshaler(),
	} {
		marshaler := _marshaler
		t.Run(name, func(t *testing.T) {
			t.Run("single instance assumes leader", func(t *testing.T) {
				doctest.TestDocumentService(t, func(t *testing.T) document.Service {
					lockFile := makeTempLockFile(t)
					cfg := testConfig()
					cfg.Marshaler = marshaler
					svc := bluestore.AdaptTo(document.NewInMemoryServiceWithMarshaler(marshaler))
					return bluestore.AdaptFrom(New(svc, lockFile, cfg))
				})
			})

			t.Run("two instances, seconds assumes follower", func(t *testing.T) {
				doctest.TestDocumentService(t, func(t *testing.T) document.Service {
					lockFile := makeTempLockFile(t)
					svc := bluestore.AdaptTo(document.NewInMemoryServiceWithMarshaler(marshaler))
					cfg := testConfig()
					cfg.Marshaler = marshaler
					leader := New(svc, lockFile, cfg)
					var temp testStruct
					err := bluestore.AdaptFrom(leader).Get(context.Background(), lockFile, &temp)
					require.Equal(t, document.ErrNotFound, err)
					follower := New(svc, lockFile, cfg)
					return bluestore.AdaptFrom(follower)
				})
			})
		})
	}

	t.Run("single instance with lock path on non-existent folder attempts to create directory structure", func(t *testing.T) {
		doctest.TestDocumentService(t, func(t *testing.T) document.Service {
			lockFile := makeTempLockFile(t)
			lockFileDir := filepath.Join(filepath.Dir(lockFile), "newDir", "otherDir", "moreDirs")
			lockFile = filepath.Join(lockFileDir, ".lock")
			cfg := testConfig()
			cfg.Marshaler = docbson.Marshaler()
			svc := bluestore.AdaptTo(document.NewInMemoryServiceWithMarshaler(cfg.Marshaler))
			return bluestore.AdaptFrom(New(svc, lockFile, cfg))
		})
	})

	t.Run("single instance preconditions (bson)", func(t *testing.T) {
		doctest.TestDocumentServicePreconditions(t, func(t *testing.T) document.Service {
			lockFile := makeTempLockFile(t)
			cfg := testConfig()
			cfg.Marshaler = docbson.Marshaler()
			svc := bluestore.AdaptTo(document.NewInMemoryServiceWithMarshaler(cfg.Marshaler))
			return bluestore.AdaptFrom(New(svc, lockFile, cfg))
		})
	})

	t.Run("single instance eventually assumes leader if leader is non-responsive (lock leaked)", func(t *testing.T) {
		doctest.TestDocumentService(t, func(t *testing.T) document.Service {
			f, err := os.CreateTemp("", "")
			require.NoError(t, err)
			require.NoError(t, f.Close())
			svc := bluestore.AdaptTo(document.NewInMemoryService())
			return bluestore.AdaptFrom(New(svc, f.Name(), testConfig()))
		})
	})

	t.Run("two instances, seconds assumes leader after leader dies", func(t *testing.T) {
		doctest.TestDocumentService(t, func(t *testing.T) document.Service {
			lockFile := makeTempLockFile(t)
			svc := bluestore.AdaptTo(document.NewInMemoryService())
			leader := New(svc, lockFile, testConfig())

			err := leader.Set(context.Background(), "random", &testStruct{A: "1234"})
			require.NoError(t, err)
			follower := New(svc, lockFile, testConfig())

			var temp testStruct
			err = follower.Get(context.Background(), "random", &temp)
			require.NoError(t, err)
			require.Equal(t, "1234", temp.A)
			_ = leader.Close()

			err = follower.Delete(context.Background(), "random")
			require.NoError(t, err)

			return bluestore.AdaptFrom(follower)
		})
	})

	t.Run("remove leader lock, second instance assumes leader after leader is unresponsive", func(t *testing.T) {
		doctest.TestDocumentService(t, func(t *testing.T) document.Service {
			lockFile := makeTempLockFile(t)
			svc := bluestore.AdaptTo(document.NewInMemoryService())
			leader := New(svc, lockFile, testConfig())
			err := leader.Set(context.Background(), "dragonballz", &testStruct{A: "1234"})
			require.NoError(t, err)
			follower := New(svc, lockFile, testConfig())

			var temp testStruct
			err = follower.Get(context.Background(), "dragonballz", &temp)
			require.NoError(t, err)
			require.Equal(t, "1234", temp.A)
			err = follower.Delete(context.Background(), "dragonballz")
			require.NoError(t, err)

			_ = os.Remove(lockFile)
			return bluestore.AdaptFrom(follower)
		})
	})

	t.Run("remove leader lock, other assumes leader after leader is unresponsive", func(t *testing.T) {
		doctest.TestDocumentService(t, func(t *testing.T) document.Service {
			lockFile := makeTempLockFile(t)
			svc := bluestore.AdaptTo(document.NewInMemoryService())
			leader := New(svc, lockFile, testConfig())
			err := leader.Set(context.Background(), "dragonballz", &testStruct{A: "1234"})
			require.NoError(t, err)
			follower1 := New(svc, lockFile, testConfig())
			follower2 := New(svc, lockFile, testConfig())
			follower3 := New(svc, lockFile, testConfig())
			follower4 := New(svc, lockFile, testConfig())
			follower5 := New(svc, lockFile, testConfig())
			followers := []document.Service{
				bluestore.AdaptFrom(follower1),
				bluestore.AdaptFrom(follower2),
				bluestore.AdaptFrom(follower3),
				bluestore.AdaptFrom(follower4),
				bluestore.AdaptFrom(follower5),
			}

			for _, follower := range followers {
				var temp testStruct
				err = follower.Get(context.Background(), "dragonballz", &temp)
				require.NoError(t, err)
				require.Equal(t, "1234", temp.A)
			}
			err = leader.Delete(context.Background(), "dragonballz")
			require.NoError(t, err)

			_ = os.Remove(lockFile)
			return &alternatingService{svc: followers}
		})
	})

	t.Run("multiple instances", func(t *testing.T) {
		doctest.TestDocumentServiceNoList(t, func(t *testing.T) document.Service {
			const n = 100
			cfg := testConfig()

			lockFile := makeTempLockFile(t)
			svc := bluestore.AdaptTo(document.NewInMemoryService())

			instances := make([]*Service, 0, n)
			for i := 0; i < n-1; i++ {
				instance := New(svc, lockFile, cfg)
				_ = instance.Get(context.Background(), lockFile, nil)
				instances = append(instances, instance)
			}
			ret := instances[len(instances)-1]

			go func() {
				for i := 0; i < n-1; i++ {
					time.Sleep(cfg.DialTimeout + cfg.ConnectRetryCadence)
					for idx, instance := range instances {
						if instance.IsLeader() {
							_ = instance.Close()
							if idx == len(instances)-1 {
								instances = instances[:idx]
							} else {
								instances = append(instances[:idx], instances[idx+1:]...)
							}
							break
						}
					}
				}
			}()
			return bluestore.AdaptFrom(ret)
		})
	})

	t.Run("Close on massive network of peers", func(t *testing.T) {
		const n = 100
		cfg := testConfig()

		lockFile := makeTempLockFile(t)
		svc := bluestore.AdaptTo(document.NewInMemoryService())

		instances := make([]*Service, 0, n)
		for i := 0; i < n; i++ {
			instance := New(svc, lockFile, cfg)
			_ = instance.Get(context.Background(), lockFile, nil)
			instances = append(instances, instance)
		}
		for i := 0; i < n; i++ {
			instance := instances[i]
			require.NoError(t, instance.Close())
		}
	})
}

func TestCustomRetryableErrors(t *testing.T) {
	myError := errors.New("dia de los muertos")
	tsuite := []struct {
		desc         string
		methodError  error
		closeError   error
		wantSuccess  bool
		makeInstance func(storageapi.Service, string, Config) (storageapi.Service, func())
	}{
		{"leader does not retry error passed in Config", myError, myError, false, makeLeader},
		{"follower retries close error passed in Config until success", myError, myError, true, makeFollower},
		{"follower does not retry any error other than the error passed in Config", errors.New("wasup"), myError, false, makeFollower},
		{"follower does not retry any error if error in config is nil", errors.New("wasup"), nil, false, makeFollower},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.desc, func(t *testing.T) {
			lockFile := makeTempLockFile(t)
			require.Error(t, tcase.methodError)
			mock := newTestService(tcase.methodError)
			cfg := testConfig()
			cfg.CloseError = tcase.closeError
			svc, doneFn := tcase.makeInstance(mock, lockFile, cfg)
			defer doneFn()
			err := svc.Set(context.Background(), "bluegrass", &testStruct{A: "1234"})
			if !tcase.wantSuccess {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func makeFollower(svc storageapi.Service, lockFile string, cfg Config) (storageapi.Service, func()) {
	var temp testStruct
	leader := New(svc, lockFile, cfg)
	_ = leader.Get(context.Background(), "bla", &temp)
	follower := New(svc, lockFile, cfg)
	_ = follower.Get(context.Background(), "bla", &temp)
	return follower, func() {
		_ = leader.Close()
		_ = follower.Close()
	}
}

func makeLeader(svc storageapi.Service, lockFile string, cfg Config) (storageapi.Service, func()) {
	leader := New(svc, lockFile, cfg)
	return leader, func() {
		_ = leader.Close()
	}
}

type testStruct struct {
	A string
}

func testConfig() Config {
	return Config{
		Marshaler:                      docbson.Marshaler(),
		TransientFailureRecoverTimeout: 450 * time.Millisecond,
		MethodRetryCadence:             20 * time.Millisecond,
		ReceiveRetryCadence:            500 * time.Millisecond,
		ConnectRetryCadence:            50 * time.Millisecond,
		TimeToCoup:                     500 * time.Millisecond,
		DialTimeout:                    50 * time.Millisecond,
		MaxMessageSize:                 DefaultMaxMessageSize * 2,
	}
}

type testService struct {
	err   error
	tries int
	svc   storageapi.Service
}

func newTestService(err error) storageapi.Service {
	return &testService{err: err, svc: storagestub.NewInMemoryService()}
}

func (t *testService) Set(ctx context.Context, ID string, doc any) error {
	if t.err == nil {
		panic("incorrect test case")
	}
	t.tries++
	if t.tries < 2 {
		return t.err
	}
	return t.svc.Set(ctx, ID, doc)
}

func (t *testService) Get(ctx context.Context, ID string, doc any) error {
	return t.svc.Get(ctx, ID, doc)
}

func (t *testService) Create(ctx context.Context, ID string, doc any) error {
	panic("unimplemented")
}

func (t *testService) Update(
	ctx context.Context, ID string, updates []storageapi.Update,
	preconds ...storageapi.Precondition,
) error {
	panic("unimplemented")
}

func (t *testService) Delete(ctx context.Context, ID string) error {
	panic("unimplemented")
}

func (t *testService) List(ctx context.Context, filters []storageapi.Filter) (storageapi.Iterator, error) {
	panic("unimplemented")
}

func (t *testService) Close() error {
	return nil
}

func makeTempLockFile(t *testing.T) string {
	f, err := os.CreateTemp("", "")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	require.NoError(t, os.Remove(f.Name()))
	t.Cleanup(func() {
		_ = os.Remove(f.Name())
	})
	return f.Name()
}

type alternatingService struct {
	i   atomic.Int64
	svc []document.Service
}

func (a *alternatingService) Create(ctx context.Context, ID string, doc interface{}) error {
	i := int(a.i.Add(1))
	return a.svc[i%len(a.svc)].Create(ctx, ID, doc)
}

func (a *alternatingService) Set(ctx context.Context, ID string, doc interface{}) error {
	i := int(a.i.Add(1))
	return a.svc[i%len(a.svc)].Set(ctx, ID, doc)
}

func (a *alternatingService) Update(
	ctx context.Context, ID string, updates []document.Update,
	precond ...document.Precondition,
) error {
	i := int(a.i.Add(1))
	return a.svc[i%len(a.svc)].Update(ctx, ID, updates, precond...)
}

func (a *alternatingService) Get(ctx context.Context, ID string, doc interface{}) error {
	i := int(a.i.Add(1))
	return a.svc[i%len(a.svc)].Get(ctx, ID, doc)
}

func (a *alternatingService) Delete(ctx context.Context, ID string) error {
	i := int(a.i.Add(1))
	return a.svc[i%len(a.svc)].Delete(ctx, ID)
}

func (a *alternatingService) List(ctx context.Context, filters []document.Filter) (document.Iterator, error) {
	i := int(a.i.Add(1))
	return a.svc[i%len(a.svc)].List(ctx, filters)
}

func (a *alternatingService) Close() (ret error) {
	for _, svc := range a.svc {
		ret = multierror.Append(ret, svc.Close())
	}
	return
}
