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

package llamaserver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
)

// TestService_Acquire_FailsFastOnEarlyExit asserts that a model whose process
// crashes during load surfaces the failure well before the startup timeout,
// via a single error notification that leads with the salient llama.cpp line.
func TestService_Acquire_FailsFastOnEarlyExit(t *testing.T) {
	exec := &fakeExecutor{
		exitBeforeReady: true,
		emitLines: []string{
			"llama_model_loader: loaded meta data",
			"load_tensors: loading model tensors",
			"error loading model: missing tensor 'blk.64.ssm_conv1d.weight'",
		},
	}
	// A deliberately long startup timeout proves the failure surfaces via the
	// early-exit path, not by waiting out the deadline.
	svc, notis := newTestService(t, exec, Config{StartupTimeout: 2 * time.Minute})

	start := time.Now()
	_, _, err := svc.acquire(context.Background(), testModel("qwen"))
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Less(t, elapsed, 10*time.Second,
		"early exit must fail fast, not wait out the startup timeout")
	assert.Contains(t, err.Error(), "failed to load model qwen")
	assert.Contains(t, err.Error(),
		"missing tensor 'blk.64.ssm_conv1d.weight'")

	require.Eventually(t, func() bool {
		return len(notis.errorMessages()) >= 1
	}, 2*time.Second, 10*time.Millisecond, "an error notification is emitted")
	msgs := notis.errorMessages()
	require.Len(t, msgs, 1, "exactly one error notification")
	assert.Contains(t, msgs[0], "failed to load model qwen")
	assert.Contains(t, msgs[0], "missing tensor 'blk.64.ssm_conv1d.weight'")
	assert.NotContains(t, notis.levels(), browserapi.LevelSuccess)
}

// TestService_Acquire_EarlyExit_NoDoubleNotify asserts that only one error
// notification is emitted even though both the early-exit and startup-timeout
// paths could fire.
func TestService_Acquire_EarlyExit_NoDoubleNotify(t *testing.T) {
	exec := &fakeExecutor{
		exitBeforeReady: true,
		emitLines:       []string{"error loading model: bad file"},
	}
	// A very short timeout invites the pollHealth deadline to also fire.
	svc, notis := newTestService(t, exec, Config{StartupTimeout: 50 * time.Millisecond})

	_, _, err := svc.acquire(context.Background(), testModel("m"))
	require.Error(t, err)

	// Give any lingering pollHealth deadline a chance to (wrongly) notify.
	time.Sleep(300 * time.Millisecond)
	assert.Len(t, notis.errorMessages(), 1,
		"exit and timeout paths must not double-notify")
}

func TestSummarizeLoadFailure(t *testing.T) {
	tests := []struct {
		name string
		tail []string
		want string
	}{
		{
			name: "missing tensor is the salient line",
			tail: []string{
				"llama_model_loader: loaded meta data",
				"error loading model: missing tensor 'blk.64.ssm_conv1d.weight'",
				"llama_load_model_from_file: failed to load model",
			},
			want: "error loading model: missing tensor 'blk.64.ssm_conv1d.weight'",
		},
		{
			name: "unknown architecture",
			tail: []string{
				"llama_model_loader: loaded meta data",
				"error loading model: unknown model architecture: 'qwen35'",
			},
			want: "error loading model: unknown model architecture: 'qwen35'",
		},
		{
			name: "generic failure falls back to matching marker",
			tail: []string{
				"system info: BLAS = 1",
				"failed to load model",
			},
			want: "failed to load model",
		},
		{
			name: "no marker falls back to last non-empty line",
			tail: []string{"system info: BLAS = 1", "some other line", ""},
			want: "some other line",
		},
		{
			name: "empty tail yields generic message",
			tail: nil,
			want: "server exited before ready",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, summarizeLoadFailure(tt.tail))
		})
	}
}
