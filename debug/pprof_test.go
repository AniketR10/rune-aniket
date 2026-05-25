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

package debug

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStartPProfHTTPRandomPortServesHeap binds the pprof server to a
// random localhost port and verifies that the heap endpoint responds
// to a real HTTP request. This is the contract relied on by both the
// SIGUSR1 trigger and the in-IDE :pprof debug command.
func TestStartPProfHTTPRandomPortServesHeap(t *testing.T) {
	addr, err := StartPProfHTTP("127.0.0.1:0")
	require.NoError(t, err)
	require.NotEmpty(t, addr)

	url := "http://" + addr + "/debug/pprof/heap?debug=1"
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	// Heap profile in debug=1 format starts with "heap profile:".
	assert.True(t, strings.HasPrefix(string(body), "heap profile:"),
		"pprof heap response should start with heap profile prefix: %q",
		string(body[:min(64, len(body))]))
}

// TestStartPProfHTTPBindError verifies that an invalid bind address
// surfaces a real error rather than panicking or silently logging.
func TestStartPProfHTTPBindError(t *testing.T) {
	_, err := StartPProfHTTP("not-a-valid-addr")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pprof listen")
}
