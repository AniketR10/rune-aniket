// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package openai

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/packages/param"
	"github.com/openai/openai-go/v2/responses"
	"github.com/openai/openai-go/v2/shared"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
)

// chatToolChoice maps the llmapi ToolChoice literal onto the Chat
// Completions API tool_choice option. Returns nil when the caller did
// not request a specific mode (leave OpenAI's default in place).
func chatToolChoice(tc llmapi.ToolChoice) *openai.ChatCompletionToolChoiceOptionUnionParam {
	if tc == "" {
		return nil
	}
	out := openai.ChatCompletionToolChoiceOptionUnionParam{
		OfAuto: param.NewOpt(string(tc)),
	}
	return &out
}

// responsesToolChoice maps the llmapi ToolChoice literal onto the
// Responses API tool_choice option. Returns nil when the caller did
// not request a specific mode.
func responsesToolChoice(tc llmapi.ToolChoice) *responses.ResponseNewParamsToolChoiceUnion {
	if tc == "" {
		return nil
	}
	out := responses.ResponseNewParamsToolChoiceUnion{
		OfToolChoiceMode: param.NewOpt(responses.ToolChoiceOptions(tc)),
	}
	return &out
}

func openAIMessageFromModel(msg llmapi.Message) (openai.ChatCompletionMessageParamUnion, error) {
	switch msg.Role {
	case llmapi.RoleSystem:
		return openai.SystemMessage(msg.Content), nil
	case llmapi.RoleUser:
		if len(msg.MultiContent) > 0 {
			parts := make([]openai.ChatCompletionContentPartUnionParam, len(msg.MultiContent))
			for i, p := range msg.MultiContent {
				switch p.Type {
				case llmapi.ContentPartTypeImageURL:
					parts[i] = openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
						URL: p.ImageURL,
					})
				default:
					parts[i] = openai.TextContentPart(p.Text)
				}
			}
			return openai.UserMessage(parts), nil
		}
		return openai.UserMessage(msg.Content), nil
	case llmapi.RoleAssistant:
		if len(msg.ToolCalls) > 0 {
			toolCalls := make([]openai.ChatCompletionMessageToolCallUnionParam, len(msg.ToolCalls))
			for i, tc := range msg.ToolCalls {
				toolCalls[i] = openai.ChatCompletionMessageToolCallUnionParam{
					OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
						ID: tc.ID,
						Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					},
				}
			}
			assistant := openai.ChatCompletionAssistantMessageParam{
				ToolCalls: toolCalls,
			}
			if msg.Content != "" {
				assistant.Content.OfString = param.NewOpt(msg.Content)
			}
			if msg.Name != "" {
				assistant.Name = param.NewOpt(msg.Name)
			}
			return openai.ChatCompletionMessageParamUnion{OfAssistant: &assistant}, nil
		}
		m := openai.AssistantMessage(msg.Content)
		if msg.Name != "" {
			m.OfAssistant.Name = param.NewOpt(msg.Name)
		}
		return m, nil
	case llmapi.RoleTool:
		return openai.ToolMessage(msg.Content, msg.ToolCallID), nil
	default:
		return openai.ChatCompletionMessageParamUnion{}, fmt.Errorf("role %q is unrecognized by openai's API", msg.Role)
	}
}

func openAIToolsFromModel(tools []llmapi.Tool) []openai.ChatCompletionToolUnionParam {
	if len(tools) == 0 {
		return nil
	}
	ret := make([]openai.ChatCompletionToolUnionParam, len(tools))
	for i, tool := range tools {
		def := shared.FunctionDefinitionParam{
			Name:        tool.Function.Name,
			Description: param.NewOpt(tool.Function.Description),
		}
		def.Parameters = openAIToolParameters(tool.Function.Name, tool.Function.Parameters)
		ret[i] = openai.ChatCompletionFunctionTool(def)
	}
	return ret
}

func openAIResponseFormatFromModel(format *llmapi.ResponseFormat) *openai.ChatCompletionNewParamsResponseFormatUnion {
	if format == nil {
		return nil
	}

	ret := new(openai.ChatCompletionNewParamsResponseFormatUnion)
	switch format.Type {
	case llmapi.ResponseFormatTypeText:
		ret.OfText = &openai.ResponseFormatTextParam{}
	case llmapi.ResponseFormatTypeJSONObject:
		ret.OfJSONObject = &openai.ResponseFormatJSONObjectParam{}
	case llmapi.ResponseFormatTypeJSONSchema:
		if format.JSONSchema != nil {
			ret.OfJSONSchema = &openai.ResponseFormatJSONSchemaParam{
				JSONSchema: openai.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:        format.JSONSchema.Name,
					Description: param.NewOpt(format.JSONSchema.Description),
					Schema:      format.JSONSchema.Schema,
					Strict:      param.NewOpt(format.JSONSchema.Strict),
				},
			}
		}
	}
	return ret
}

