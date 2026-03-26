// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2026 Unstable Build, All Rights Reserved.
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

package anthropic

import (
	"encoding/json"
	"strings"

	ant "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"unstable.build/go-tui/cmd/rune-agent/llm"
)

// anthropicParamsFromRequest converts an llm.Request into Anthropic API parameters.
// System messages are extracted into the separate System field, and the remaining
// messages are converted with strict user/assistant alternation.
func anthropicParamsFromRequest(request llm.Request, config Config) ant.MessageNewParams {
	system, messages := convertMessages(request.Messages)

	tools := anthropicToolsFromModel(request.Tools)

	params := ant.MessageNewParams{
		Model:    ant.Model(config.Model),
		Messages: messages,
		System:   system,
		Tools:    tools,
	}

	switch {
	case config.MaxTokens > 0:
		params.MaxTokens = int64(config.MaxTokens)
	case config.EnableThinking:
		// Adaptive thinking shares the max_tokens budget between thinking
		// and text output. With high/max effort Opus 4.6 can easily
		// exhaust a small budget on thinking alone, leaving no tokens for
		// visible content. Use a generous default so the model has room
		// for both.
		params.MaxTokens = 32768
	default:
		params.MaxTokens = 8192
	}

	if config.Temperature != 0 {
		params.Temperature = param.NewOpt(config.Temperature)
	}
	if config.TopP != 0 {
		params.TopP = param.NewOpt(config.TopP)
	}

	// Adaptive thinking: Claude decides dynamically when and how much to think.
	// budget_tokens is deprecated on 4.6 models; adaptive is the recommended mode.
	if config.EnableThinking {
		adaptive := ant.NewThinkingConfigAdaptiveParam()
		params.Thinking = ant.ThinkingConfigParamUnion{OfAdaptive: &adaptive}
	}

	// Output effort — already normalized by client.CreateCompletion.
	if effort := string(request.ReasoningEffort); effort != "" {
		params.OutputConfig.Effort = ant.OutputConfigEffort(effort)
	}

	// Structured output: map ResponseFormat to Anthropic's JSON schema format.
	format := config.ResponseFormat
	if request.ResponseFormat != nil {
		format = request.ResponseFormat
	}
	if format != nil && format.Type == llm.ResponseFormatTypeJSONSchema && format.JSONSchema != nil {
		schema, err := format.JSONSchema.Schema.MarshalJSON()
		if err == nil {
			var m map[string]any
			if json.Unmarshal(schema, &m) == nil {
				params.OutputConfig.Format = ant.JSONOutputFormatParam{
					Schema: m,
				}
			}
		}
	}

	return params
}

// convertMessages separates system messages and converts the rest into Anthropic
// message params. Tool result messages are grouped into user messages and
// consecutive same-role messages are merged.
func convertMessages(msgs []llm.Message) ([]ant.TextBlockParam, []ant.MessageParam) {
	var system []ant.TextBlockParam
	var raw []ant.MessageParam

	for _, msg := range msgs {
		switch msg.Role {
		case llm.RoleSystem:
			system = append(system, ant.TextBlockParam{Text: msg.Content})

		case llm.RoleUser:
			blocks := userContentBlocks(msg)
			raw = append(raw, ant.NewUserMessage(blocks...))

		case llm.RoleAssistant:
			blocks := assistantContentBlocks(msg)
			raw = append(raw, ant.NewAssistantMessage(blocks...))

		case llm.RoleTool:
			// Tool results become user-role messages with ToolResultBlock content.
			raw = append(raw, ant.NewUserMessage(
				ant.NewToolResultBlock(msg.ToolCallID, msg.Content, false),
			))
		}
	}

	// Merge consecutive same-role messages for strict alternation.
	merged := mergeConsecutiveRoles(raw)
	return system, merged
}

// userContentBlocks converts an llm.Message with role=user into Anthropic content blocks.
func userContentBlocks(msg llm.Message) []ant.ContentBlockParamUnion {
	if len(msg.MultiContent) > 0 {
		blocks := make([]ant.ContentBlockParamUnion, 0, len(msg.MultiContent))
		for _, p := range msg.MultiContent {
			switch p.Type {
			case llm.ContentPartTypeImageURL:
				blocks = append(blocks, imageBlockFromURL(p.ImageURL))
			default:
				blocks = append(blocks, ant.NewTextBlock(p.Text))
			}
		}
		return blocks
	}
	return []ant.ContentBlockParamUnion{ant.NewTextBlock(msg.Content)}
}

// imageBlockFromURL creates an image content block from a data URL or regular URL.
func imageBlockFromURL(url string) ant.ContentBlockParamUnion {
	// Handle data URIs: data:image/png;base64,<data>
	if strings.HasPrefix(url, "data:") {
		mediaType, data := parseDataURI(url)
		return ant.ContentBlockParamUnion{
			OfImage: &ant.ImageBlockParam{
				Source: ant.ImageBlockParamSourceUnion{
					OfBase64: &ant.Base64ImageSourceParam{
						MediaType: ant.Base64ImageSourceMediaType(mediaType),
						Data:      data,
					},
				},
			},
		}
	}
	return ant.ContentBlockParamUnion{
		OfImage: &ant.ImageBlockParam{
			Source: ant.ImageBlockParamSourceUnion{
				OfURL: &ant.URLImageSourceParam{URL: url},
			},
		},
	}
}

