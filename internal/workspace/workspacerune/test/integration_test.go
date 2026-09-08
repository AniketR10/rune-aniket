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

package runetest

import (
	"testing"

	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"unstable.build/rune/internal/workspace/workspacetest"
)

// TestIntegrationScheme is the full-stack end-to-end test for the
// rune:// scheme: a Headscale coordination server in docker, a second
// Rune instance in its own process joined to it, and the shared
// workspace scheme conformance suites driven across the network from
// this process.
//
// It is the same shape as the deployed system — the only thing the
// tests substitute is where the two instances happen to run.
func TestIntegrationScheme(t *testing.T) {
	SkipIfRace(t)
	SkipIfNoDocker(t)

	control := StartHeadscale(t)
	StartInstance(t, control, "peer")

	client := StartNode(t, control, "client")
	WaitPeer(t, client, "peer")

	newScheme := func(t *testing.T) schemeapi.Scheme {
		return OpenWorkspace(t, client, "peer", t.TempDir())
	}
	workspacetest.TestWorkspaceSchemeFiles(t, newScheme)
	workspacetest.TestWorkspaceSchemeExecutor(t, newScheme)
}
