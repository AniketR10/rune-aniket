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


package extension

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"

	"unstable.build/go-tui/cmd/rune-agent/llm/llmtest"
)

// Bare names must not appear in /model completions: two providers
// can ship the same Name (e.g. openai/gpt-5.5 vs codex/gpt-5.5),
// and a bare candidate would silently dispatch to whichever provider
// Models() iterates first.
func TestCommandAdapterModelCompleter(t *testing.T) {
	svc := llmtest.New([]llmapi.ModelEntry{
		{Provider: "openai", Name: "gpt-5.5"},
		{Provider: "codex", Name: "gpt-5.5"},
		{Provider: "openai", Name: "gpt-4o"},
	})
	a := &commandAdapter{llmSvc: svc}
	ctx := context.Background()
	it, err := a.Complete(ctx, "model", nil)
	require.NoError(t, err)
	got, err := iterator.ToSlice(ctx, it)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"openai/gpt-5.5",
		"codex/gpt-5.5",
		"openai/gpt-4o",
	}, got)
}

// Same hazard as TestCommandAdapterModelCompleter, for :chat / :query.
func TestHandlerModelCompleter(t *testing.T) {
	svc := llmtest.New([]llmapi.ModelEntry{
		{Provider: "openai", Name: "gpt-5.5"},
		{Provider: "codex", Name: "gpt-5.5"},
		{Provider: "openai", Name: "gpt-4o"},
	})
	h := &aiEditorHandler{llmSvc: svc}
	ctx := context.Background()
	it, err := h.completeWithModelsIterator(ctx)
	require.NoError(t, err)
	got, err := iterator.ToSlice(ctx, it)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"openai/gpt-5.5",
		"codex/gpt-5.5",
		"openai/gpt-4o",
	}, got)
}
