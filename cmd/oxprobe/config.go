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
	"fmt"
	"net/url"
	"time"

	"unstable.build/rune/cmd/oxprobe/probe"
)

// EnvConfig is the per-environment matrix describing every host, IP, and
// expected oauth advertisement oxprobe asserts against.
type EnvConfig struct {
	Name string

	// GCPProject is the Google Cloud project whose Secret Manager holds
	// the probe secret and PagerDuty routing key for this environment.
	GCPProject string

	APIHost       string
	AuthHost      string
	DownloadsHost string

	// ExpectedAPIIP is the static load-balancer IP the api hostname must
	// resolve to. Empty disables the assertion.
	ExpectedAPIIP string

	OAuth probe.OAuthMatrix

	Archs            []string
	CertExpiryWindow time.Duration
}

// environments holds the static per-env matrices. IPs are intentionally
// left blank until the load-balancer addresses are allowlisted; an empty
// ExpectedIP turns the DNS probe into a resolves-at-all check.
var environments = map[string]EnvConfig{
	"staging": {
		Name:          "staging",
		GCPProject:    "unstable-build-blue-dev",
		APIHost:       "api.unstable.build",
		AuthHost:      "dev-fv7z5qrer6vkxhxf.us.auth0.com",
		DownloadsHost: "downloads.unstable.build",
		OAuth: probe.OAuthMatrix{
			TokenURL: "https://api.unstable.build/o/oauth2/token",
			AuthURL:  "https://dev-fv7z5qrer6vkxhxf.us.auth0.com/authorize",
			JWKSURL:  "https://dev-fv7z5qrer6vkxhxf.us.auth0.com/.well-known/jwks.json",
			ClientID: "AhY5YlLUiEjOXNFmmyw4Nve32Hp0ag22",
			Scopes:   []string{"offline_access", "openid"},
		},
		Archs: []string{
			"darwin-arm64", "darwin-amd64", "linux-amd64", "linux-arm64",
		},
		CertExpiryWindow: 14 * 24 * time.Hour,
	},
	"prod": {
		Name:          "prod",
		GCPProject:    "rune-prod",
		APIHost:       "api.rune.build",
		AuthHost:      "rune-prod.us.auth0.com",
		DownloadsHost: "downloads.rune.build",
		OAuth: probe.OAuthMatrix{
			TokenURL: "https://api.rune.build/o/oauth2/token",
			AuthURL:  "https://auth.rune.build/authorize",
			JWKSURL:  "https://auth.rune.build/.well-known/jwks.json",
			ClientID: "XHBpJIm3q6PYazpxZMAhcwxAuR5Ks9B7",
			Scopes:   []string{"offline_access", "openid"},
		},
		Archs: []string{
			"darwin-arm64", "darwin-amd64", "linux-amd64", "linux-arm64",
		},
		CertExpiryWindow: 14 * 24 * time.Hour,
	},
}

func (c EnvConfig) apiURL() *url.URL  { return &url.URL{Scheme: "https", Host: c.APIHost} }
func (c EnvConfig) authURL() *url.URL { return &url.URL{Scheme: "https", Host: c.AuthHost} }
func (c EnvConfig) downloadsURL() *url.URL {
	return &url.URL{Scheme: "https", Host: c.DownloadsHost}
}

func lookupEnv(name string) (EnvConfig, error) {
	cfg, ok := environments[name]
	if !ok {
		return EnvConfig{}, fmt.Errorf("unknown env %q (want one of unstable, prod)", name)
	}
	return cfg, nil
}