// parseDataURI extracts media type and base64 data from a data URI.
func parseDataURI(uri string) (mediaType, data string) {
	// Format: data:<mediatype>;base64,<data>
	uri = strings.TrimPrefix(uri, "data:")
	parts := strings.SplitN(uri, ";base64,", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "image/png", uri
}

// assistantContentBlocks converts an llm.Message with role=assistant into content blocks.
func assistantContentBlocks(msg llm.Message) []ant.ContentBlockParamUnion {
	var blocks []ant.ContentBlockParamUnion
	if msg.Content != "" {
		blocks = append(blocks, ant.NewTextBlock(msg.Content))
	}
	for _, tc := range msg.ToolCalls {
		// Pass arguments as json.RawMessage to avoid an Unmarshal→Marshal
		// round-trip. The SDK serialises the input field directly, so raw
		// JSON bytes are forwarded as-is.
		var input any = json.RawMessage("{}")
		if tc.Function.Arguments != "" {
			input = json.RawMessage(tc.Function.Arguments)
		}
		blocks = append(blocks, ant.NewToolUseBlock(tc.ID, input, tc.Function.Name))
	}
	return blocks
}

// mergeConsecutiveRoles merges consecutive messages with the same role by
// concatenating their content blocks. Anthropic requires strict user/assistant
// alternation.
func mergeConsecutiveRoles(msgs []ant.MessageParam) []ant.MessageParam {
	if len(msgs) <= 1 {
		return msgs
	}
	var merged []ant.MessageParam
	for _, msg := range msgs {
		if len(merged) > 0 && merged[len(merged)-1].Role == msg.Role {
			merged[len(merged)-1].Content = append(merged[len(merged)-1].Content, msg.Content...)
		} else {
			merged = append(merged, msg)
		}
	}
	return merged
}

// anthropicToolsFromModel converts llm.Tool slices to Anthropic tool params.
func anthropicToolsFromModel(tools []llm.Tool) []ant.ToolUnionParam {
	if len(tools) == 0 {
		return nil
	}
	ret := make([]ant.ToolUnionParam, len(tools))
	for i, tool := range tools {
		tp := &ant.ToolParam{
			Name: tool.Function.Name,
		}
		if tool.Function.Description != "" {
			tp.Description = param.NewOpt(tool.Function.Description)
		}
		tp.InputSchema = toolInputSchema(tool.Function.Parameters)
		ret[i] = ant.ToolUnionParam{OfTool: tp}
	}
	return ret
}

// toolInputSchema converts the generic Parameters value to an Anthropic
// ToolInputSchemaParam. The input may be raw JSON bytes, a map, or a struct
// that marshals to a JSON schema object.
func toolInputSchema(params any) ant.ToolInputSchemaParam {
	schema := ant.ToolInputSchemaParam{}
	switch p := params.(type) {
	case map[string]any:
		if props, ok := p["properties"]; ok {
			schema.Properties = props
		}
		if req, ok := p["required"].([]any); ok {
			for _, r := range req {
				if s, ok := r.(string); ok {
					schema.Required = append(schema.Required, s)
				}
			}
		}
	case json.RawMessage:
		var m map[string]any
		if json.Unmarshal(p, &m) == nil {
			return toolInputSchema(m)
		}
	case []byte:
		var m map[string]any
		if json.Unmarshal(p, &m) == nil {
			return toolInputSchema(m)
		}
	default:
		if p != nil {
			b, err := json.Marshal(p)
			if err == nil {
				var m map[string]any
				if json.Unmarshal(b, &m) == nil {
					return toolInputSchema(m)
				}
			}
		}
	}
	return schema
}

// applyCacheBreakpoints places explicit cache_control markers on the last
// system text block, the last tool definition, and the conversation prefix
// boundary (second-to-last message). Explicit breakpoints give precise
// control over what is cached and consistently outperform top-level
// auto-caching for multi-turn agent conversations.
//
// The cacheControl value selects the TTL: "5m" (default), "1h",
// "ephemeral" (API default), or "" to disable caching.
func applyCacheBreakpoints(params *ant.MessageNewParams, cacheControl string) {
	cc := ant.NewCacheControlEphemeralParam()
	switch cacheControl {
	case "1h":
		cc.TTL = ant.CacheControlEphemeralTTLTTL1h
	case "ephemeral":
		// Leave TTL unset
	default:
		cc.TTL = ant.CacheControlEphemeralTTLTTL5m
	}

	// Mark the last system text block — caches the entire system prompt
	// (including resource context) as a single prefix.
	if len(params.System) > 0 {
		params.System[len(params.System)-1].CacheControl = cc
	}

	// Mark the last tool definition — caches all tool schemas.
	if len(params.Tools) > 0 {
		last := &params.Tools[len(params.Tools)-1]
		if last.OfTool != nil {
			last.OfTool.CacheControl = cc
		}
	}

	// Mark the second-to-last message — caches the conversation history
	// prefix. Each new turn appends to the end, so everything before the
	// final message is stable and benefits from caching.
	if len(params.Messages) >= 2 {
		turn := &params.Messages[len(params.Messages)-2]
		if n := len(turn.Content); n > 0 {
			setCacheControlOnBlock(&turn.Content[n-1], cc)
		}
	}
}

// setCacheControlOnBlock sets cache_control on the underlying block of a
// ContentBlockParamUnion. Only the block types that appear in agent
// conversations are handled (text, tool_use, tool_result).
func setCacheControlOnBlock(block *ant.ContentBlockParamUnion, cc ant.CacheControlEphemeralParam) {
	switch {
	case block.OfText != nil:
		block.OfText.CacheControl = cc
	case block.OfToolUse != nil:
		block.OfToolUse.CacheControl = cc
	case block.OfToolResult != nil:
		block.OfToolResult.CacheControl = cc
	}
}
