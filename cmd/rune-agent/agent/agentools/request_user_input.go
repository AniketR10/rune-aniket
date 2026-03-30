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
	"unstable.build/go-tui/cmd/rune-agent/llm"
)

type requestUserInputTool struct {
	prompter agent.Prompter
}

type requestUserInputArgs struct {
	Questions []requestUserInputQuestion `json:"questions"`
}

type requestUserInputQuestion struct {
	ID       string                   `json:"id"`
	Header   string                   `json:"header"`
	Question string                   `json:"question"`
	Options  []requestUserInputOption `json:"options"`
}

type requestUserInputOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

// NewRequestUserInput creates a request_user_input tool backed by the given
// Prompter. This is the OpenAI-flavored variant of ask_user_question, using
// the schema that OpenAI models are trained with.
func NewRequestUserInput(prompter agent.Prompter) agent.Tool {
	return &requestUserInputTool{prompter: prompter}
}

func (t *requestUserInputTool) Definition() llm.Tool {
	return llm.Tool{
		Type: llm.ToolTypeFunction,
		Function: llm.FunctionDefinition{
			Name:        "request_user_input",
			Description: "Request user input for one to three short questions and wait for the response. When options is empty, the user types a free-form text answer. If the user needs a custom answer, the client automatically adds an Other option and collects free-form text before returning.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"questions": map[string]any{
						"type":        "array",
						"description": "Questions to show the user. Prefer 1 and do not exceed 3",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"id": map[string]any{
									"type":        "string",
									"description": "Stable identifier for mapping answers (snake_case).",
								},
								"header": map[string]any{
									"type":        "string",
									"description": "Short header label shown in the UI (12 or fewer chars).",
								},
								"question": map[string]any{
									"type":        "string",
									"description": "Single-sentence prompt shown to the user.",
								},
								"options": map[string]any{
									"type":        "array",
									"description": "Provide 0-3 mutually exclusive choices. Pass an empty array for free-form text input. Put the recommended option first and suffix its label with \"(Recommended)\". Do not include an \"Other\" option in this list; the client will add an \"Other\" option automatically and collect typed context before returning.",
									"items": map[string]any{
										"type": "object",
										"properties": map[string]any{
											"label": map[string]any{
												"type":        "string",
												"description": "User-facing label (1-5 words).",
											},
											"description": map[string]any{
												"type":        "string",
												"description": "One short sentence explaining impact/tradeoff if selected.",
											},
										},
										"required":             []string{"label", "description"},
										"additionalProperties": false,
									},
								},
							},
							"required":             []string{"id", "header", "question", "options"},
							"additionalProperties": false,
						},
					},
				},
				"required":             []string{"questions"},
				"additionalProperties": false,
			},
		},
	}
}

func (t *requestUserInputTool) Summary(arguments string) string {
	var args requestUserInputArgs
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

func (t *requestUserInputTool) Execute(ctx context.Context, arguments string) agent.ToolResult {
	var args requestUserInputArgs
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
		ID       string   `json:"id"`
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
				ID:       q.ID,
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
				Label:         otherOptionLabel,
				Description:   "Type a custom answer",
				Value:         otherOptionLabel,
				RequiresInput: true,
			}

			resp, err := t.prompter.Prompt(ctx, agent.PromptRequest{
				Title:   q.Question,
				Header:  q.Header,
				Options: opts,
			})
			if err != nil {
				return agent.ToolResult{
					Content: fmt.Sprintf("prompt dismissed: %v", err),
					IsError: true,
				}
			}

			selected, err := selectedAnswersForPrompt(ctx, t.prompter, q, resp)
			if err != nil {
				return agent.ToolResult{
					Content: fmt.Sprintf("custom answer required: %v", err),
					IsError: true,
				}
			}

			answers = append(answers, answer{
				ID:       q.ID,
				Selected: selected,
			})
		}
	}

	// Build result matching codex response format:
	// {"answers": {"id": {"answers": ["selected1", ...]}}}
	type answerValue struct {
		Answers []string `json:"answers"`
	}
	type result struct {
		Answers map[string]answerValue `json:"answers"`
	}
	res := result{Answers: make(map[string]answerValue, len(answers))}
	for _, a := range answers {
		res.Answers[a.ID] = answerValue{Answers: a.Selected}
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

func selectedAnswersForPrompt(
	ctx context.Context,
	prompter agent.Prompter,
	q requestUserInputQuestion,
	resp agent.PromptResponse,
) ([]string, error) {
	if len(resp.Values) != 1 || resp.Values[0] != otherOptionLabel {
		return resp.Values, nil
	}
	if text := strings.TrimSpace(resp.TextInput); text != "" {
		return []string{text}, nil
	}

	// Fallback for prompters that don't yet surface RequiresInput text inline:
	// immediately collect the custom answer before returning to the model.
	followUp, err := prompter.Prompt(ctx, agent.PromptRequest{
		Title:  q.Question,
		Header: q.Header,
	})
	if err != nil {
		return nil, err
	}
	if text := strings.TrimSpace(followUp.TextInput); text != "" {
		return []string{text}, nil
	}
	return nil, fmt.Errorf("selected %q but did not provide custom text", otherOptionLabel)
}
