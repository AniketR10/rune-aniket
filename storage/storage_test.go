package storage

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/document/firstmover"
	"github.com/unstablebuild/blue/document/test"
	"github.com/unstablebuild/blue/encoding/toml"
	"github.com/stretchr/testify/require"
)

func TestStorageConcurrentInstances(t *testing.T) {
	dirs := make(map[string][]document.Service)
	wgs := make([]*sync.WaitGroup, 0)

	// NOTE: don't test List as the returned iterator from it is not fully resilient
	// to changes in leader; certaintly not to such an aggressive test.
	test.TestDocumentServiceNoList(t, func(t *testing.T) document.Service {
		const n = 100
		name, err := ioutil.TempDir("", "workspace_document_service_test")
		require.NoError(t, err)
		if _, ok := dirs[name]; ok {
			require.NoError(t, fmt.Errorf("created a duplicate temp dir: %s", name))
		}
		dirs[name] = make([]document.Service, 0, n)
		instances := make([]document.Service, 0, n)

		for i := 0; i < n-1; i++ {
			instance, err := New(context.Background(), name, toml.Marshaler())
			require.NoError(t, err)
			_ = instance.Get(context.Background(), "a", nil)
			instances = append(instances, instance)
			dirs[name] = append(dirs[name], instance)
		}
		ret := instances[len(instances)-1]

		wg := new(sync.WaitGroup)
		wgs = append(wgs, wg)
		wg.Add(n - 1)
		go func() {
			for i := 0; i < n-1; i++ {
				defer wg.Done()
				time.Sleep(time.Duration(200 * time.Millisecond))
				for idx, instance := range instances {
					if firstmover.TestIsLeader(instance) {
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
		return ret
	})

	// wait for all the slaughter to finish
	for _, wg := range wgs {
		wg.Wait()
	}

	for dir, svcs := range dirs {
		for _, svc := range svcs {
			svc.Close()
		}
		_ = os.RemoveAll(dir)
	}
}
