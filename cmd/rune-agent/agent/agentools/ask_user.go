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
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
)

type askUserTool struct {
	prompter agent.Prompter
}

type askUserArgs struct {
	Questions []askQuestion `json:"questions"`
}

type askQuestion struct {
	Question    string      `json:"question"`
	Header      string      `json:"header"`
	Options     []askOption `json:"options"`
	MultiSelect bool        `json:"multiSelect"`
}

type askOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

const otherOptionLabel = "Other"

// NewAskUser creates an ask_user_question tool backed by the given Prompter.
func NewAskUser(prompter agent.Prompter) agent.Tool {
	return &askUserTool{prompter: prompter}
}

func (t *askUserTool) NeedsDeterministicOrder() bool { return false }

func (t *askUserTool) Definition() llmapi.Tool {
	return llmapi.Tool{
		Type: llmapi.ToolTypeFunction,
		Function: llmapi.FunctionDefinition{
			Name: "ask_user_question",
			Description: `Ask the user one or more multiple-choice questions. Use this tool when you
need to gather preferences, clarify ambiguity, or offer choices before proceeding.

When options are provided, each question shows an inline selection UI that the
user navigates with arrow keys and confirms with Enter.
An "Other" option is automatically appended so the user always has an escape hatch. If the user selects "Other", you should follow
up by asking them to clarify in natural language.

When options is an empty array, the question is shown in the chat and the user
types a free-form text answer using the standard input box.

Guidelines:
- Use 0-4 options per question and 1-4 questions per call.
- Use an empty options array when the answer is open-ended (e.g. a file path,
  a name, or any value that cannot be enumerated).
- Keep option labels short (1-3 words).
- Place the recommended option first and suffix its label with "(Recommended)".
- Provide a description for each option when helpful.
- Set multiSelect to true only when the user should pick more than one.
- The header is a short label (max 12 chars) shown as a chip next to the title.
- Do NOT use this tool just to confirm proceeding — only to gather actual choices.`,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"questions": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"question": map[string]any{
									"type":        "string",
									"description": "The question to display as the title.",
								},
								"header": map[string]any{
									"type":        []string{"string", "null"},
									"description": "Short chip label (max 12 chars).",
								},
								"options": map[string]any{
									"type": "array",
									"items": map[string]any{
										"type": "object",
										"properties": map[string]any{
											"label": map[string]any{
												"type":        "string",
												"description": "Short label for the option.",
											},
											"description": map[string]any{
												"type":        []string{"string", "null"},
												"description": "Optional longer description.",
											},
										},
										"required":             []string{"label", "description"},
										"additionalProperties": false,
									},
									"maxItems":    4,
									"description": "The options to present. Pass an empty array for free-form text input.",
								},
								"multiSelect": map[string]any{
									"type":        []string{"boolean", "null"},
									"description": "Allow multiple selections.",
								},
							},
							"required":             []string{"question", "header", "options", "multiSelect"},
							"additionalProperties": false,
						},
						"minItems":    1,
						"maxItems":    4,
						"description": "The questions to ask.",
					},
				},
				"required":             []string{"questions"},
				"additionalProperties": false,
			},
		},
	}
}

func (t *askUserTool) Summary(arguments string) string {
	var args askUserArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return ""
	}
	if len(args.Questions) == 0 {
		return ""
	}
	q := args.Questions[0].Question
	if len(q) > 60 {
		return q[:60] + "..."
	}
	return q
}

func (t *askUserTool) Execute(ctx context.Context, arguments string) agent.ToolResult {
	var args askUserArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return agent.ToolResult{
			Content: fmt.Sprintf("invalid arguments: %v", err),
			IsError: true,
		}
	}
	if len(args.Questions) == 0 {
		return agent.ToolResult{
			Content: "invalid arguments: at least one question is required",
			IsError: true,
		}
	}

	type answer struct {
		Question string   `json:"question"`
		Selected []string `json:"selected"`
	}
	var answers []answer

	for _, q := range args.Questions {
		if len(q.Options) == 0 {
			// Free-form text input: no options, no "Other".
			resp, err := t.prompter.Prompt(ctx, agent.PromptRequest{
				Title:  q.Question,
				Header: q.Header,
			})
			if err != nil {
				return agent.ToolResult{
					Content: fmt.Sprintf("prompt dismissed: %v", err),
					IsError: true,
				}
			}
			answers = append(answers, answer{
				Question: q.Question,
				Selected: []string{resp.TextInput},
			})
		} else {
			opts := make([]agent.PromptOption, len(q.Options)+1)
			for i, o := range q.Options {
				opts[i] = agent.PromptOption{
					Label:       o.Label,
					Description: o.Description,
					Value:       o.Label,
				}
			}
			opts[len(q.Options)] = agent.PromptOption{
				Label:       otherOptionLabel,
				Description: "None of the above",
				Value:       otherOptionLabel,
			}

			resp, err := t.prompter.Prompt(ctx, agent.PromptRequest{
				Title:       q.Question,
				Header:      q.Header,
				Options:     opts,
				MultiSelect: q.MultiSelect,
			})
			if err != nil {
				return agent.ToolResult{
					Content: fmt.Sprintf("prompt dismissed: %v", err),
					IsError: true,
				}
			}

			answers = append(answers, answer{
				Question: q.Question,
				Selected: resp.Values,
			})
		}
	}

	// Build result
	type result struct {
		Answers map[string]string `json:"answers"`
	}
	res := result{Answers: make(map[string]string, len(answers))}
	for _, a := range answers {
		res.Answers[a.Question] = strings.Join(a.Selected, ", ")
	}
	data, err := json.Marshal(res)
	if err != nil {
		return agent.ToolResult{
			Content: fmt.Sprintf("marshal result: %v", err),
			IsError: true,
		}
	}
	return agent.ToolResult{Content: string(data)}
}
