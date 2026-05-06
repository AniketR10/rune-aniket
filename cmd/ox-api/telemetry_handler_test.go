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

package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	blueauth "github.com/unstablebuild/blue/auth"
	"unstable.build/go-tui/cmd/ox-api/oxapi"
	"unstable.build/go-tui/cmd/ox-api/auth"
)

const symmetricKey = "12345678901234567890123456789012"

func TestTelemetryHandler(t *testing.T) {
	testSignKey, err := blueauth.SymmetricKey([]byte(symmetricKey))
	require.NoError(t, err)
	testSignKeys := blueauth.StaticSymmetricKeys(testSignKey)
	validToken, err := blueauth.SignToken(testSignKey, "1234", "1234", auth.RPCUser{Role: auth.RoleAdmin}, 1*time.Hour)
	require.NoError(t, err)

	suite := []struct {
		description        string
		requestBody        string
		authHeader         string
		expectedStatusCode int
		expectedLog        string
	}{
		{"non JSON payload is a 400", "{hello:world]", "Bearer " + validToken, 400, ""},
		{"non JSON payload is a 400 without authentication", "{hello:world]", "", 400, ""},
		{"valid JSON payload with authentication", `{"a": 1}`, "Bearer " + validToken, 200,
			`{"a": 1, "Auth":true, "Auth.Role":"admin", "Auth.Subject":"1234", "Auth.UserID":"1234",` +
				`"callType":"telemetry", "class":"telemetryHandler","level":"info","msg":""}`},
		{"valid JSON payload without authentication", `{"a": 1}`, "", 200,
			`{"a": 1, "Auth":false, "callType":"telemetry", "class":"telemetryHandler","level":"info","msg":""}`},
	}

	for _, test := range suite {
		t.Run(test.description, func(t *testing.T) {
			var buf bytes.Buffer
			logger := log.New()
			logger.SetFormatter(&log.JSONFormatter{})
			logger.SetOutput(&buf)
			logger.SetLevel(log.InfoLevel)

			req := httptest.NewRequest("POST", "http://localhost:3001/telemetry",
				strings.NewReader(test.requestBody))
			req.Header.Add("Content-Type", "application/json")

			if test.authHeader != "" {
				req.Header.Set("Authorization", test.authHeader)
			}

			w := httptest.NewRecorder()
			sut := oxapi.NewTelemetryHandler(logger, testSignKeys)
			sut.ServeHTTP(w, req)

			resp := w.Result()
			require.Equal(t, test.expectedStatusCode, resp.StatusCode)

			var actual, expected map[string]interface{}
			if test.expectedLog == "" {
				assert.Zero(t, buf.String())
			} else {
				require.NoError(t, json.NewDecoder(bytes.NewReader(buf.Bytes())).Decode(&actual))
				require.NoError(t, json.NewDecoder(strings.NewReader(test.expectedLog)).Decode(&expected), test.expectedLog)
			}

			// delete fields with dynamic values
			delete(actual, "time")
			delete(actual, "Auth.IssuedAt")
			delete(actual, "Auth.Expiry")
			delete(actual, "Auth.ID")
			assert.Equal(t, expected, actual)
		})
	}
}
