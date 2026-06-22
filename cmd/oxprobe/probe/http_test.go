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

package probe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/ox-api/api/oxapi"
	"github.com/unstablebuild/ox-api/auth"
)

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}

func TestOAuthConfigProbe(t *testing.T) {
	matrix := OAuthMatrix{
		TokenURL: "https://api.example/o/oauth2/token",
		AuthURL:  "https://api.example/o/oauth2/auth",
		JWKSURL:  "https://api.example/o/oauth2/jwk",
		ClientID: "client-123",
		Scopes:   []string{"openid", "offline_access"},
	}
	good := auth.Config{JWKSURL: matrix.JWKSURL}
	good.ClientID = matrix.ClientID
	good.Scopes = []string{"offline_access", "openid"} // order-insensitive
	good.Endpoint.TokenURL = matrix.TokenURL
	good.Endpoint.AuthURL = matrix.AuthURL

	tests := []struct {
		name     string
		cfg      auth.Config
		wantFail bool
	}{
		{name: "matches", cfg: good, wantFail: false},
		{
			name: "token url mismatch",
			cfg: func() auth.Config {
				c := good
				c.Endpoint.TokenURL = "https://evil/token"
				return c
			}(),
			wantFail: true,
		},
		{
			name: "scope mismatch",
			cfg: func() auth.Config {
				c := good
				c.Scopes = []string{"openid"}
				return c
			}(),
			wantFail: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := OAuthConfigProbe{
				APIURL:     mustURL(t, "https://api.example"),
				Expected:   matrix,
				IsCritical: true,
				Fetch: func(api *url.URL) (auth.Config, error) {
					return tc.cfg, nil
				},
			}
			res := p.Run(context.Background())
			if tc.wantFail {
				require.Equal(t, oxapi.CheckFail, res.Status, res.Detail)
			} else {
				require.Equal(t, oxapi.CheckOK, res.Status, res.Detail)
			}
		})
	}
}

func TestAuth0Probe(t *testing.T) {
	discoveryOK := `{"scopes_supported":["openid","profile"],"id_token_signing_alg_values_supported":["RS256"]}`

	tests := []struct {
		name          string
		discoveryBody string
		discoveryCode int
		tokenCode     int
		wantFail      bool
	}{
		{name: "healthy", discoveryBody: discoveryOK, discoveryCode: 200, tokenCode: 400, wantFail: false},
		{name: "401 client unknown also healthy", discoveryBody: discoveryOK, discoveryCode: 200, tokenCode: 401, wantFail: false},
		{name: "token 404 fails", discoveryBody: discoveryOK, discoveryCode: 200, tokenCode: 404, wantFail: true},
		{name: "missing rs256 fails", discoveryBody: `{"scopes_supported":["openid"],"id_token_signing_alg_values_supported":["HS256"]}`, discoveryCode: 200, tokenCode: 400, wantFail: true},
		{name: "discovery 500 fails", discoveryBody: discoveryOK, discoveryCode: 500, tokenCode: 400, wantFail: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/.well-known/openid-configuration":
					w.WriteHeader(tc.discoveryCode)
					_, _ = w.Write([]byte(tc.discoveryBody))
				case "/oauth/token":
					w.WriteHeader(tc.tokenCode)
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer srv.Close()

			p := Auth0Probe{AuthURL: mustURL(t, srv.URL), Client: srv.Client()}
			res := p.Run(context.Background())
			if tc.wantFail {
				require.Equal(t, oxapi.CheckFail, res.Status, res.Detail)
			} else {
				require.Equal(t, oxapi.CheckOK, res.Status, res.Detail)
			}
		})
	}
}

func TestDownloadsCDNProbe(t *testing.T) {
	seen := map[string]bool{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodHead, r.Method)
		seen[r.URL.Path] = true
		if r.URL.Path == "/linux-amd64/manifest.json" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ok := DownloadsCDNProbe{
		DownloadsHost: mustURL(t, srv.URL), Archs: []string{"darwin-arm64"}, Client: srv.Client(),
	}
	require.Equal(t, oxapi.CheckOK, ok.Run(context.Background()).Status)

	bad := DownloadsCDNProbe{
		DownloadsHost: mustURL(t, srv.URL), Archs: []string{"linux-amd64"}, Client: srv.Client(),
	}
	require.Equal(t, oxapi.CheckFail, bad.Run(context.Background()).Status)
	require.True(t, seen["/darwin-arm64/manifest.json"])
}

func TestDeepProbeFanout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "1", r.URL.Query().Get("verbose"))
		require.Equal(t, "good-secret", r.Header.Get("X-Probe-Secret"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"degraded","checks":[{"layer":"stripe","status":"fail"},{"layer":"keys","status":"ok"}],"signed_downloads":{"darwin-arm64":"https://signed.example/ada"}}`))
	}))
	defer srv.Close()

	p := DeepProbe{APIURL: mustURL(t, srv.URL), ProbeSecret: "good-secret", Client: srv.Client()}
	results, report, ok := p.Fanout(context.Background())
	require.True(t, ok)
	require.Len(t, results, 3)
	layers := map[string]oxapi.CheckStatus{}
	for _, r := range results {
		layers[r.Layer] = r.Status
	}
	require.Equal(t, oxapi.CheckOK, layers["deep"])
	require.Contains(t, layers, "deep.stripe")
	require.Contains(t, layers, "deep.keys")
	require.Equal(t, "https://signed.example/ada", report.SignedDownloads["darwin-arm64"])
}

func TestDeepProbeBadSecret(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Body-less response, as the server gives unauthorized verbose requests.
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := DeepProbe{APIURL: mustURL(t, srv.URL), ProbeSecret: "wrong", Client: srv.Client()}
	results, _, ok := p.Fanout(context.Background())
	require.False(t, ok)
	require.Len(t, results, 1)
	require.Equal(t, "deep", results[0].Layer)
	require.Equal(t, oxapi.CheckFail, results[0].Status)
}
