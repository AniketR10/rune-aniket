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
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

// argValue returns the value following flag in args, or "" if absent.
func argValue(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func TestConfig_ServerArgs_CoreFlags(t *testing.T) {
	cfg := Config{
		ContextWindow:   4096,
		BatchSize:       512,
		NGPULayers:      -1,
		Threads:         8,
		FlashAttention:  true,
		MaxOutputTokens: 256,
		ChatTemplate:    "chatml",
		NCacheReuse:     128,
		ExtraArgs:       []string{"--verbose"},
	}
	args := cfg.serverArgs("/m.gguf", "/proj.gguf", 0, "127.0.0.1", 8080)

	assert.Equal(t, "/m.gguf", argValue(args, "--model"))
	assert.Equal(t, "/proj.gguf", argValue(args, "--mmproj"))
	assert.Equal(t, "127.0.0.1", argValue(args, "--host"))
	assert.Equal(t, "8080", argValue(args, "--port"))
	assert.Equal(t, "4096", argValue(args, "--ctx-size"))
	assert.Equal(t, "512", argValue(args, "--batch-size"))
	assert.Equal(t, "999", argValue(args, "--n-gpu-layers"), "negative offloads all layers")
	assert.Equal(t, "8", argValue(args, "--threads"))
	assert.True(t, slices.Contains(args, "--flash-attn"))
	assert.Equal(t, "256", argValue(args, "--predict"))
	assert.Equal(t, "chatml", argValue(args, "--chat-template"))
	assert.Equal(t, "128", argValue(args, "--cache-reuse"))
	assert.Equal(t, "--verbose", args[len(args)-1], "extra args appended last")
}

func TestConfig_ServerArgs_PerModelContextOverridesConfig(t *testing.T) {
	cfg := Config{ContextWindow: 4096}
	args := cfg.serverArgs("/m.gguf", "", 8192, "127.0.0.1", 1)
	assert.Equal(t, "8192", argValue(args, "--ctx-size"),
		"registry per-model context window wins over config default")
}

func TestConfig_ServerArgs_OmitsUnsetKnobs(t *testing.T) {
	cfg := Config{}
	args := cfg.serverArgs("/m.gguf", "", 0, "127.0.0.1", 1)
	assert.Empty(t, argValue(args, "--mmproj"))
	assert.Empty(t, argValue(args, "--ctx-size"))
	assert.Empty(t, argValue(args, "--batch-size"))
	assert.Empty(t, argValue(args, "--n-gpu-layers"), "zero gpu layers omits the flag")
	assert.False(t, slices.Contains(args, "--flash-attn"))
}

func TestConfig_ServerArgs_SamplingFlags(t *testing.T) {
	cfg := Config{Sampling: SamplerParams{
		Temperature:     0.7,
		TopK:            40,
		TopP:            0.9,
		MinP:            0.05,
		RepeatPenalty:   1.1,
		RepeatLastN:     64,
		FreqPenalty:     0.2,
		PresencePenalty: 0.3,
	}}
	args := cfg.serverArgs("/m.gguf", "", 0, "127.0.0.1", 1)
	assert.Equal(t, "0.7", argValue(args, "--temp"))
	assert.Equal(t, "40", argValue(args, "--top-k"))
	assert.Equal(t, "0.9", argValue(args, "--top-p"))
	assert.Equal(t, "0.05", argValue(args, "--min-p"))
	assert.Equal(t, "1.1", argValue(args, "--repeat-penalty"))
	assert.Equal(t, "64", argValue(args, "--repeat-last-n"))
	assert.Equal(t, "0.2", argValue(args, "--frequency-penalty"))
	assert.Equal(t, "0.3", argValue(args, "--presence-penalty"))
}

func TestConfig_WithDefaults(t *testing.T) {
	got := Config{}.withDefaults()
	assert.Equal(t, defaultIdleTimeout, got.IdleTimeout)
	assert.Equal(t, defaultMaxServers, got.MaxServers)
	assert.Equal(t, defaultStartupTimeout, got.StartupTimeout)
	assert.Equal(t, defaultHost, got.Host)
}

func TestParseLoadProgress(t *testing.T) {
	for _, tc := range []struct {
		line string
		want int
		ok   bool
	}{
		{"load_tensors: loading model tensors 42%", 42, true},
		{"llama_model_loader: 100%", 100, true},
		{"some line with 7.5% done", 7, true},
		{"no percentage here", 0, false},
		{"trailing percent sign only %", 0, false},
	} {
		got, ok := parseLoadProgress(tc.line)
		assert.Equal(t, tc.ok, ok, tc.line)
		if tc.ok {
			assert.Equal(t, tc.want, got, tc.line)
		}
	}
}
