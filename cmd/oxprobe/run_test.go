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
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/ox-api/api/oxapi"
	"github.com/unstablebuild/ox-api/api/pager"

	"unstable.build/rune/cmd/oxprobe/probe"
)

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}

type capturePager struct {
	pages []pager.Page
}

func (c *capturePager) Page(ctx context.Context, p pager.Page) error {
	c.pages = append(c.pages, p)
	return nil
}

// flakyProbe fails its first failUntil runs and succeeds afterwards.
type flakyProbe struct {
	layer     string
	critical  bool
	failUntil int
	runs      atomic.Int64
}

func (f *flakyProbe) Layer() string  { return f.layer }
func (f *flakyProbe) Critical() bool { return f.critical }

func (f *flakyProbe) Run(ctx context.Context) oxapi.CheckResult {
	n := f.runs.Add(1)
	res := oxapi.CheckResult{Layer: f.layer, Status: oxapi.CheckOK, Critical: f.critical}
	if int(n) <= f.failUntil {
		res.Status = oxapi.CheckFail
		res.Detail = "flaky"
	}
	return res
}

func TestNewRunnerCanSkipDownloadsCDN(t *testing.T) {
	cfg := EnvConfig{
		Name:             "test",
		APIHost:          "api.example",
		AuthHost:         "auth.example",
		DownloadsHost:    "downloads.example",
		ExpectedAPIIP:    "127.0.0.1",
		CertExpiryWindow: time.Hour,
	}

	r := newRunner(cfg, http.DefaultClient, "secret", runnerConfig{SkipDownloadsCDN: true})
	for _, p := range r.probes {
		require.NotEqual(t, "downloads_cdn", p.Layer())
	}
}

func TestCriticalFailuresSelectsPagingGroups(t *testing.T) {
	tests := []struct {
		name    string
		results [][]oxapi.CheckResult
		want    []int
	}{
		{
			name: "all ok",
			results: [][]oxapi.CheckResult{
				{{Layer: "dns_api", Status: oxapi.CheckOK, Critical: true}},
				{{Layer: "auth0", Status: oxapi.CheckOK}},
			},
		},
		{
			name: "non-critical failure is not selected",
			results: [][]oxapi.CheckResult{
				{{Layer: "auth0", Status: oxapi.CheckFail, Critical: false}},
			},
		},
		{
			name: "critical failure selects its group",
			results: [][]oxapi.CheckResult{
				{{Layer: "dns_api", Status: oxapi.CheckOK, Critical: true}},
				{{Layer: "tls_api", Status: oxapi.CheckFail, Critical: true}},
			},
			want: []int{1},
		},
		{
			name: "critical failure anywhere in the deep group selects it once",
			results: [][]oxapi.CheckResult{
				{
					{Layer: "deep.stripe", Status: oxapi.CheckFail, Critical: false},
					{Layer: "deep", Status: oxapi.CheckFail, Critical: true},
					{Layer: "pkg_download", Status: oxapi.CheckFail, Critical: true},
				},
			},
			want: []int{0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, criticalFailures(tt.results))
		})
	}
}

func TestRunnerConfirmsCriticalFailures(t *testing.T) {
	tests := []struct {
		name            string
		probe           *flakyProbe
		confirmDelay    time.Duration
		wantStatus      string
		wantRuns        int64
		wantUnconfirmed []string
	}{
		{
			name:            "transient critical failure does not fail the report",
			probe:           &flakyProbe{layer: "dns_api", critical: true, failUntil: 1},
			confirmDelay:    time.Millisecond,
			wantStatus:      oxapi.StatusOK,
			wantRuns:        2,
			wantUnconfirmed: []string{"dns_api"},
		},
		{
			name:         "persistent critical failure fails the report",
			probe:        &flakyProbe{layer: "dns_api", critical: true, failUntil: 2},
			confirmDelay: time.Millisecond,
			wantStatus:   oxapi.StatusFail,
			wantRuns:     2,
		},
		{
			name:         "non-critical failure is not re-run",
			probe:        &flakyProbe{layer: "auth0", critical: false, failUntil: 1},
			confirmDelay: time.Millisecond,
			wantStatus:   oxapi.StatusDegraded,
			wantRuns:     1,
		},
		{
			name:         "zero delay pages on the first failure",
			probe:        &flakyProbe{layer: "dns_api", critical: true, failUntil: 1},
			confirmDelay: 0,
			wantStatus:   oxapi.StatusFail,
			wantRuns:     1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &runner{
				env:          EnvConfig{Name: "test"},
				probes:       []probe.Probe{tt.probe},
				confirmDelay: tt.confirmDelay,
			}
			report, unconfirmed := r.run(context.Background(), time.Second)
			require.Equal(t, tt.wantStatus, report.Status)
			require.Equal(t, tt.wantRuns, tt.probe.runs.Load())
			require.Equal(t, tt.wantUnconfirmed, unconfirmed)
		})
	}
}

