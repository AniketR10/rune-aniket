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
	"net/http/httptest"
	"testing"

	"tailscale.com/net/netns"
	"tailscale.com/tstest/integration"
	"tailscale.com/tstest/integration/testcontrol"
	"tailscale.com/types/logger"
)

// StartTestControl runs an in-process coordination server plus a local
// DERP and STUN server. It is the docker-free stand-in for Headscale:
// the nodes still speak the real control protocol and still carry real
// WireGuard traffic, so everything above the coordination server is
// exercised exactly as in production.
//
// sameUser controls whether the registered nodes belong to one account,
// which is what [runenet.PeerAuthorizer] admits or rejects on.
func StartTestControl(t *testing.T, sameUser bool) ControlPlane {
	t.Helper()

	// Tests must not bind their sockets to a physical interface: the
	// nodes talk to each other over loopback.
	netns.SetEnabled(false)
	t.Cleanup(func() { netns.SetEnabled(true) })

	control := &testcontrol.Server{
		DERPMap:          integration.RunDERPAndSTUN(t, logger.Discard, "127.0.0.1"),
		MagicDNSDomain:   "rune.test",
		AllNodesSameUser: sameUser,
		Logf:             logger.Discard,
	}
	server := httptest.NewUnstartedServer(control)
	server.Start()
	t.Cleanup(server.Close)

	return ControlPlane{URL: server.URL}
}
