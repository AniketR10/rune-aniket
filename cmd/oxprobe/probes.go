// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package main

import (
	"net"
	"net/http"
	"runtime"
	"time"

	"unstable.build/go-tui/cmd/oxprobe/probe"
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
