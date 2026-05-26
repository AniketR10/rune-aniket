// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2024 Unstable Build, All Rights Reserved.
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
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/ox-api/auth"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"golang.org/x/oauth2"
	"unstable.build/go-tui/ide/ideplan"
)

func TestNewDoesNotStartTelemetryWhenDisabled(t *testing.T) {
	requests := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	config := DefaultConfig()
	config.HTTPEndpointAddress = srv.URL

	client, err := New(nil, storagestub.NewInMemoryService(), config, t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}()

	select {
	case <-requests:
		t.Fatal("telemetry request sent with disabled telemetry")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestNewStartsTelemetryWhenEnabled(t *testing.T) {
	requests := make(chan string, 8)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	config := DefaultConfig()
	config.HTTPEndpointAddress = srv.URL
	config.EnableTelemetry = true

	client, err := New(nil, storagestub.NewInMemoryService(), config, t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}()

	timer := time.NewTimer(time.Second)
	defer timer.Stop()

	for {
		select {
		case path := <-requests:
			if path == telemetryPath {
				return
			}
		case <-timer.C:
			t.Fatal("expected telemetry request when telemetry is enabled")
		}
	}
}

func TestNewPanicsOnZeroPeriodWithTelemetryEnabled(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for zero TelemetryPeriod with EnableTelemetry=true")
		}
	}()
	config := DefaultConfig()
	config.EnableTelemetry = true
	config.TelemetryPeriod = 0
	config.HTTPEndpointAddress = "http://localhost"
	_, _ = New(nil, storagestub.NewInMemoryService(), config, t.TempDir())
}

// fakeOAuthServer responds with a minimal /config endpoint and a 400
// invalid_grant on /token so the test cannot accidentally complete a flow.
func fakeOAuthServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	return srv
}

type nopNotifier struct{}

func (nopNotifier) Notify(browserapi.NotificationLevel, string, ...any) (string, error) {
	return "", nil
}
func (nopNotifier) NotifyOnce(browserapi.NotificationLevel, string, ...any) (string, error) {
	return "", nil
}
func (nopNotifier) UpdateNotificationProgress(string, string, int64, int64) error {
	return nil
}

func TestClient_TokenSource_NoImplicitBrowserFlow(t *testing.T) {
	srv := fakeOAuthServer(t)
	defer srv.Close()

	config := DefaultConfig()
	config.HTTPEndpointAddress = srv.URL

	client, err := New(nopNotifier{}, storagestub.NewInMemoryService(), config, t.TempDir())
	require.NoError(t, err)
	defer client.Close()

	var browserCalls atomic.Int32
	client.openBrowser = func(*url.URL) error {
		browserCalls.Add(1)
		return nil
	}

	_, err = client.OAuthTokenSource().Token()
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrNotAuthenticated),
		"expected ErrNotAuthenticated, got %v", err)
	assert.Equal(t, int32(0), browserCalls.Load(),
		"browser must not be opened when :login was not invoked")
}

func TestClient_Login_AttemptsBrowserFlow(t *testing.T) {
	srv := fakeOAuthServer(t)
	defer srv.Close()

	config := DefaultConfig()
	config.HTTPEndpointAddress = srv.URL

	client, err := New(nopNotifier{}, storagestub.NewInMemoryService(), config, t.TempDir())
	require.NoError(t, err)
	defer client.Close()

	browserCalls := make(chan struct{}, 1)
	client.openBrowser = func(*url.URL) error {
		select {
		case browserCalls <- struct{}{}:
		default:
		}
		// Return an error to short-circuit the underlying oauth2 flow so
		// the test does not hang waiting for the redirect.
		return errors.New("test: browser not actually opened")
	}

	require.NoError(t, client.Login(t.Context()))

	select {
	case <-browserCalls:
	case <-time.After(5 * time.Second):
		t.Fatal("expected browser to be opened by :login flow")
	}
}

func TestDecidePlan(t *testing.T) {
	now := time.Now()
	grace := 7 * 24 * time.Hour
	withRole := func(role auth.Role, expiry time.Time) *oauth2.Token {
		t := &oauth2.Token{AccessToken: "tok", Expiry: expiry}
		return t.WithExtra(map[string]any{"extra": map[string]any{"Role": role}})
	}

	cases := []struct {
		name string
		tok  *oauth2.Token
		want ideplan.Kind
	}{
		{"nil token denied", nil, ideplan.Denied},
		{"empty access token denied", &oauth2.Token{}, ideplan.Denied},
		{"basic role denied", withRole(auth.RoleBasic, now.Add(time.Hour)), ideplan.Denied},
		{"paid role allowed", withRole(auth.RolePaid, now.Add(time.Hour)), ideplan.Allowed},
		{"user role allowed", withRole(auth.RoleUser, now.Add(time.Hour)), ideplan.Allowed},
		{"zero expiry treated as valid", withRole(auth.RolePaid, time.Time{}), ideplan.Allowed},
		{"expired within grace", withRole(auth.RolePaid, now.Add(-24*time.Hour)), ideplan.GracePeriod},
		{"expired exactly at grace boundary", withRole(auth.RolePaid, now.Add(-grace)), ideplan.GracePeriod},
		{"expired past grace denied", withRole(auth.RolePaid, now.Add(-grace-time.Minute)), ideplan.Denied},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := decidePlan(tc.tok, now, grace)
			assert.Equal(t, tc.want, got.Kind, "decision=%+v", got)
			if tc.want == ideplan.GracePeriod {
				assert.False(t, got.GraceExpiresAt.IsZero())
			}
		})
	}
}

func TestRolePlanFromToken(t *testing.T) {
	t.Run("missing extra returns basic", func(t *testing.T) {
		assert.Equal(t, auth.RoleBasic, rolePlanFromToken(&oauth2.Token{}))
	})
	t.Run("present role returned", func(t *testing.T) {
		tok := (&oauth2.Token{}).WithExtra(map[string]any{
			"extra": map[string]any{"Role": auth.RolePaid},
		})
		assert.Equal(t, auth.RolePaid, rolePlanFromToken(tok))
	})
}
