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
	"strings"

	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/agent/skills"
	"unstable.build/go-tui/cmd/rune-agent/llm"
)

// NewSkillTool creates a skill tool backed by a skill registry.
// The tool always sees the latest set of skills, even after runtime
// add-dir/remove-dir operations. When spawner is non-nil, agent-type
// skills are executed by spawning a sub-agent via the spawner.
// childEvents receives sub-agent events tagged with the parent tool
// call ID for TUI rendering.
func NewSkillTool(
	registry *skills.SkillRegistry, spawner agent.Spawner,
	childEvents chan<- agent.ChildEvent,
) agent.Tool {
	return &skillTool{registry: registry, spawner: spawner, childEvents: childEvents}
}

type skillTool struct {
	registry    *skills.SkillRegistry
	spawner     agent.Spawner
	childEvents chan<- agent.ChildEvent
}

type skillArgs struct {
	Name string `json:"name"`
	Args string `json:"args,omitempty"`
}

func (t *skillTool) Definition() llm.Tool {
	return llm.Tool{
		Type: llm.ToolTypeFunction,
		Function: llm.FunctionDefinition{
			Name: "skill",
			Description: `Load and execute a skill by name. When the user's request matches a
skill listed in the system-reminder, call this tool BEFORE generating
any other response. Do not mention a skill without calling this tool.
If a <skill_content> block is already present in the current turn,
the skill has been loaded — follow its instructions directly without
calling this tool again.
When users reference a slash command or /<something> (e.g. "/review",
"/commit"), they are referring to a skill. Use this tool to invoke it.`,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "The skill name to load",
					},
					"args": map[string]any{
						"type":        []string{"string", "null"},
						"description": "Optional arguments for the skill",
					},
				},
				"required":             []string{"name", "args"},
				"additionalProperties": false,
			},
		},
	}
}

func (t *skillTool) Summary(arguments string) string {
	var args skillArgs
	if json.Unmarshal([]byte(arguments), &args) != nil {
		return ""
	}
	if args.Args != "" {
		return args.Name + " " + args.Args
	}
	return args.Name
}

func (t *skillTool) Execute(
	ctx context.Context, arguments string,
) agent.ToolResult {
	var args skillArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return agent.ToolResult{
			Content: fmt.Sprintf("invalid arguments: %v", err),
			IsError: true,
		}
	}

	skill, ok := t.registry.Get(args.Name)
	if !ok {
		all := t.registry.List()
		names := make([]string, 0, len(all))
		for _, s := range all {
			names = append(names, s.Name)
		}
		return agent.ToolResult{
			Content: fmt.Sprintf(
				"unknown skill %q. Available skills: %s",
				args.Name, strings.Join(names, ", "),
			),
			IsError: true,
		}
	}

	// Agent-type skills: spawn a sub-agent instead of injecting inline.
	if skill.Type == "agent" {
		return t.executeAgentSkill(ctx, skill, args.Args)
	}

	// Dedup: don't re-inject the full body if this skill was already
	// loaded in this run. Still point the model at the existing
	// <skill_content> block so it follows the instructions instead of
	// giving up.
	if agent.IsSkillActivated(ctx, args.Name) {
		return agent.ToolResult{
			Content: fmt.Sprintf(
				"Skill %q is already loaded in this conversation. "+
					"Follow the instructions in the existing "+
					"<skill_content name=%q> block above instead of "+
					"calling this tool again.",
				args.Name, args.Name,
			),
		}
	}
	agent.MarkSkillActivated(ctx, args.Name)

	return agent.ToolResult{Content: skills.FormatSkillContent(skill)}
}

// executeAgentSkill spawns a sub-agent with the skill's body as
// system prompt and its allowed-tools as the tool filter. It blocks
// until the sub-agent completes and returns its output.
func (t *skillTool) executeAgentSkill(
	ctx context.Context, skill skills.Skill, userArgs string,
) agent.ToolResult {
	if t.spawner == nil {
		return agent.ToolResult{
			Content: fmt.Sprintf("skill %q is an agent skill but no spawner is available", skill.Name),
			IsError: true,
		}
	}

	task := userArgs
	if task == "" {
		task = skill.Body
	}

	var allowedTools []string
	if skill.AllowedTools != "" {
		allowedTools = strings.Fields(skill.AllowedTools)
	}

	handle, err := t.spawner.Run(ctx, agent.RunRequest{
		Label:        skill.Name,
		Model:        agent.CurrentModel(ctx),
		Message:      task,
		AllowedTools: allowedTools,
		SystemPrompt: skill.Body,
	})
	if err != nil {
		return agent.ToolResult{
			Content: fmt.Sprintf("agent skill %q error: %v", skill.Name, err),
			IsError: true,
		}
	}

	return consumeSubAgent(ctx, handle.Events, t.childEvents)
}
