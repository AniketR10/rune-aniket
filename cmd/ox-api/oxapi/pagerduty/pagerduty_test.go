// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2025 Unstable Build, All Rights Reserved.
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

package pagerduty

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/cmd/ox-api/oxapi"
)

type fakeSecretStore struct {
	secrets map[string][]byte
	err     error
}

func (s fakeSecretStore) GetSecretMetadata(_ context.Context, _ string) (map[string]string, error) {
	return nil, nil
}

func (s fakeSecretStore) AccessSecret(_ context.Context, id string) ([]byte, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.secrets[id], nil
}

func TestNewPanicsWithNilSecretStore(t *testing.T) {
	assert.Panics(t, func() {
		New(nil)
	})
}

func TestNewPanicsWithEmptyEndpoint(t *testing.T) {
	assert.Panics(t, func() {
		newWithEndpoint(fakeSecretStore{}, "", nil)
	})
}

func TestPager_PageSendsTriggerEvent(t *testing.T) {
	var actual event
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&actual))
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"success"}`))
	}))
	t.Cleanup(server.Close)

	pager := newWithEndpoint(fakeSecretStore{
		secrets: map[string][]byte{
			routingKeySecretID: []byte("routing-key"),
		},
	}, server.URL, server.Client())
	err := pager.Page(context.Background(), oxapi.Page{
		Summary:   "Rune crash report: panic",
		Source:    "gs://rune-reports/reports/2026/04/16/1200_deadbeef.yaml",
		Severity:  oxapi.PageSeverityError,
		Component: "ox-api",
		Group:     "rune",
		Class:     "crash-report",
		DedupKey:  "rune-crash:reports/2026/04/16/1200_deadbeef.yaml",
		Details: map[string]any{
			"fingerprint": "deadbeef",
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "routing-key", actual.RoutingKey)
	assert.Equal(t, "trigger", actual.EventAction)
	assert.Equal(t, "rune-crash:reports/2026/04/16/1200_deadbeef.yaml", actual.DedupKey)
	assert.Equal(t, "Rune crash report: panic", actual.Payload.Summary)
	assert.Equal(t, "gs://rune-reports/reports/2026/04/16/1200_deadbeef.yaml", actual.Payload.Source)
	assert.Equal(t, oxapi.PageSeverityError, actual.Payload.Severity)
	assert.Equal(t, "ox-api", actual.Payload.Component)
	assert.Equal(t, "rune", actual.Payload.Group)
	assert.Equal(t, "crash-report", actual.Payload.Class)
	assert.Equal(t, "deadbeef", actual.Payload.CustomDetails["fingerprint"])
}

func TestPager_PageReturnsStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad routing key", http.StatusBadRequest)
	}))
	t.Cleanup(server.Close)

	pager := newWithEndpoint(fakeSecretStore{
		secrets: map[string][]byte{
			routingKeySecretID: []byte("routing-key"),
		},
	}, server.URL, server.Client())
	err := pager.Page(context.Background(), oxapi.Page{Summary: "panic"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "pagerduty event status 400")
	assert.Contains(t, err.Error(), "bad routing key")
}

func TestPager_PageRequiresRoutingKeySecret(t *testing.T) {
	pager := New(fakeSecretStore{
		secrets: map[string][]byte{
			routingKeySecretID: []byte("  \n"),
		},
	})
	err := pager.Page(context.Background(), oxapi.Page{Summary: "panic"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "routing key secret")
	assert.Contains(t, err.Error(), "empty")
}

func TestPager_PageReturnsSecretStoreError(t *testing.T) {
	pager := New(fakeSecretStore{err: errors.New("secret manager down")})
	err := pager.Page(context.Background(), oxapi.Page{Summary: "panic"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "access pagerduty routing key secret")
	assert.Contains(t, err.Error(), "secret manager down")
}
