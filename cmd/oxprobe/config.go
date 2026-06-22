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
	"fmt"
	"net/url"
	"time"

	"unstable.build/go-tui/cmd/oxprobe/probe"
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
