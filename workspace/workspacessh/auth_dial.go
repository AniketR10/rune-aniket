// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.

package workspacessh

import (
	"context"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// AuthDialOptions configures TestAuthDial.
type AuthDialOptions struct {
	PrivateKeys    []string
	Insecure       bool
	KnownHostsPath string
	Timeout        time.Duration
	KbdInteractive bool
	// StrictHostKeyChecking mirrors workspace.ssh.strict_host_key_checking.
	// When true, an unknown or changed host key prompts the UI before the
	// presented key is trusted and recorded.
	StrictHostKeyChecking bool
}

// TestAuthDial is a thin wrapper around the std remote dial path. It is
// intended for integration tests that want to exercise the SSH auth UX
// without going through the full workspace-scheme initialization (which
// requires the `rune` workspace-server binary on the remote host).
//
// On success the underlying remote is closed before returning.
func TestAuthDial(
	ctx context.Context, ui UI, uri workspaceapi.URI, opts AuthDialOptions,
) error {
	cfg := sshConfig{
		privateKeys:           opts.PrivateKeys,
		insecure:              opts.Insecure,
		knownHostsPath:        opts.KnownHostsPath,
		timeout:               opts.Timeout,
		kbdInteractive:        opts.KbdInteractive,
		strictHostKeyChecking: opts.StrictHostKeyChecking,
	}
	if cfg.timeout == 0 {
		cfg.timeout = defSSHTimeout
	}
	r, err := newStdRemote(ctx, cfg, uri, ui)
	if err != nil {
		return err
	}
	return r.Close()
}
