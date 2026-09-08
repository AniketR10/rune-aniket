// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
