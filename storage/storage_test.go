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
package storage

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/document/firstmover"
	"github.com/unstablebuild/blue/document/test"
	"github.com/unstablebuild/blue/encoding/toml"
)

func TestStorageConcurrentInstances(t *testing.T) {
	dirs := make(map[string][]document.Service)
	wgs := make([]*sync.WaitGroup, 0)

	// NOTE: don't test List as the returned iterator from it is not fully resilient
	// to changes in leader; certaintly not to such an aggressive test.
	test.TestDocumentServiceNoList(t, func(t *testing.T) document.Service {
		const n = 100
		name, err := os.MkdirTemp("", "workspace_document_service_test")
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
