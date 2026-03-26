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

package agentools

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/cmd/rune-agent/agent"
)

func TestRequestSkill_Done(t *testing.T) {
	mp := &mockPrompter{
		responses: []agent.PromptResponse{
			{Values: []string{"done"}},
		},
	}
	tool := NewRequestSkill(mp)
	result := tool.Execute(context.Background(), `{
		"skill": "web_search",
		"description": "Search the web for documentation"
	}`)

	require.False(t, result.IsError, result.Content)
	assert.Contains(t, result.Content, "installed")
	assert.Contains(t, result.Content, "web_search")

	require.Len(t, mp.calls, 1)
	assert.Contains(t, mp.calls[0].Title, "web_search")
	assert.Equal(t, "Skill", mp.calls[0].Header)
	assert.Equal(t, "Search the web for documentation", mp.calls[0].Body)
	require.Len(t, mp.calls[0].Options, 2)
	assert.Equal(t, "Done", mp.calls[0].Options[0].Label)
	assert.Equal(t, "Won't do", mp.calls[0].Options[1].Label)
}

func TestRequestSkill_WontDo(t *testing.T) {
	mp := &mockPrompter{
		responses: []agent.PromptResponse{
			{Values: []string{"wont_do"}},
		},
	}
	tool := NewRequestSkill(mp)
	result := tool.Execute(context.Background(), `{
		"skill": "deploy",
		"description": "Deploy to production"
	}`)

	require.False(t, result.IsError, result.Content)
	assert.Contains(t, result.Content, "declined")
	assert.Contains(t, result.Content, "deploy")
}

func TestRequestSkill_PrompterError(t *testing.T) {
	mp := &mockPrompter{err: errors.New("dismissed")}
	tool := NewRequestSkill(mp)
	result := tool.Execute(context.Background(), `{
		"skill": "web_search",
		"description": "Search the web"
	}`)

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "dismissed")
}

func TestRequestSkill_InvalidJSON(t *testing.T) {
	mp := &mockPrompter{}
	tool := NewRequestSkill(mp)
	result := tool.Execute(context.Background(), `bad json`)

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "invalid arguments")
}

func TestRequestSkill_EmptySkillName(t *testing.T) {
	mp := &mockPrompter{}
	tool := NewRequestSkill(mp)
	result := tool.Execute(context.Background(), `{
		"skill": "",
		"description": "Something"
	}`)

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "skill name is required")
}

func TestRequestSkill_Summary(t *testing.T) {
	tool := NewRequestSkill(&mockPrompter{})
	assert.Equal(t, "web_search", tool.Summary(`{"skill":"web_search","description":"search"}`))
	assert.Equal(t, "", tool.Summary(`bad`))
}

func TestRequestSkill_Definition(t *testing.T) {
	tool := NewRequestSkill(&mockPrompter{})
	def := tool.Definition()
	assert.Equal(t, "request_skill", def.Function.Name)
	assert.NotEmpty(t, def.Function.Description)
}
