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
