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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
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
