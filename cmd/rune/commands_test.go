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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"unstable.build/go-tui/cmd/rune/ide/apiclient"
)

// runLoginCommand must return without waiting for the OAuth flow to
// complete. The :login command runs on the Ebiten UI goroutine, so a
// synchronous wait freezes the IDE until the user finishes (or
// abandons) the browser flow.
func TestRunLoginCommandReturnsImmediately(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	config := apiclient.DefaultConfig()
	config.HTTPEndpointAddress = srv.URL
	client := apiclient.New(storagestub.NewInMemoryService(), config, t.TempDir())
	defer client.Close()

	returned := make(chan error, 1)
	go func() { returned <- runLoginCommand(t.Context(), client, nopNotifier{}) }()

	select {
	case err := <-returned:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("runLoginCommand did not return; UI goroutine would deadlock")
	}
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
