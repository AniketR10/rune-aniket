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

package claudememory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguemanager"
	"unstable.build/go-tui/cmd/rune-agent/llm"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
)

func TestParseConversation(t *testing.T) {
	t.Run("basic user and assistant messages", func(t *testing.T) {
		jsonl := lines(
			`{"type":"user","message":{"id":"u1","role":"user","content":"Hello"}}`,
			`{"type":"assistant","message":{"id":"a1","role":"assistant","content":[{"type":"text","text":"Hi there!"}]}}`,
		)
		msgs, err := parseConversation(strings.NewReader(jsonl))
		require.NoError(t, err)
		require.Len(t, msgs, 2)
		assert.Equal(t, llm.RoleUser, msgs[0].Role)
		assert.Equal(t, "Hello", msgs[0].Content)
		assert.Equal(t, llm.RoleAssistant, msgs[1].Role)
		assert.Equal(t, "Hi there!", msgs[1].Content)
	})

	t.Run("sidechain entries are skipped", func(t *testing.T) {
		jsonl := lines(
			`{"type":"user","message":{"id":"u1","role":"user","content":"Hello"}}`,
			`{"type":"assistant","isSidechain":true,"message":{"id":"a-side","role":"assistant","content":[{"type":"text","text":"sidechain response"}]}}`,
			`{"type":"assistant","message":{"id":"a1","role":"assistant","content":[{"type":"text","text":"main response"}]}}`,
		)
		msgs, err := parseConversation(strings.NewReader(jsonl))
		require.NoError(t, err)
		require.Len(t, msgs, 2)
		assert.Equal(t, "main response", msgs[1].Content)
	})

	t.Run("non-conversational types are skipped", func(t *testing.T) {
		jsonl := lines(
			`{"type":"system","message":{"id":"s1","role":"system","content":"system prompt"}}`,
			`{"type":"progress","message":{"id":"p1"}}`,
			`{"type":"file-history-snapshot","message":{"id":"f1"}}`,
			`{"type":"user","message":{"id":"u1","role":"user","content":"Hello"}}`,
		)
		msgs, err := parseConversation(strings.NewReader(jsonl))
		require.NoError(t, err)
		require.Len(t, msgs, 1)
		assert.Equal(t, "Hello", msgs[0].Content)
	})

	t.Run("assistant message grouping by ID", func(t *testing.T) {
		jsonl := lines(
			`{"type":"user","message":{"id":"u1","role":"user","content":"Hello"}}`,
			`{"type":"assistant","message":{"id":"a1","role":"assistant","content":[{"type":"text","text":"part one "}]}}`,
			`{"type":"assistant","message":{"id":"a1","role":"assistant","content":[{"type":"text","text":"part two"}]}}`,
		)
		msgs, err := parseConversation(strings.NewReader(jsonl))
		require.NoError(t, err)
		require.Len(t, msgs, 2)
		assert.Equal(t, "part one part two", msgs[1].Content)
	})

	t.Run("different assistant IDs produce separate messages", func(t *testing.T) {
		jsonl := lines(
			`{"type":"user","message":{"id":"u1","role":"user","content":"Q1"}}`,
			`{"type":"assistant","message":{"id":"a1","role":"assistant","content":[{"type":"text","text":"A1"}]}}`,
			`{"type":"user","message":{"id":"u2","role":"user","content":"Q2"}}`,
			`{"type":"assistant","message":{"id":"a2","role":"assistant","content":[{"type":"text","text":"A2"}]}}`,
		)
		msgs, err := parseConversation(strings.NewReader(jsonl))
		require.NoError(t, err)
		require.Len(t, msgs, 4)
		assert.Equal(t, "A1", msgs[1].Content)
		assert.Equal(t, "A2", msgs[3].Content)
	})

	t.Run("thinking and tool_use blocks are skipped", func(t *testing.T) {
		jsonl := lines(
			`{"type":"user","message":{"id":"u1","role":"user","content":"Hello"}}`,
			`{"type":"assistant","message":{"id":"a1","role":"assistant","content":[{"type":"thinking","text":"hmm..."},{"type":"text","text":"answer"},{"type":"tool_use","text":"{}"}]}}`,
		)
		msgs, err := parseConversation(strings.NewReader(jsonl))
		require.NoError(t, err)
		require.Len(t, msgs, 2)
		assert.Equal(t, "answer", msgs[1].Content)
	})

	t.Run("empty conversation", func(t *testing.T) {
		msgs, err := parseConversation(strings.NewReader(""))
		require.NoError(t, err)
		assert.Empty(t, msgs)
	})

	t.Run("malformed lines are skipped", func(t *testing.T) {
		jsonl := lines(
			`not json`,
			`{"type":"user","message":{"id":"u1","role":"user","content":"Hello"}}`,
		)
		msgs, err := parseConversation(strings.NewReader(jsonl))
		require.NoError(t, err)
		require.Len(t, msgs, 1)
		assert.Equal(t, "Hello", msgs[0].Content)
	})

	t.Run("user content as array of blocks", func(t *testing.T) {
		jsonl := lines(
			`{"type":"user","message":{"id":"u1","role":"user","content":[{"type":"text","text":"block content"}]}}`,
		)
		msgs, err := parseConversation(strings.NewReader(jsonl))
		require.NoError(t, err)
		require.Len(t, msgs, 1)
		assert.Equal(t, "block content", msgs[0].Content)
	})
}

