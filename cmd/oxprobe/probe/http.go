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

package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"

	"github.com/unstablebuild/ox-api/api/oxapi"
	"github.com/unstablebuild/ox-api/auth"
)

// OAuthMatrix is the expected advertised oauth2 configuration for an
// environment. Empty fields are not asserted.
type OAuthMatrix struct {
	TokenURL string
	AuthURL  string
	JWKSURL  string
	ClientID string
	Scopes   []string
}

// OAuthConfigProbe fetches /o/oauth2/config from the api host and asserts
// the advertised endpoints match the per-env matrix.
type OAuthConfigProbe struct {
	APIURL     *url.URL
	Expected   OAuthMatrix
	IsCritical bool
	// Fetch defaults to auth.FetchConfig; overridable in tests.
	Fetch func(api *url.URL) (auth.Config, error)
}

// Layer implements Probe.
func (p OAuthConfigProbe) Layer() string { return "oauth_config" }

// Critical implements Probe.
func (p OAuthConfigProbe) Critical() bool { return p.IsCritical }

// Run implements Probe.
func (p OAuthConfigProbe) Run(ctx context.Context) oxapi.CheckResult {
	return run(ctx, "oauth_config", p.IsCritical, func(ctx context.Context) (string, error) {
		fetch := p.Fetch
		if fetch == nil {
			fetch = auth.FetchConfig
		}
		cfg, err := fetch(p.APIURL)
		if err != nil {
			return "", fmt.Errorf("fetch config: %w", err)
		}
		var mismatches []string
		check := func(name, want, got string) {
			if want != "" && want != got {
				mismatches = append(mismatches, fmt.Sprintf("%s=%q want %q", name, got, want))
			}
		}
		check("token_url", p.Expected.TokenURL, cfg.Endpoint.TokenURL)
		check("auth_url", p.Expected.AuthURL, cfg.Endpoint.AuthURL)
		check("jwks_url", p.Expected.JWKSURL, cfg.JWKSURL)
		check("client_id", p.Expected.ClientID, cfg.ClientID)
		if len(p.Expected.Scopes) > 0 && !sameStringSet(p.Expected.Scopes, cfg.Scopes) {
			mismatches = append(mismatches, fmt.Sprintf("scopes=%v want %v", cfg.Scopes, p.Expected.Scopes))
		}
		if len(mismatches) > 0 {
			return "", fmt.Errorf("oauth config mismatch: %s", strings.Join(mismatches, "; "))
		}
		return "config matches matrix", nil
	})
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ac := append([]string(nil), a...)
	bc := append([]string(nil), b...)
	sort.Strings(ac)
	sort.Strings(bc)
	for i := range ac {
		if ac[i] != bc[i] {
			return false
		}
	}
	return true
}

// Auth0Probe verifies the auth0 tenant by checking its OIDC discovery
// document advertises RS256 + openid, and that the token endpoint
// rejects a bogus authorization code with HTTP 400 (a 404 means the
// tenant or endpoint moved).
type Auth0Probe struct {
	AuthURL    *url.URL
	IsCritical bool
	Client     *http.Client
}

// Layer implements Probe.
func (p Auth0Probe) Layer() string { return "auth0" }

// Critical implements Probe.
func (p Auth0Probe) Critical() bool { return p.IsCritical }

// Run implements Probe.
func (p Auth0Probe) Run(ctx context.Context) oxapi.CheckResult {
	return run(ctx, "auth0", p.IsCritical, func(ctx context.Context) (string, error) {
		disco := p.AuthURL.JoinPath(".well-known", "openid-configuration").String()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, disco, nil)
		if err != nil {
			return "", err
		}
		resp, err := p.Client.Do(req)
		if err != nil {
			return "", fmt.Errorf("discovery: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("discovery status %d", resp.StatusCode)
		}
		var doc struct {
			ScopesSupported                  []string `json:"scopes_supported"`
			IDTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
			return "", fmt.Errorf("discovery decode: %w", err)
		}
		if !slices.Contains(doc.ScopesSupported, "openid") {
			return "", fmt.Errorf("discovery missing openid scope")
		}
		if !slices.Contains(doc.IDTokenSigningAlgValuesSupported, "RS256") {
			return "", fmt.Errorf("discovery missing RS256 signing alg")
		}

		tokenURL := p.AuthURL.JoinPath("oauth", "token").String()
		form := url.Values{
			"grant_type":   {"authorization_code"},
			"code":         {"oxprobe-bogus-code"},
			"redirect_uri": {"https://oxprobe.invalid/callback"},
			"client_id":    {"oxprobe"},
		}
		tokReq, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL,
			strings.NewReader(form.Encode()))
		if err != nil {
			return "", err
		}
		tokReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		tokResp, err := p.Client.Do(tokReq)
		if err != nil {
			return "", fmt.Errorf("token: %w", err)
		}
		defer tokResp.Body.Close()
		// A live token endpoint rejects a bogus authorization_code with a
		// 4xx error (Auth0 returns 400 for a bad code and 401 when the
		// client is unknown). A 404 means the endpoint moved; 5xx means
		// the tenant is unhealthy. Both are failures.
		if tokResp.StatusCode != http.StatusBadRequest && tokResp.StatusCode != http.StatusUnauthorized {
			return "", fmt.Errorf("token endpoint returned %d for bogus code, want 400/401", tokResp.StatusCode)
		}
		return "tenant healthy", nil
	})
}

