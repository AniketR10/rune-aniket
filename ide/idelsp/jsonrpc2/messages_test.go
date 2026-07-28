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

package jsonrpc2

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEncodeResponseAlwaysHasResultOrError locks in the JSON-RPC 2.0
// invariant that a successful response carries a "result" member even
// when the value is null. A nil or empty Result must not drop the
// member; servers such as ty reject a reply with neither "result" nor
// "error" and then stall any request that depends on it (observed with
// workspace/configuration blocking workspace/diagnostic).
func TestEncodeResponseAlwaysHasResultOrError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		result json.RawMessage
		err    error
	}{
		{"nil result", nil, nil},
		{"empty result", json.RawMessage{}, nil},
		{"explicit null result", json.RawMessage("null"), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data, err := EncodeMessage(&Response{
				ID:     Int64ID(1),
				Result: tt.result,
				Error:  tt.err,
			})
			require.NoError(t, err)

			var wire map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(data, &wire))
			_, hasResult := wire["result"]
			_, hasError := wire["error"]
			assert.True(t, hasResult || hasError,
				"response must contain result or error: %s", data)
			assert.True(t, hasResult,
				"successful response must contain a result member: %s", data)
			assert.Equal(t, json.RawMessage("null"), wire["result"])
		})
	}
}

// TestEncodeResponseErrorOmitsResult verifies that when an error is
// present the result member is not synthesized, so an error response
// stays well-formed.
func TestEncodeResponseErrorOmitsResult(t *testing.T) {
	t.Parallel()

	data, err := EncodeMessage(&Response{
		ID:    Int64ID(1),
		Error: ErrInternal,
	})
	require.NoError(t, err)

	var wire map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &wire))
	_, hasResult := wire["result"]
	_, hasError := wire["error"]
	assert.False(t, hasResult, "error response must not carry a result: %s", data)
	assert.True(t, hasError, "error response must carry an error: %s", data)
}