func TestStoreListAndGet(t *testing.T) {
	dir := setupTestDir(t,
		"proj1/session1.jsonl", lines(
			`{"type":"user","message":{"id":"u1","role":"user","content":"Hello"}}`,
			`{"type":"assistant","message":{"id":"a1","role":"assistant","content":[{"type":"text","text":"Hi"}]}}`,
		),
		"proj1/session2.jsonl", lines(
			`{"type":"user","message":{"id":"u2","role":"user","content":"Bye"}}`,
		),
	)

	store := NewStore(dir)
	ctx := context.Background()

	t.Run("List returns all conversations", func(t *testing.T) {
		it, err := store.List(ctx)
		require.NoError(t, err)
		defer func() { _ = it.Close() }()

		var dialogues []dialoguemanager.DialogueHeader
		for {
			d, ok := it.Next(ctx)
			if !ok {
				break
			}
			dialogues = append(dialogues, d)
		}
		require.NoError(t, it.Err())
		assert.Len(t, dialogues, 2)

		ids := make(map[string]bool)
		for _, d := range dialogues {
			ids[d.ID] = true
		}
		assert.True(t, ids["proj1/session1"])
		assert.True(t, ids["proj1/session2"])
	})

	t.Run("Get returns specific conversation", func(t *testing.T) {
		d, err := store.Get(ctx, "proj1/session1")
		require.NoError(t, err)
		assert.Equal(t, "proj1/session1", d.ID)
		require.Len(t, d.Messages, 2)
		assert.Equal(t, "Hello", d.Messages[0].Content)
		assert.Equal(t, "Hi", d.Messages[1].Content)
		assert.Equal(t, 2, d.Version)
		assert.False(t, d.UpdatedAt.IsZero())
	})

	t.Run("Get returns ErrNotFound for missing", func(t *testing.T) {
		_, err := store.Get(ctx, "proj1/nonexistent")
		assert.ErrorIs(t, err, storageapi.ErrNotFound)
	})

	t.Run("Get returns ErrNotFound for bad ID format", func(t *testing.T) {
		_, err := store.Get(ctx, "noslash")
		assert.ErrorIs(t, err, storageapi.ErrNotFound)
	})

	t.Run("List with missing projects dir returns empty", func(t *testing.T) {
		s := NewStore(filepath.Join(t.TempDir(), "nonexistent"))
		it, err := s.List(ctx)
		require.NoError(t, err)
		defer func() { _ = it.Close() }()

		_, ok := it.Next(ctx)
		assert.False(t, ok)
	})
}

func TestStoreReadOnly(t *testing.T) {
	store := NewStore(t.TempDir())
	ctx := context.Background()

	assert.ErrorIs(t, store.Create(ctx, dialoguemanager.Dialogue{ID: "x"}), errReadOnly)
	assert.ErrorIs(t, store.Delete(ctx, "x"), errReadOnly)
	assert.ErrorIs(t, store.AppendMessages(ctx, dialoguemanager.Dialogue{}, nil, llm.DialogueUsage{}), errReadOnly)
}

func TestStoreHealth(t *testing.T) {
	t.Run("healthy when projects dir exists", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "projects"), 0o755))
		store := NewStore(dir)
		assert.NoError(t, store.Health(context.Background()))
	})

	t.Run("unhealthy when projects dir missing", func(t *testing.T) {
		store := NewStore(filepath.Join(t.TempDir(), "nonexistent"))
		assert.Error(t, store.Health(context.Background()))
	})
}

// --- helpers ---

func lines(ss ...string) string {
	return strings.Join(ss, "\n") + "\n"
}

// setupTestDir creates a temp directory mimicking ~/.claude with a
// projects/ subdirectory. files is alternating path/content pairs
// relative to projects/.
func setupTestDir(t *testing.T, files ...string) string {
	t.Helper()
	dir := t.TempDir()
	projectsDir := filepath.Join(dir, "projects")
	for i := 0; i < len(files); i += 2 {
		path := filepath.Join(projectsDir, files[i])
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(files[i+1]), 0o644))
	}
	return dir
}
