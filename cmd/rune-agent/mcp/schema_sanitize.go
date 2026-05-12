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

package mcp

import (
	"log/slog"
	"slices"
	"strconv"
	"strings"
)

// sanitizeMCPInputSchema defensively patches an MCP tool's JSON-schema
// InputSchema so it satisfies OpenAI's strict function-schema validator,
// which rejects any object with `"type": "array"` that lacks an `"items"`
// keyword (error: `invalid_function_parameters: array schema missing
// items`). MCP servers (built-in or user-configured) are free to publish
// schemas that omit `items`, and a single defective tool fails the entire
// turn when forwarded to OpenAI verbatim.
//
// We walk the schema and inject `"items": {}` (the most permissive items
// schema accepted by the validator) for every offending array. A warning
// is logged per patched property to surface the upstream defect.
//
// The MCP SDK declares InputSchema as `any`. On the client side it is
// typically a `map[string]any` (from JSON unmarshal). Other types are
// returned unchanged.
func sanitizeMCPInputSchema(serverName, toolName string, schema any) any {
	m, ok := schema.(map[string]any)
	if !ok {
		return schema
	}
	sanitizeSchemaNode(serverName, toolName, "", m)
	return m
}

// sanitizeSchemaNode walks a single JSON-schema object, injecting a
// permissive `items` schema when an array type lacks one, then recurses
// into the standard schema composition keywords.
func sanitizeSchemaNode(serverName, toolName, path string, node map[string]any) {
	if node == nil {
		return
	}

	if isArrayType(node["type"]) {
		if items, ok := node["items"]; !ok || items == nil {
			node["items"] = map[string]any{}
			slog.Warn("mcp: patched tool schema: array missing items",
				"server", serverName,
				"tool", toolName,
				"path", pathOrRoot(path),
			)
		}
	}

	if items, ok := node["items"]; ok {
		sanitizeSchemaChild(serverName, toolName, joinPath(path, "items"), items)
	}

	if props, ok := node["properties"].(map[string]any); ok {
		for k, v := range props {
			sanitizeSchemaChild(serverName, toolName, joinPath(path, "properties", k), v)
		}
	}

	if ap, ok := node["additionalProperties"]; ok {
		sanitizeSchemaChild(serverName, toolName, joinPath(path, "additionalProperties"), ap)
	}

	for _, key := range []string{"oneOf", "anyOf", "allOf"} {
		if arr, ok := node[key].([]any); ok {
			for i, v := range arr {
				sanitizeSchemaChild(serverName, toolName, joinPathIndex(path, key, i), v)
			}
		}
	}

	for _, key := range []string{"$defs", "definitions"} {
		if defs, ok := node[key].(map[string]any); ok {
			for k, v := range defs {
				sanitizeSchemaChild(serverName, toolName, joinPath(path, key, k), v)
			}
		}
	}
}

// sanitizeSchemaChild recurses into a value that may be a single schema
// object or an array of schema objects.
func sanitizeSchemaChild(serverName, toolName, path string, v any) {
	switch c := v.(type) {
	case map[string]any:
		sanitizeSchemaNode(serverName, toolName, path, c)
	case []any:
		for i, item := range c {
			if m, ok := item.(map[string]any); ok {
				sanitizeSchemaNode(serverName, toolName, joinPathIndex(path, "", i), m)
			}
		}
	}
}

// isArrayType reports whether a JSON-schema `type` value declares an array.
// The keyword may be a string ("array") or a list of strings (e.g.
// ["array", "null"]).
func isArrayType(t any) bool {
	switch v := t.(type) {
	case string:
		return v == "array"
	case []any:
		for _, e := range v {
			if s, ok := e.(string); ok && s == "array" {
				return true
			}
		}
	case []string:
		return slices.Contains(v, "array")
	}
	return false
}

func joinPath(base string, parts ...string) string {
	if len(parts) == 0 {
		return base
	}
	return base + "/" + strings.Join(parts, "/")
}

func joinPathIndex(base, key string, i int) string {
	out := base
	if key != "" {
		out += "/" + key
	}
	return out + "/" + strconv.Itoa(i)
}

func pathOrRoot(p string) string {
	if p == "" {
		return "/"
	}
	return p
}
