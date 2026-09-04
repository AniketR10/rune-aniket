// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
	r, err := newStdRemote(ctx, cfg, uri, ui, nil, nil)
	if err != nil {
		return err
	}
	return r.Close()
}
