// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2024 Unstable Build, All Rights Reserved.
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

package openai

import (
	"fmt"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/packages/param"
	"github.com/openai/openai-go/v2/responses"
	"github.com/openai/openai-go/v2/shared"
	"unstable.build/go-tui/cmd/rune-agent/llm"
)

func openAIMessageFromModel(msg llm.Message) (openai.ChatCompletionMessageParamUnion, error) {
	switch msg.Role {
	case llm.RoleSystem:
		return openai.SystemMessage(msg.Content), nil
	case llm.RoleUser:
		if len(msg.MultiContent) > 0 {
			parts := make([]openai.ChatCompletionContentPartUnionParam, len(msg.MultiContent))
			for i, p := range msg.MultiContent {
				switch p.Type {
				case llm.ContentPartTypeImageURL:
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
	case llm.RoleAssistant:
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
	case llm.RoleTool:
		return openai.ToolMessage(msg.Content, msg.ToolCallID), nil
	default:
		return openai.ChatCompletionMessageParamUnion{}, fmt.Errorf("role %q is unrecognized by openai's API", msg.Role)
	}
}

func openAIToolsFromModel(tools []llm.Tool) []openai.ChatCompletionToolUnionParam {
	if len(tools) == 0 {
		return nil
	}
	ret := make([]openai.ChatCompletionToolUnionParam, len(tools))
	for i, tool := range tools {
		def := shared.FunctionDefinitionParam{
			Name:        tool.Function.Name,
			Description: param.NewOpt(tool.Function.Description),
		}
		switch p := tool.Function.Parameters.(type) {
		case shared.FunctionParameters:
			def.Parameters = p
		case map[string]any:
			def.Parameters = shared.FunctionParameters(p)
		}
		ret[i] = openai.ChatCompletionFunctionTool(def)
	}
	return ret
}

func openAIResponseFormatFromModel(format *llm.ResponseFormat) *openai.ChatCompletionNewParamsResponseFormatUnion {
	if format == nil {
		return nil
	}

	ret := new(openai.ChatCompletionNewParamsResponseFormatUnion)
	switch format.Type {
	case llm.ResponseFormatTypeText:
		ret.OfText = &openai.ResponseFormatTextParam{}
	case llm.ResponseFormatTypeJSONObject:
		ret.OfJSONObject = &openai.ResponseFormatJSONObjectParam{}
	case llm.ResponseFormatTypeJSONSchema:
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

// responsesInputFromMessages converts llm.Messages to Responses API input items.
// The Responses API uses a different message format from Chat Completions.
func responsesInputFromMessages(msgs []llm.Message) (responses.ResponseInputParam, string) {
	var items responses.ResponseInputParam
	var instructions string

	for _, msg := range msgs {
		switch msg.Role {
		case llm.RoleSystem:
			// System messages become the instructions parameter.
			if instructions != "" {
				instructions += "\n"
			}
			instructions += msg.Content

		case llm.RoleUser:
			item := responses.ResponseInputItemUnionParam{
				OfMessage: &responses.EasyInputMessageParam{
					Role: responses.EasyInputMessageRoleUser,
				},
			}
			if len(msg.MultiContent) > 0 {
				parts := make(responses.ResponseInputMessageContentListParam, len(msg.MultiContent))
				for i, p := range msg.MultiContent {
					switch p.Type {
					case llm.ContentPartTypeImageURL:
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

		case llm.RoleAssistant:
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

		case llm.RoleTool:
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

// responsesToolsFromModel converts llm.Tool slices to Responses API tool params.
// Tool schemas must already be strict-compliant (additionalProperties: false,
// all properties in required). See agent/agentools/ for the tool definitions.
func responsesToolsFromModel(tools []llm.Tool) []responses.ToolUnionParam {
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
		switch p := tool.Function.Parameters.(type) {
		case shared.FunctionParameters:
			ft.Parameters = map[string]any(p)
		case map[string]any:
			ft.Parameters = p
		}
		ret[i] = responses.ToolUnionParam{OfFunction: ft}
	}
	return ret
}
