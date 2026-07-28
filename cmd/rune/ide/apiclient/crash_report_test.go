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

package apiclient

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/unstablebuild/ox-api/auth"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"golang.org/x/oauth2"
	"unstable.build/go-tui/cmd/rune/crashreport"
)

// newValidTestTokenSource creates a CachedTokenSource backed by a static
// oauth2 token, so that Token() returns a valid token for test assertions.
func newValidTestTokenSource() *auth.CachedTokenSource {
	ts := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: "test-token",
		TokenType:   "Bearer",
	})
	return auth.NewCachedTokenSource(
		auth.FuncTokenSourcer(func(_ context.Context, _ *oauth2.Token) (oauth2.TokenSource, error) {
			return ts, nil
		}),
		storagestub.NewInMemoryService(),
	)
}

func TestPostReport_Success(t *testing.T) {
	var receivedBody []byte
	var receivedAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/reports" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != reportContentType {
			t.Errorf("unexpected content-type: %s", ct)
		}
		receivedAuth = r.Header.Get("Authorization")
		var err error
		receivedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	client := &Client{
		httpEndpointURL: u,
		tokenSource:     newValidTestTokenSource(),
	}

	payload := crashreport.Payload{YAML: []byte("package: testpkg\nversion: v1.0.0\n")}

	err := client.PostReport(context.Background(), payload)
	if err != nil {
		t.Fatalf("PostReport: %v", err)
	}

	if string(receivedBody) != string(payload.YAML) {
		t.Errorf("body = %q, want %q", string(receivedBody), string(payload.YAML))
	}
	if receivedAuth != "Bearer test-token" {
		t.Errorf("Authorization = %q, want %q", receivedAuth, "Bearer test-token")
	}
}

func TestPostReport_NonSuccessStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	client := &Client{
		httpEndpointURL: u,
		tokenSource:     newValidTestTokenSource(),
	}

	err := client.PostReport(context.Background(), crashreport.Payload{YAML: []byte("package: test\n")})
	if err == nil {
		t.Fatal("expected error for non-success status")
	}
}
