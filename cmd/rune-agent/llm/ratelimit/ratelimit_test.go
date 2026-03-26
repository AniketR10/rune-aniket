// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
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

package ratelimit

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsRetryableStreamError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{
			name:      "nil error",
			err:       nil,
			retryable: false,
		},
		{
			name:      "overloaded_error from Anthropic SSE",
			err:       fmt.Errorf(`received error while streaming: {"type":"error","error":{"details":null,"type":"overloaded_error","message":"Overloaded"},"request_id":"req_test123"}`),
			retryable: true,
		},
		{
			name:      "api_error from SSE",
			err:       fmt.Errorf(`received error while streaming: {"type":"error","error":{"type":"api_error","message":"Internal server error"}}`),
			retryable: true,
		},
		{
			name:      "rate_limit_error from SSE",
			err:       fmt.Errorf(`received error while streaming: {"type":"error","error":{"type":"rate_limit_error","message":"Rate limited"}}`),
			retryable: true,
		},
		{
			name:      "authentication_error is not retryable",
			err:       fmt.Errorf(`received error while streaming: {"type":"error","error":{"type":"authentication_error","message":"Invalid API key"}}`),
			retryable: false,
		},
		{
			name:      "invalid_request_error is not retryable",
			err:       fmt.Errorf(`received error while streaming: {"type":"error","error":{"type":"invalid_request_error","message":"Bad request"}}`),
			retryable: false,
		},
		{
			name:      "permission_error is not retryable",
			err:       fmt.Errorf(`received error while streaming: {"type":"error","error":{"type":"permission_error","message":"Forbidden"}}`),
			retryable: false,
		},
		{
			name:      "not_found_error is not retryable",
			err:       fmt.Errorf(`received error while streaming: {"type":"error","error":{"type":"not_found_error","message":"Not found"}}`),
			retryable: false,
		},
		{
			name:      "unrelated error is not retryable",
			err:       fmt.Errorf("invalid JSON response"),
			retryable: false,
		},
		{
			name:      "network error is not a stream error",
			err:       fmt.Errorf("connection reset by peer"),
			retryable: false,
		},
		{
			name:      "HTTP error with rate_limit_error in body is not a stream error",
			err:       fmt.Errorf(`429 Too Many Requests {"type":"rate_limit_error","message":"rate limited"}`),
			retryable: false,
		},
		{
			name:      "HTTP error with api_error in body is not a stream error",
			err:       fmt.Errorf(`500 Internal Server Error {"type":"api_error","message":"internal error"}`),
			retryable: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.retryable, IsRetryableStreamError(tt.err))
		})
	}
}
