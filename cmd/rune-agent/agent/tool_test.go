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

package agent

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryWithFilteredTools(t *testing.T) {
	t.Run("filters to allowed names", func(t *testing.T) {
		r := NewRegistry(
			&mockTool{name: "read_file"},
			&mockTool{name: "search"},
			&mockTool{name: "bash"},
		)

		filtered := r.WithFilteredTools([]string{"read_file", "search"})

		_, ok := filtered.Get("read_file", "")
		assert.True(t, ok, "read_file should be in filtered registry")
		_, ok = filtered.Get("search", "")
		assert.True(t, ok, "search should be in filtered registry")
		_, ok = filtered.Get("bash", "")
		assert.False(t, ok, "bash should not be in filtered registry")
	})

	t.Run("unknown names are silently ignored", func(t *testing.T) {
		r := NewRegistry(
			&mockTool{name: "read_file"},
		)

		filtered := r.WithFilteredTools([]string{"read_file", "nonexistent"})

		assert.Len(t, filtered.AllTools(), 1)
		_, ok := filtered.Get("read_file", "")
		assert.True(t, ok)
	})

	t.Run("empty allowed names returns empty registry", func(t *testing.T) {
		r := NewRegistry(
			&mockTool{name: "read_file"},
		)

		filtered := r.WithFilteredTools(nil)

		assert.Empty(t, filtered.AllTools())
	})

	t.Run("preserves overrides for allowed names", func(t *testing.T) {
		r := NewRegistry(
			&mockTool{name: "search"},
			&mockTool{name: "bash"},
		)
		override := &mockTool{name: "search", result: ToolResult{Content: "openai-search"}}
		r.RegisterOverrides("openai", override)

		filtered := r.WithFilteredTools([]string{"search"})

		got, ok := filtered.Get("search", "openai")
		assert.True(t, ok)
		assert.Equal(t, override, got)
	})

	t.Run("does not propagate exclusions for allowed names", func(t *testing.T) {
		r := NewRegistry(
			&mockTool{name: "exit_plan_mode"},
			&mockTool{name: "bash"},
		)
		r.RegisterExclusions("openai", "exit_plan_mode")

		filtered := r.WithFilteredTools([]string{"exit_plan_mode"})

		// exit_plan_mode is explicitly allowed, so the exclusion should not carry over.
		got, ok := filtered.Get("exit_plan_mode", "openai")
		assert.True(t, ok, "explicitly allowed tool should not be excluded")
		assert.NotNil(t, got)

		// Tools() should include it.
		tools := filtered.Tools("openai")
		var names []string
		for _, tool := range tools {
			names = append(names, tool.Function.Name)
		}
		assert.Contains(t, names, "exit_plan_mode")
	})

	t.Run("drops overrides for excluded names", func(t *testing.T) {
		r := NewRegistry(
			&mockTool{name: "search"},
			&mockTool{name: "bash"},
		)
		r.RegisterOverrides("openai", &mockTool{name: "bash"})

		filtered := r.WithFilteredTools([]string{"search"})

		_, ok := filtered.Get("bash", "openai")
		assert.False(t, ok)
	})
}

func TestRegistryOverrides(t *testing.T) {
	t.Run("override replaces base tool for provider", func(t *testing.T) {
		base := &mockTool{name: "search", result: ToolResult{Content: "base"}}
		override := &mockTool{name: "search", result: ToolResult{Content: "openai"}}
		r := NewRegistry(base)
		r.RegisterOverrides("openai", override)

		got, ok := r.Get("search", "openai")
		require.True(t, ok)
		assert.Equal(t, override, got)

		got, ok = r.Get("search", "")
		require.True(t, ok)
		assert.Equal(t, base, got)

		got, ok = r.Get("search", "anthropic")
		require.True(t, ok)
		assert.Equal(t, base, got)
	})

	t.Run("override adds provider-only tool", func(t *testing.T) {
		r := NewRegistry(&mockTool{name: "read_file"})
		extra := &mockTool{name: "extra_tool"}
		r.RegisterOverrides("openai", extra)

		_, ok := r.Get("extra_tool", "")
		assert.False(t, ok, "extra_tool should not exist for empty provider")

		got, ok := r.Get("extra_tool", "openai")
		assert.True(t, ok)
		assert.Equal(t, extra, got)
	})

	t.Run("exclude removes base tool for provider", func(t *testing.T) {
		r := NewRegistry(
			&mockTool{name: "search"},
			&mockTool{name: "bash"},
		)
		r.RegisterExclusions("gemini", "bash")

		_, ok := r.Get("bash", "")
		assert.True(t, ok, "bash should exist for empty provider")

		_, ok = r.Get("bash", "gemini")
		assert.False(t, ok, "bash should be excluded for gemini")

		_, ok = r.Get("search", "gemini")
		assert.True(t, ok, "search should still exist for gemini")
	})

	t.Run("Tools returns merged definitions for provider", func(t *testing.T) {
		r := NewRegistry(
			&mockTool{name: "a"},
			&mockTool{name: "b"},
		)
		r.RegisterOverrides("openai", &mockTool{name: "c"})
		r.RegisterExclusions("openai", "b")

		tools := r.Tools("openai")
		names := make([]string, len(tools))
		for i, tool := range tools {
			names[i] = tool.Function.Name
		}
		sort.Strings(names)
		assert.Equal(t, []string{"a", "c"}, names)
	})

}

func TestRegistryCopyOverridesFrom(t *testing.T) {
	t.Run("copies overrides and exclusions", func(t *testing.T) {
		src := NewRegistry(&mockTool{name: "search"})
		override := &mockTool{name: "search", result: ToolResult{Content: "openai-search"}}
		src.RegisterOverrides("openai", override)
		src.RegisterExclusions("openai", "bash")

		dst := NewRegistry(
			&mockTool{name: "search"},
			&mockTool{name: "bash"},
		)
		dst.CopyOverridesFrom(src)

		got, ok := dst.Get("search", "openai")
		require.True(t, ok)
		assert.Equal(t, override, got)

		_, ok = dst.Get("bash", "openai")
		assert.False(t, ok, "bash should be excluded for openai")
	})

	t.Run("merges with existing overrides", func(t *testing.T) {
		src := NewRegistry()
		src.RegisterOverrides("openai", &mockTool{name: "b"})

		dst := NewRegistry(&mockTool{name: "a"})
		existing := &mockTool{name: "a", result: ToolResult{Content: "openai-a"}}
		dst.RegisterOverrides("openai", existing)
		dst.CopyOverridesFrom(src)

		got, ok := dst.Get("a", "openai")
		require.True(t, ok)
		assert.Equal(t, existing, got, "existing override should be preserved")

		got, ok = dst.Get("b", "openai")
		require.True(t, ok)
		assert.Equal(t, "b", got.Definition().Function.Name, "copied override should be present")
	})
}
