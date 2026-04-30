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

package codex

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/cmd/rune-agent/llm/llmregistry"
)

func TestRegisterModels(t *testing.T) {
	reg := llmregistry.NewStatic()
	RegisterModels(reg)

	entry, ok := reg.Get(context.Background(), GPT5Dot4)
	require.True(t, ok)
	assert.Equal(t, LLMProvider, entry.Provider)
	assert.Equal(t, OpenAICompatibleURL, entry.BaseURL)
	assert.Equal(t, 1000000, entry.ContextWindow)
}

func TestUpstreamModelName(t *testing.T) {
	assert.Equal(t, "gpt-5.4", UpstreamModelName(GPT5Dot4))
	assert.Equal(t, "unknown", UpstreamModelName("unknown"))
}

func TestUpstreamAvailableModels(t *testing.T) {
	models := UpstreamAvailableModels()
	assert.Equal(t, 272000, models["gpt-5.5"])
	assert.Equal(t, 1000000, models["gpt-5.4"])
	assert.Equal(t, 272000, models["gpt-5.4-mini"])
	assert.Equal(t, 272000, models["gpt-5.3-codex"])
	assert.Equal(t, 272000, models["gpt-5.2"])
	assert.Equal(t, 1000000, models["codex-auto-review"])
}
