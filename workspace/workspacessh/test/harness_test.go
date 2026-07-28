// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.

package workspacetest

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestContainerHelper boots two scenarios in parallel and confirms each
// reports a distinct host port.
func TestContainerHelper(t *testing.T) {
	SkipIfNoDocker(t)
	EnsureImage(t)

	scenarios := []SSHDScenario{
		{Name: "pubkey", PublicKeyFile: "/id_ed25519.pub"},
		{Name: "password", PasswordAccess: true, UserPassword: "hunter2"},
	}
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		ports = make(map[string]string)
	)
	for _, s := range scenarios {
		wg.Add(1)
		go func(s SSHDScenario) {
			defer wg.Done()
			c := StartContainer(t, s)
			mu.Lock()
			ports[s.Name] = c.HostPort
			mu.Unlock()
		}(s)
	}
	wg.Wait()
	assert.Len(t, ports, 2)
	assert.NotEqual(t, ports["pubkey"], ports["password"])
}
