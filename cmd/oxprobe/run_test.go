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
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/ox-api/api/oxapi"
	"github.com/unstablebuild/ox-api/api/pager"

	"unstable.build/go-tui/cmd/oxprobe/probe"
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

func TestPageDedupKeysPerLayer(t *testing.T) {
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
	require.NoError(t, page(context.Background(), cp, "prod", report))
	require.Len(t, cp.pages, 2) // only failing critical layers
	keys := map[string]bool{}
	for _, p := range cp.pages {
		keys[p.DedupKey] = true
		require.Equal(t, pager.SeverityCritical, p.Severity)
		require.Equal(t, []pager.Link{{Href: runbookURL, Text: "oxprobe runbook"}}, p.Links)
	}
	require.True(t, keys["oxprobe-prod-dns_api"])
	require.True(t, keys["oxprobe-prod-tls_api"])
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
	report := r.run(context.Background(), 5*time.Second)
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
