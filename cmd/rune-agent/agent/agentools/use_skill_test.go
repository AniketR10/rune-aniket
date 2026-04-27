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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/agent/skills"
	"unstable.build/go-tui/cmd/rune-agent/llm"
)

type osFileSystem struct{}

func (osFileSystem) URI(path string) (workspaceapi.URI, error) {
	return workspaceapi.CurrentUserHostURI(path)
}
func (osFileSystem) OpenFile(path string, flag int, mode os.FileMode) (workspaceapi.File, error) {
	return os.OpenFile(path, flag, mode)
}
func (osFileSystem) Remove(path string) error                   { return os.Remove(path) }
func (osFileSystem) Stat(path string) (os.FileInfo, error)      { return os.Stat(path) }
func (osFileSystem) ReadDir(name string) ([]os.DirEntry, error) { return os.ReadDir(name) }
func (osFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func writeTestSkill(t *testing.T, dir, name, content string) {
	t.Helper()
	skillDir := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(skillDir, "SKILL.md"),
		[]byte(content), 0o644,
	))
}

func TestSkillTool(t *testing.T) {
	dir := t.TempDir()
	writeTestSkill(t, dir, "debug", `---
name: debug
description: Debug issues
---
## Debug

1. Check logs
2. Fix`)
	writeTestSkill(t, dir, "test-repo", `---
name: test-repo
description: Test strategy
---
Run the test suite`)

	registry := skills.NewRegistry(osFileSystem{}, dirURI(""), []string{dir}, nil)

	t.Run("valid name returns structured content", func(t *testing.T) {
		tool := NewSkillTool(registry, nil, nil)
		result := tool.Execute(
			context.Background(), `{"name":"debug"}`,
		)
		assert.False(t, result.IsError)
		skillDir := filepath.Join(dir, "debug")
		expected := fmt.Sprintf(`<skill_content name="debug">
## Debug

1. Check logs
2. Fix

Skill directory: %s
Relative paths in this skill are relative to the skill directory.
</skill_content>`, skillDir)
		assert.Equal(t, expected, result.Content)
	})

	t.Run("unknown name returns error with available names", func(t *testing.T) {
		tool := NewSkillTool(registry, nil, nil)
		result := tool.Execute(
			context.Background(), `{"name":"nonexistent"}`,
		)
		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "unknown skill")
		assert.Contains(t, result.Content, "debug")
		assert.Contains(t, result.Content, "test-repo")
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		tool := NewSkillTool(registry, nil, nil)
		result := tool.Execute(
			context.Background(), `bad json`,
		)
		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "invalid arguments")
	})

	t.Run("definition has correct schema", func(t *testing.T) {
		tool := NewSkillTool(registry, nil, nil)
		def := tool.Definition()

		assert.Equal(t, llm.ToolTypeFunction, def.Type)
		assert.Equal(t, "skill", def.Function.Name)
		assert.NotEmpty(t, def.Function.Description)
		assert.NotNil(t, def.Function.Parameters)
	})

	t.Run("summary returns skill name", func(t *testing.T) {
		tool := NewSkillTool(registry, nil, nil)
		assert.Equal(t, "debug", tool.Summary(`{"name":"debug"}`))
		assert.Equal(t, "", tool.Summary(`bad json`))
	})

	t.Run("summary includes args when present", func(t *testing.T) {
		tool := NewSkillTool(registry, nil, nil)
		assert.Equal(t, "debug --verbose",
			tool.Summary(`{"name":"debug","args":"--verbose"}`))
	})

	t.Run("duplicate activation re-injects full skill body", func(t *testing.T) {
		tool := NewSkillTool(registry, nil, nil)
		ctx := context.Background()

		first := tool.Execute(ctx, `{"name":"debug"}`)
		assert.False(t, first.IsError)
		assert.Contains(t, first.Content, "<skill_content")

		second := tool.Execute(ctx, `{"name":"debug"}`)
		assert.False(t, second.IsError)
		// Re-injecting the full body is a deliberate trade-off: a
		// "see the block above" hint is unreliable because the model
		// frequently fails to locate the transient system message
		// (especially for slash-command preloads) and gives up.
		// Returning the full body costs a few tokens but guarantees
		// the model has the instructions it needs to proceed.
		assert.Contains(t, second.Content, `<skill_content name="debug">`)
		assert.Contains(t, second.Content, "</skill_content>")
		assert.Contains(t, second.Content, "Check logs")
	})

	t.Run("definition description references actual injected tag", func(t *testing.T) {
		tool := NewSkillTool(registry, nil, nil)
		desc := tool.Definition().Function.Description
		// The preloaded skill content is injected with a
		// <skill_content> tag (see skills.FormatSkillContent). The tool
		// description must reference that real tag — not a fictional
		// <skill-name> tag — so the model recognises a preloaded skill
		// and follows its instructions directly.
		assert.Contains(t, desc, "<skill_content")
		assert.NotContains(t, desc, "<skill-name>")
	})

	t.Run("different skills are not deduped", func(t *testing.T) {
		tool := NewSkillTool(registry, nil, nil)
		ctx := context.Background()

		first := tool.Execute(ctx, `{"name":"debug"}`)
		assert.Contains(t, first.Content, "<skill_content")

		second := tool.Execute(ctx, `{"name":"test-repo"}`)
		assert.Contains(t, second.Content, "<skill_content")
	})

}

