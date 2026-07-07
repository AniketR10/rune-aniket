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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"golang.org/x/oauth2"
)

// TestCloseFlushesFinalUsage verifies that closing telemetry posts the
// counts accumulated since the last periodic flush, so the final window
// of usage is not lost on shutdown.
func TestCloseFlushesFinalUsage(t *testing.T) {
	usage := make(chan telemetryUsagePayload, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p telemetryUsagePayload
		_ = json.NewDecoder(r.Body).Decode(&p)
		if p.Type == "ClientUsage" {
			usage <- p
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	// A long period keeps the periodic flush from firing so the only
	// ClientUsage post we observe is the one Close emits.
	tel := newTelemetry(oauth2.StaticTokenSource(&oauth2.Token{}), u, time.Hour, "test")

	tel.Handle(context.Background(), textapi.Event{Type: textapi.EventTypeEdit})
	tel.Handle(context.Background(), textapi.Event{Type: textapi.EventTypeEdit})
	tel.Handle(context.Background(), textapi.Event{Type: textapi.EventTypeOpen})

	if err := tel.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	select {
	case p := <-usage:
		if p.Edited != 2 || p.Opened != 1 {
			t.Fatalf("final usage = %+v, want Edited=2 Opened=1", p)
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not flush a final ClientUsage event")
	}
}

// TestCloseFlushesWithNoEvents verifies Close still posts a final usage
// event when no editor events were recorded (e.g. the user opened no
// files), so a session is not dropped just because its counters are zero.
func TestCloseFlushesWithNoEvents(t *testing.T) {
	usage := make(chan telemetryUsagePayload, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p telemetryUsagePayload
		_ = json.NewDecoder(r.Body).Decode(&p)
		if p.Type == "ClientUsage" {
			usage <- p
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	tel := newTelemetry(oauth2.StaticTokenSource(&oauth2.Token{}), u, time.Hour, "test")

	if err := tel.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	select {
	case p := <-usage:
		if p.Opened != 0 || p.Closed != 0 || p.Flushed != 0 || p.Edited != 0 {
			t.Fatalf("final usage = %+v, want all zero counters", p)
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not flush a final ClientUsage event with no prior events")
	}
}

// TestCloseFlushBoundedWhenOffline verifies Close returns within a small
// multiple of the flush timeout even when the server never responds, so
// an offline user is not blocked on shutdown.
func TestCloseFlushBoundedWhenOffline(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	defer srv.Close()
	defer close(block)

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	tel := newTelemetry(oauth2.StaticTokenSource(&oauth2.Token{}), u, time.Hour, "test")
	tel.Handle(context.Background(), textapi.Event{Type: textapi.EventTypeEdit})

	done := make(chan error, 1)
	go func() { done <- tel.Close() }()

	select {
	case <-done:
	case <-time.After(flushTimeout + 2*time.Second):
		t.Fatalf("Close blocked longer than flushTimeout %v when offline", flushTimeout)
	}
}
