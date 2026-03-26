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

package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfigServing(t *testing.T) {
	logger := log.New()
	logger.SetOutput(io.Discard)
	logger.SetLevel(log.TraceLevel)
	url, err := url.Parse("http://localhost:3001")
	require.NoError(t, err)

	for _, method := range []string{"POST", "OPTIONS", "PUT", "DELETE"} {
		t.Run(fmt.Sprintf("%s other than GET fails", method), func(t *testing.T) {
			req := httptest.NewRequest("POST", url.JoinPath(ServeConfigPath).String(), nil)

			w := httptest.NewRecorder()
			sut, err := ServeNativeConfig(logger, url)
			require.NoError(t, err)
			sut.ServeHTTP(w, req)

			resp := w.Result()
			require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		})
	}

	t.Run("GET", func(t *testing.T) {
		req := httptest.NewRequest("GET", url.JoinPath(ServeConfigPath).String(), nil)

		w := httptest.NewRecorder()
		sut, err := ServeNativeConfig(logger, url)
		require.NoError(t, err)
		sut.ServeHTTP(w, req)

		resp := w.Result()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var actualCfg Config
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&actualCfg))
		assert.Equal(t, DefaultNativeConfig(url), actualCfg)
	})
}