func TestSkillToolWithResources(t *testing.T) {
	dir := t.TempDir()
	writeTestSkill(t, dir, "my-skill", `---
name: my-skill
description: A skill with resources
---
Do stuff`)

	// Create resource files in scripts/ and assets/ subdirs.
	skillDir := filepath.Join(dir, "my-skill")
	require.NoError(t, os.MkdirAll(filepath.Join(skillDir, "scripts"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(skillDir, "assets"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(skillDir, "references"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "scripts", "run.sh"), []byte("#!/bin/sh"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "assets", "data.json"), []byte("{}"), 0o644))
	// Subdirectories inside resource dirs should be skipped.
	require.NoError(t, os.MkdirAll(filepath.Join(skillDir, "scripts", "subdir"), 0o755))

	registry := skills.NewRegistry(osFileSystem{}, dirURI(""), []string{dir}, nil)
	tool := NewSkillTool(registry, nil, nil)
	ctx := context.Background()
	result := tool.Execute(ctx, `{"name":"my-skill"}`)

	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, `<skill_content name="my-skill">`)
	assert.Contains(t, result.Content, "<skill_resources>")
	assert.Contains(t, result.Content, fmt.Sprintf("<file>%s</file>", filepath.Join("assets", "data.json")))
	assert.Contains(t, result.Content, fmt.Sprintf("<file>%s</file>", filepath.Join("scripts", "run.sh")))
	assert.NotContains(t, result.Content, "subdir")
}

func TestSkillToolWithoutResources(t *testing.T) {
	dir := t.TempDir()
	writeTestSkill(t, dir, "bare", `---
name: bare
description: No resources
---
Just instructions`)

	registry := skills.NewRegistry(osFileSystem{}, dirURI(""), []string{dir}, nil)
	tool := NewSkillTool(registry, nil, nil)
	ctx := context.Background()
	result := tool.Execute(ctx, `{"name":"bare"}`)

	assert.False(t, result.IsError)
	assert.NotContains(t, result.Content, "<skill_resources>")
	assert.Contains(t, result.Content, "</skill_content>")
}

func TestSkillToolAgentType(t *testing.T) {
	dir := t.TempDir()
	writeTestSkill(t, dir, "research", `---
name: research
description: Research agent
type: agent
allowed-tools: read_file search_content
---
You are a researcher.`)
	writeTestSkill(t, dir, "prompt", `---
name: prompt
description: A prompt skill
---
Prompt content`)

	registry := skills.NewRegistry(osFileSystem{}, dirURI(""), []string{dir}, nil)

	t.Run("agent skill delegates to spawner", func(t *testing.T) {
		spawner := &mockSpawner{
			handle: agent.RunHandle{
				Events: newMockEventIterator(
					agent.Event{Type: agent.EventText, Text: "research findings here"},
				),
			},
		}
		childEvents := make(chan agent.ChildEvent, 64)
		tool := NewSkillTool(registry, spawner, childEvents)
		ctx := agent.WithParentToolCallID(context.Background(), "parent-skill")
		result := tool.Execute(
			ctx,
			`{"name":"research","args":"find all tests"}`,
		)

		assert.False(t, result.IsError)
		assert.Equal(t, "research findings here", result.Content)
		assert.Equal(t, "research", spawner.lastRunReq.Label)
		assert.Equal(t, "find all tests", spawner.lastRunReq.Message)
		assert.Equal(t, []string{"read_file", "search_content"}, spawner.lastRunReq.AllowedTools)
		assert.Equal(t, "You are a researcher.", spawner.lastRunReq.SystemPrompt)
	})

	t.Run("agent skill uses body as task when no args", func(t *testing.T) {
		spawner := &mockSpawner{
			handle: agent.RunHandle{
				Events: newMockEventIterator(
					agent.Event{Type: agent.EventText, Text: "done"},
				),
			},
		}
		childEvents := make(chan agent.ChildEvent, 64)
		tool := NewSkillTool(registry, spawner, childEvents)
		ctx := agent.WithParentToolCallID(context.Background(), "parent-skill-noargs")
		result := tool.Execute(
			ctx,
			`{"name":"research"}`,
		)

		assert.False(t, result.IsError)
		assert.Equal(t, "You are a researcher.", spawner.lastRunReq.Message)
	})

	t.Run("agent skill returns error on spawner failure", func(t *testing.T) {
		spawner := &mockSpawner{
			runErr: fmt.Errorf("service unavailable"),
		}
		tool := NewSkillTool(registry, spawner, nil)
		result := tool.Execute(
			context.Background(),
			`{"name":"research","args":"test"}`,
		)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "service unavailable")
	})

	t.Run("agent skill returns error when no spawner", func(t *testing.T) {
		tool := NewSkillTool(registry, nil, nil)
		result := tool.Execute(
			context.Background(),
			`{"name":"research","args":"test"}`,
		)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "no spawner")
	})

	t.Run("prompt skill ignores spawner", func(t *testing.T) {
		spawner := &mockSpawner{}
		tool := NewSkillTool(registry, spawner, nil)
		ctx := context.Background()
		result := tool.Execute(ctx, `{"name":"prompt"}`)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "<skill_content")
		assert.Contains(t, result.Content, "Prompt content")
		// Spawner should not have been called.
		assert.Empty(t, spawner.lastRunReq.Message)
	})

	t.Run("agent skill is not deduplicated", func(t *testing.T) {
		spawner := &mockSpawner{
			handle: agent.RunHandle{
				Events: newMockEventIterator(
					agent.Event{Type: agent.EventText, Text: "result"},
				),
			},
		}
		childEvents := make(chan agent.ChildEvent, 64)
		tool := NewSkillTool(registry, spawner, childEvents)
		ctx := agent.WithParentToolCallID(
			context.Background(), "parent-dedup")

		first := tool.Execute(ctx, `{"name":"research","args":"first"}`)
		assert.False(t, first.IsError)
		assert.Equal(t, "result", first.Content)

		// Reset events for second call.
		spawner.handle = agent.RunHandle{
			Events: newMockEventIterator(
				agent.Event{Type: agent.EventText, Text: "result"},
			),
		}

		// Agent skills should run each time, not be deduped.
		second := tool.Execute(ctx, `{"name":"research","args":"second"}`)
		assert.False(t, second.IsError)
		assert.Equal(t, "result", second.Content)
		assert.Equal(t, "second", spawner.lastRunReq.Message)
	})
}

var _ agent.Tool = (*skillTool)(nil)
