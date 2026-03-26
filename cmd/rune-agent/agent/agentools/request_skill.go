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
	"encoding/json"
	"fmt"

	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/llm"
)

type requestSkillTool struct {
	prompter agent.Prompter
}

type requestSkillArgs struct {
	Skill       string `json:"skill"`
	Description string `json:"description"`
}

// NewRequestSkill creates a request_skill tool backed by the given Prompter.
// The model calls this when it needs a capability that is not currently
// available as a tool or skill. The user is prompted to install the
// skill out of band and confirm, or decline.
func NewRequestSkill(prompter agent.Prompter) agent.Tool {
	return &requestSkillTool{prompter: prompter}
}

func (t *requestSkillTool) Definition() llm.Tool {
	return llm.Tool{
		Type: llm.ToolTypeFunction,
		Function: llm.FunctionDefinition{
			Name: "request_skill",
			Description: `Request a skill or capability that is not currently available.
Use this when you need a tool that is not in your tool list (e.g.
web_search, a language-specific linter, a deployment tool). The user
is shown what you need and can install the skill out of band, then
confirm. If the user confirms, newly installed skills will be available
on the next turn.

Do NOT use this for skills that are already available — use the skill
tool instead.`,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"skill": map[string]any{
						"type":        "string",
						"description": "Short name for the skill being requested (e.g. \"web_search\", \"deploy\").",
					},
					"description": map[string]any{
						"type":        "string",
						"description": "What the skill should do and why it is needed for the current task.",
					},
				},
				"required":             []string{"skill", "description"},
				"additionalProperties": false,
			},
		},
	}
}

func (t *requestSkillTool) Summary(arguments string) string {
	var args requestSkillArgs
	if json.Unmarshal([]byte(arguments), &args) != nil {
		return ""
	}
	return args.Skill
}

func (t *requestSkillTool) Execute(ctx context.Context, arguments string) agent.ToolResult {
	var args requestSkillArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return agent.ToolResult{
			Content: fmt.Sprintf("invalid arguments: %v", err),
			IsError: true,
		}
	}
	if args.Skill == "" {
		return agent.ToolResult{
			Content: "invalid arguments: skill name is required",
			IsError: true,
		}
	}

	resp, err := t.prompter.Prompt(ctx, agent.PromptRequest{
		Title:  fmt.Sprintf("Skill %q is not available", args.Skill),
		Header: "Skill",
		Body:   args.Description,
		Options: []agent.PromptOption{
			{Label: "Done", Description: "I have installed the skill", Value: "done"},
			{Label: "Won't do", Description: "Skip this — continue without it", Value: "wont_do"},
		},
	})
	if err != nil {
		return agent.ToolResult{
			Content: fmt.Sprintf("prompt dismissed: %v", err),
			IsError: true,
		}
	}

	if len(resp.Values) > 0 && resp.Values[0] == "done" {
		return agent.ToolResult{
			Content: fmt.Sprintf(
				"The user has installed the %q skill. "+
					"It should now be available — check the skills list and proceed.",
				args.Skill,
			),
		}
	}

	return agent.ToolResult{
		Content: fmt.Sprintf(
			"The user declined to install the %q skill. "+
				"Continue without it or find an alternative approach.",
			args.Skill,
		),
	}
}
