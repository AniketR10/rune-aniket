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

package workspacetest

import (
	"context"
	"fmt"
	"path"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/workspace/workspacessh"
	"unstable.build/go-tui/workspace/workspacetest"
)

// TestIntegrationScheme exercises the full schemeapi.Scheme contract
// (Open / Read / Stat / Remove / Rename / ...) over both the std (Go
// crypto/ssh) and proc (forked openssh) remotes against a real
// container.
//
// The suite issues many ssh connections in quick succession against a
// single shared container, so the harness raises MaxStartups well past
// the linuxserver/openssh-server default via ExtraSSHDConfig. See also
// TestConnectSchemeEndToEnd (connect_test.go) for connectScheme
// coverage and TestAuthMatrix for SSH auth UX coverage.
func TestIntegrationScheme(t *testing.T) {
	SkipIfNoDocker(t)
	EnsureImage(t)
	c := StartContainer(t, SSHDScenario{
		PublicKeyFile:     "/id_ed25519.pub",
		InstallRuneBinary: true,
		ExtraSSHDConfig: "MaxStartups 200:30:400\n" +
			"MaxSessions 200\n" +
			"LoginGraceTime 60\n",
	})

	keyPath := PrivateKeyPath(t, "id_ed25519")

	cfgs := map[string]config.Config{
		"openssh_proc_remote": config.MapConfig(map[string]any{
			"command": "ssh -o StrictHostKeyChecking=no -i " + keyPath + " %h -p %p",
			"timeout": "20s",
		}),
		"go_stdlib_remote": config.MapConfig(map[string]any{
			"private_keys": []any{keyPath},
			"timeout":      "20s",
			"insecure":     true,
		}),
	}
	for desc, cfg := range cfgs {
		t.Run(desc, func(t *testing.T) {
			workspacetest.TestWorkspaceSchemeFiles(t, func(t *testing.T) schemeapi.Scheme {
				return newSchemeIntegration(t, c.HostPort, cfg)
			})

			workspacetest.TestWorkspaceSchemeExecutor(t, func(t *testing.T) schemeapi.Scheme {
				return newSchemeIntegration(t, c.HostPort, cfg)
			})
		})
	}
}

// integrationDirCounter ensures each schemeFn(t) call gets its own
// subdirectory under /tmp so the per-subtest filesystem state cannot
// leak into the next subtest. The container is reused across subtests
// (see TestIntegrationScheme) for connection-rate reasons; a unique
// workspace dir per subtest is the cheap way to keep them isolated.
var integrationDirCounter atomic.Uint64

func newSchemeIntegration(
	t *testing.T, hostname string, cfg config.Config,
) schemeapi.Scheme {
	t.Helper()

	// Pick a workspace directory unique to this subtest. Just the
	// counter would suffice, but keeping the test name in the path
	// makes it grep-friendly when something goes wrong. Strip every
	// non-[A-Za-z0-9_-] rune so we never have to worry about shell
	// quoting (the bootstrap forwards the path through `bash -c`).
	dir := path.Join("/tmp", fmt.Sprintf("rune_ssh_test_%s_%d",
		safeName(t.Name()), integrationDirCounter.Add(1)))

	workspaceURI, err := workspaceapi.ParseURI(
		"ssh://test@" + hostname + dir)
	require.NoError(t, err)

	ctx := context.Background()

	schemeFn := workspacessh.New(&errorUI{})

	// Pre-create the workspace dir on the remote so the bootstrap's
	// `ls $path` check succeeds before the gRPC channel comes up.
	mkdir, err := schemeFn(ctx, cfg, parentURI(t, workspaceURI))
	require.NoError(t, err)
	require.NoError(t, mkdir.MkdirAll(dir, 0o755))
	mkdir.Close()

	s, err := schemeFn(ctx, cfg, workspaceURI)
	require.NoError(t, err)

	t.Cleanup(func() {
		// The container is torn down by the parent test's t.Cleanup,
		// so we don't bother removing the per-subtest dir from the
		// remote — the entire fs is gone shortly after this returns,
		// and a per-file `rm` would multiply many small gRPC
		// round-trips through the SSH tunnel. Just close the
		// scheme to release its ssh session.
		_ = s.Close()
	})
	return s
}

// parentURI returns a URI pointing at /tmp on the remote — used to run
// MkdirAll for a fresh per-subtest workspace dir before connecting the
// real workspace scheme.
func parentURI(t *testing.T, u workspaceapi.URI) workspaceapi.URI {
	t.Helper()
	parent, err := workspaceapi.ParseURI(
		"ssh://" + u.User() + "@" + u.Host() + "/tmp")
	require.NoError(t, err)
	return parent
}

// errorUI fails any prompt request: this integration test only verifies
// non-interactive flows (configured key, healthy server).
type errorUI struct{}

func (errorUI) PromptSecret(context.Context, string) (string, error) {
	return "", fmt.Errorf("unexpected prompt: secret")
}

func (errorUI) PromptText(context.Context, string, string) (string, error) {
	return "", fmt.Errorf("unexpected prompt: text")
}

func (errorUI) PromptChoice(context.Context, string, []string) (int, error) {
	return -1, fmt.Errorf("unexpected prompt: choice")
}

func (errorUI) Notify(workspacessh.NotificationLevel, string) {}

// safeName returns a shell-safe version of name suitable for substitution
// into a path the bootstrap may quote naively.
func safeName(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}
