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

package main

import (
	"net"
	"net/http"
	"runtime"
	"time"

	"unstable.build/rune/cmd/oxprobe/probe"
)

// probeArch is the "<os>-<arch>" whose signed download URL the
// pkg_download check fetches, matching how a client on this host installs.
func probeArch() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

// buildProbes assembles the ordered probe set for an environment. The
// dns/tls layers are critical (a black hole there means total outage);
// the rest degrade. The deep probe is fanned out separately by the
// runner so its per-layer results land individually.
func buildProbes(cfg EnvConfig, client *http.Client, skipDownloadsCDN bool) []probe.Probe {
	resolver := &net.Resolver{}
	dialer := probe.NetTLSDialer{Timeout: 10 * time.Second}

	probes := []probe.Probe{
		probe.DNSProbe{
			LayerName: "dns_api", Host: cfg.APIHost,
			ExpectedIP: cfg.ExpectedAPIIP, IsCritical: true, Resolver: resolver,
		},
		probe.TLSProbe{
			LayerName: "tls_api", Addr: net.JoinHostPort(cfg.APIHost, "443"),
			MinValidity: cfg.CertExpiryWindow, IsCritical: true, Dialer: dialer,
		},
		probe.OAuthConfigProbe{
			APIURL: cfg.apiURL(), Expected: cfg.OAuth, IsCritical: true,
		},
		probe.Auth0Probe{
			AuthURL: cfg.authURL(), IsCritical: false, Client: client,
		},
	}
	if len(cfg.Archs) > 0 {
		probes = append(probes, probe.PkgLatestProbe{
			APIURL: cfg.apiURL(), Archs: cfg.Archs,
			IsCritical: true, Client: client,
		})
	}
	if !skipDownloadsCDN {
		probes = append(probes, probe.DownloadsCDNProbe{
			DownloadsHost: cfg.downloadsURL(), Archs: cfg.Archs,
			IsCritical: false, Client: client,
		})
	}
	return probes
}

// deepProbe builds the deep /health probe, which the runner fans out
// into per-layer results rather than treating as a single layer.
func deepProbe(cfg EnvConfig, client *http.Client, probeSecret string) probe.DeepProbe {
	return probe.DeepProbe{
		APIURL:      cfg.apiURL(),
		ProbeSecret: probeSecret,
		IsCritical:  false,
		Client:      client,
	}
}