func TestRunnerSkipsConfirmationOnCanceledContext(t *testing.T) {
	p := &flakyProbe{layer: "dns_api", critical: true, failUntil: 1}
	r := &runner{
		env:          EnvConfig{Name: "test"},
		probes:       []probe.Probe{p},
		confirmDelay: time.Hour,
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	report, unconfirmed := r.run(ctx, time.Second)
	require.Equal(t, oxapi.StatusFail, report.Status)
	require.Empty(t, unconfirmed)
	require.Equal(t, int64(1), p.runs.Load())
}

func TestReconcileTriggersAndResolvesPerLayer(t *testing.T) {
	cp := &capturePager{}
	report := oxapi.Report{
		Status: oxapi.StatusFail,
		Checks: []oxapi.CheckResult{
			{Layer: "dns_api", Status: oxapi.CheckFail, Critical: true, Detail: "nxdomain"},
			{Layer: "auth0", Status: oxapi.CheckFail, Critical: false},
			{Layer: "tls_api", Status: oxapi.CheckFail, Critical: true},
			{Layer: "keys", Status: oxapi.CheckOK, Critical: true},
		},
	}
	require.NoError(t, reconcile(context.Background(), cp, "prod", report))

	byKey := map[string]pager.Page{}
	for _, p := range cp.pages {
		byKey[p.DedupKey] = p
	}
	require.Len(t, cp.pages, 3) // every critical layer, not the non-critical auth0

	for _, key := range []string{"oxprobe-prod-dns_api", "oxprobe-prod-tls_api"} {
		trig := byKey[key]
		require.Empty(t, trig.Action) // empty Action defaults to trigger
		require.Equal(t, pager.SeverityCritical, trig.Severity)
		require.Equal(t, []pager.Link{{Href: runbookURL, Text: "oxprobe runbook"}}, trig.Links)
	}

	resolved := byKey["oxprobe-prod-keys"]
	require.Equal(t, pager.ActionResolve, resolved.Action)

	_, hasAuth0 := byKey["oxprobe-prod-auth0"]
	require.False(t, hasAuth0)
}

func TestReconcileResolvesWhenReportOK(t *testing.T) {
	cp := &capturePager{}
	report := oxapi.Report{
		Status: oxapi.StatusOK,
		Checks: []oxapi.CheckResult{
			{Layer: "dns_api", Status: oxapi.CheckOK, Critical: true},
			{Layer: "auth0", Status: oxapi.CheckOK, Critical: false},
		},
	}
	require.NoError(t, reconcile(context.Background(), cp, "prod", report))
	require.Len(t, cp.pages, 1) // only the critical layer is resolved
	require.Equal(t, pager.ActionResolve, cp.pages[0].Action)
	require.Equal(t, "oxprobe-prod-dns_api", cp.pages[0].DedupKey)
}

func TestRunnerFansDeepIntoAggregate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.Header().Set("Content-Type", "application/json")
			signed := "http://" + r.Host + "/signed/ada"
			_, _ = w.Write([]byte(`{"status":"degraded","checks":[{"layer":"stripe","status":"fail"}],"signed_downloads":{"` + probeArch() + `":"` + signed + `"}}`))
		case "/signed/ada":
			_, _ = w.Write([]byte("ada-bytes"))
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	r := &runner{
		env: EnvConfig{Name: "test"},
		deep: probe.DeepProbe{
			APIURL:      mustParseURL(t, srv.URL),
			ProbeSecret: "secret",
			Client:      srv.Client(),
		},
		deepFanIn: true,
		client:    srv.Client(),
		arch:      probeArch(),
	}
	report, _ := r.run(context.Background(), 5*time.Second)
	require.Equal(t, oxapi.StatusDegraded, report.Status)
	var found bool
	for _, c := range report.Checks {
		if c.Layer == "deep.stripe" {
			found = true
		}
	}
	require.True(t, found, "expected deep.stripe in fanned results")
	var pkgOK bool
	for _, c := range report.Checks {
		if c.Layer == "pkg_download" {
			pkgOK = c.Status == oxapi.CheckOK
		}
	}
	require.True(t, pkgOK, "expected pkg_download to succeed via signed url")
}