// DownloadsCDNProbe issues a HEAD for each arch's manifest.json on the
// downloads host and fails if any does not return 200.
type DownloadsCDNProbe struct {
	DownloadsHost *url.URL
	Archs         []string
	IsCritical    bool
	Client        *http.Client
}

// Layer implements Probe.
func (p DownloadsCDNProbe) Layer() string { return "downloads_cdn" }

// Critical implements Probe.
func (p DownloadsCDNProbe) Critical() bool { return p.IsCritical }

// Run implements Probe.
func (p DownloadsCDNProbe) Run(ctx context.Context) oxapi.CheckResult {
	return run(ctx, "downloads_cdn", p.IsCritical, func(ctx context.Context) (string, error) {
		for _, arch := range p.Archs {
			u := p.DownloadsHost.JoinPath(arch, "manifest.json").String()
			req, err := http.NewRequestWithContext(ctx, http.MethodHead, u, nil)
			if err != nil {
				return "", err
			}
			resp, err := p.Client.Do(req)
			if err != nil {
				return "", fmt.Errorf("head %s: %w", arch, err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return "", fmt.Errorf("%s manifest status %d", arch, resp.StatusCode)
			}
		}
		return fmt.Sprintf("%d manifests reachable", len(p.Archs)), nil
	})
}

// DeepProbe queries the api host's verbose /health endpoint with the
// probe secret and fans the server's per-layer checks into the probe
// result set. A failed deep request is itself reported as the "deep"
// layer result.
type DeepProbe struct {
	APIURL      *url.URL
	ProbeSecret string
	IsCritical  bool
	Client      *http.Client
}

// Layer implements Probe.
func (p DeepProbe) Layer() string { return "deep" }

// Critical implements Probe.
func (p DeepProbe) Critical() bool { return p.IsCritical }

// Run implements Probe.
func (p DeepProbe) Run(ctx context.Context) oxapi.CheckResult {
	return run(ctx, "deep", p.IsCritical, func(ctx context.Context) (string, error) {
		report, err := p.fetch(ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("server status %s (%d layers)", report.Status, len(report.Checks)), nil
	})
}

// Fanout fetches the deep /health report and returns the server's
// per-layer checks re-prefixed as "deep.<layer>" so they slot into the
// probe's result set without colliding with local layer names. It also
// returns the decoded report (so callers can read SignedDownloads) and a
// bool reporting whether the fetch itself succeeded.
func (p DeepProbe) Fanout(ctx context.Context) ([]oxapi.CheckResult, oxapi.Report, bool) {
	report, err := p.fetch(ctx)
	if err != nil {
		return []oxapi.CheckResult{{
			Layer:    "deep",
			Status:   oxapi.CheckFail,
			Critical: p.IsCritical,
			Detail:   err.Error(),
		}}, oxapi.Report{}, false
	}
	status := oxapi.CheckOK
	if report.Status == oxapi.StatusFail {
		status = oxapi.CheckFail
	}
	out := []oxapi.CheckResult{{
		Layer:    "deep",
		Status:   status,
		Critical: p.IsCritical,
		Detail:   fmt.Sprintf("server status %s (%d layers)", report.Status, len(report.Checks)),
	}}
	for _, c := range report.Checks {
		c.Layer = "deep." + c.Layer
		out = append(out, c)
	}
	return out, report, true
}

func (p DeepProbe) fetch(ctx context.Context) (oxapi.Report, error) {
	u := *p.APIURL
	u.Path = "/health"
	q := u.Query()
	q.Set("verbose", "1")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return oxapi.Report{}, err
	}
	req.Header.Set("X-Probe-Secret", p.ProbeSecret)
	req.Header.Set("Accept", "application/json")
	resp, err := p.Client.Do(req)
	if err != nil {
		return oxapi.Report{}, fmt.Errorf("deep health: %w", err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		return oxapi.Report{}, fmt.Errorf("deep health returned no report (status %d, content-type %q); check probe secret", resp.StatusCode, ct)
	}
	var report oxapi.Report
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		return oxapi.Report{}, fmt.Errorf("deep health decode: %w", err)
	}
	return report, nil
}