// responsesInputFromMessages converts llmapi.Messages to Responses API input items.
// The Responses API uses a different message format from Chat Completions.
func responsesInputFromMessages(msgs []llmapi.Message) (responses.ResponseInputParam, string) {
	var items responses.ResponseInputParam
	var instructions string

	for _, msg := range msgs {
		switch msg.Role {
		case llmapi.RoleSystem:
			// System messages become the instructions parameter.
			if instructions != "" {
				instructions += "\n"
			}
			instructions += msg.Content

		case llmapi.RoleUser:
			item := responses.ResponseInputItemUnionParam{
				OfMessage: &responses.EasyInputMessageParam{
					Role: responses.EasyInputMessageRoleUser,
				},
			}
			if len(msg.MultiContent) > 0 {
				parts := make(responses.ResponseInputMessageContentListParam, len(msg.MultiContent))
				for i, p := range msg.MultiContent {
					switch p.Type {
					case llmapi.ContentPartTypeImageURL:
						parts[i] = responses.ResponseInputContentUnionParam{
							OfInputImage: &responses.ResponseInputImageParam{
								ImageURL: param.NewOpt(p.ImageURL),
							},
						}
					default:
						parts[i] = responses.ResponseInputContentUnionParam{
							OfInputText: &responses.ResponseInputTextParam{
								Text: p.Text,
							},
						}
					}
				}
				item.OfMessage.Content.OfInputItemContentList = parts
			} else {
				item.OfMessage.Content.OfString = param.NewOpt(msg.Content)
			}
			items = append(items, item)

		case llmapi.RoleAssistant:
			// Prefer the verbatim provider-emitted items: they preserve
			// reasoning items (with encrypted_content), the assistant
			// message, and any function calls in the exact order the model
			// produced them. The Responses API requires this for stateless
			// (store=false / ZDR) continuity, and OpenAI recommends it
			// generally when function calling with reasoning models.
			// See: https://platform.openai.com/docs/guides/reasoning
			// ("Keeping reasoning items in context").
			if len(msg.ProviderItems) > 0 {
				for _, raw := range msg.ProviderItems {
					items = append(items,
						param.Override[responses.ResponseInputItemUnionParam](raw))
				}
				continue
			}
			if len(msg.ToolCalls) > 0 {
				// Assistant message with text content.
				if msg.Content != "" {
					items = append(items, responses.ResponseInputItemUnionParam{
						OfMessage: &responses.EasyInputMessageParam{
							Role:    responses.EasyInputMessageRoleAssistant,
							Content: responses.EasyInputMessageContentUnionParam{OfString: param.NewOpt(msg.Content)},
						},
					})
				}
				// Each tool call becomes a separate function_call input item.
				for _, tc := range msg.ToolCalls {
					items = append(items, responses.ResponseInputItemUnionParam{
						OfFunctionCall: &responses.ResponseFunctionToolCallParam{
							CallID:    tc.ID,
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					})
				}
			} else {
				items = append(items, responses.ResponseInputItemUnionParam{
					OfMessage: &responses.EasyInputMessageParam{
						Role:    responses.EasyInputMessageRoleAssistant,
						Content: responses.EasyInputMessageContentUnionParam{OfString: param.NewOpt(msg.Content)},
					},
				})
			}

		case llmapi.RoleTool:
			items = append(items, responses.ResponseInputItemUnionParam{
				OfFunctionCallOutput: &responses.ResponseInputItemFunctionCallOutputParam{
					CallID: msg.ToolCallID,
					Output: msg.Content,
				},
			})
		}
	}

	return items, instructions
}

// responsesToolsFromModel converts llmapi.Tool slices to Responses API tool params.
// Tool schemas must already be strict-compliant (additionalProperties: false,
// all properties in required). See agent/agentools/ for the tool definitions.
func responsesToolsFromModel(tools []llmapi.Tool) []responses.ToolUnionParam {
	if len(tools) == 0 {
		return nil
	}
	ret := make([]responses.ToolUnionParam, len(tools))
	for i, tool := range tools {
		ft := &responses.FunctionToolParam{
			Name: tool.Function.Name,
		}
		if tool.Function.Description != "" {
			ft.Description = param.NewOpt(tool.Function.Description)
		}
		ft.Parameters = map[string]any(openAIToolParameters(tool.Function.Name, tool.Function.Parameters))
		ret[i] = responses.ToolUnionParam{OfFunction: ft}
	}
	return ret
}

// openAIToolParameters converts the generic llmapi.FunctionDefinition.Parameters
// value into a shared.FunctionParameters map that both the Chat Completions
// and the Responses APIs can consume. The input may already be a
// shared.FunctionParameters / map[string]any, raw JSON bytes
// (json.RawMessage or []byte) when the tool definition crossed the
// llmapi gRPC boundary, or any other value that JSON-marshals to an
// object schema. Returns nil when conversion fails so the caller emits a
// tool with no schema rather than an invalid one.
//
// This mirrors the anthropic client's toolInputSchema in llm/anthropic/api.go
// and exists to fix RUNE-186: after the host-side llmapi.Service cutover,
// rune-agent tool schemas arrive as json.RawMessage on the wire, and the
// prior type switch silently dropped them.
func openAIToolParameters(name string, params any) shared.FunctionParameters {
	switch p := params.(type) {
	case nil:
		return nil
	case shared.FunctionParameters:
		return p
	case map[string]any:
		return shared.FunctionParameters(p)
	case json.RawMessage:
		var m map[string]any
		if err := json.Unmarshal(p, &m); err != nil {
			slog.Warn("openai: failed to unmarshal tool parameters",
				"tool", name, "error", err)
			return nil
		}
		return shared.FunctionParameters(m)
	case []byte:
		var m map[string]any
		if err := json.Unmarshal(p, &m); err != nil {
			slog.Warn("openai: failed to unmarshal tool parameters",
				"tool", name, "error", err)
			return nil
		}
		return shared.FunctionParameters(m)
	default:
		b, err := json.Marshal(p)
		if err != nil {
			slog.Warn("openai: failed to marshal tool parameters",
				"tool", name, "error", err)
			return nil
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			slog.Warn("openai: failed to unmarshal tool parameters",
				"tool", name, "error", err)
			return nil
		}
		return shared.FunctionParameters(m)
	}
}
