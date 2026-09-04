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

package claudememory

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeGoTemplate(t *testing.T) {
	t.Run("compiles alongside default templates", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}
		workspace := buildWorkspace(t)
		runGoInWorkspace(t, workspace, "", "build", "./...")
	})

	t.Run("FetchClaudeDialogue returns messages", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}
		claudeHome := setupTestDir(t,
			"proj1/session1.jsonl", lines(
				`{"type":"user","message":{"id":"u1","role":"user","content":"Hello"}}`,
				`{"type":"assistant","message":{"id":"a1","role":"assistant","content":[{"type":"text","text":"Hi there!"}]}}`,
			),
		)
		workspace := buildWorkspace(t)
		writeGoFile(t, workspace, "claude_fetch_test.go", `package main

import (
	"context"
	"testing"
)

func TestFetchClaudeDialogue(t *testing.T) {
	d, err := FetchClaudeDialogue(context.Background(), "proj1/session1")
	if err != nil {
		t.Fatalf("FetchClaudeDialogue: %v", err)
	}
	if d.ID != "proj1/session1" {
		t.Fatalf("expected ID proj1/session1, got %s", d.ID)
	}
	if len(d.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(d.Messages))
	}
	if d.Messages[0].Role != "user" || d.Messages[0].Content != "Hello" {
		t.Fatalf("unexpected first message: role=%s content=%s", d.Messages[0].Role, d.Messages[0].Content)
	}
	if d.Messages[1].Role != "assistant" || d.Messages[1].Content != "Hi there!" {
		t.Fatalf("unexpected second message: role=%s content=%s", d.Messages[1].Role, d.Messages[1].Content)
	}
}
`)
		runGoInWorkspace(t, workspace, claudeHome, "test", "./...")
	})

	t.Run("FetchClaudeDialogue invalid ID", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}
		claudeHome := setupTestDir(t)
		workspace := buildWorkspace(t)
		writeGoFile(t, workspace, "claude_invalid_test.go", `package main

import (
	"context"
	"testing"
)

func TestFetchClaudeDialogueInvalidID(t *testing.T) {
	_, err := FetchClaudeDialogue(context.Background(), "noslash")
	if err == nil {
		t.Fatal("expected error for invalid ID")
	}
}
`)
		runGoInWorkspace(t, workspace, claudeHome, "test", "./...")
	})

	t.Run("FetchClaudeDialogue missing file", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}
		claudeHome := setupTestDir(t)
		workspace := buildWorkspace(t)
		writeGoFile(t, workspace, "claude_missing_test.go", `package main

import (
	"context"
	"testing"
)

func TestFetchClaudeDialogueMissingFile(t *testing.T) {
	_, err := FetchClaudeDialogue(context.Background(), "proj/nonexistent")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
`)
		runGoInWorkspace(t, workspace, claudeHome, "test", "./...")
	})

	t.Run("skips sidechains and thinking blocks", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}
		claudeHome := setupTestDir(t,
			"proj1/session1.jsonl", lines(
				`{"type":"user","message":{"id":"u1","role":"user","content":"Hello"}}`,
				`{"type":"assistant","isSidechain":true,"message":{"id":"a-side","role":"assistant","content":[{"type":"text","text":"sidechain"}]}}`,
				`{"type":"assistant","message":{"id":"a1","role":"assistant","content":[{"type":"thinking","text":"hmm"},{"type":"text","text":"real answer"}]}}`,
			),
		)
		workspace := buildWorkspace(t)
		writeGoFile(t, workspace, "claude_filter_test.go", `package main

import (
	"context"
	"testing"
)

func TestFetchClaudeDialogueFiltering(t *testing.T) {
	d, err := FetchClaudeDialogue(context.Background(), "proj1/session1")
	if err != nil {
		t.Fatalf("FetchClaudeDialogue: %v", err)
	}
	if len(d.Messages) != 2 {
		t.Fatalf("expected 2 messages (sidechain/thinking filtered), got %d", len(d.Messages))
	}
	if d.Messages[1].Content != "real answer" {
		t.Fatalf("expected 'real answer', got %q", d.Messages[1].Content)
	}
}
`)
		runGoInWorkspace(t, workspace, claudeHome, "test", "./...")
	})

	t.Run("groups consecutive assistant messages by ID", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}
		claudeHome := setupTestDir(t,
			"proj1/session1.jsonl", lines(
				`{"type":"user","message":{"id":"u1","role":"user","content":"Hello"}}`,
				`{"type":"assistant","message":{"id":"a1","role":"assistant","content":[{"type":"text","text":"part one "}]}}`,
				`{"type":"assistant","message":{"id":"a1","role":"assistant","content":[{"type":"text","text":"part two"}]}}`,
			),
		)
		workspace := buildWorkspace(t)
		writeGoFile(t, workspace, "claude_group_test.go", `package main

import (
	"context"
	"testing"
)

func TestFetchClaudeDialogueGrouping(t *testing.T) {
	d, err := FetchClaudeDialogue(context.Background(), "proj1/session1")
	if err != nil {
		t.Fatalf("FetchClaudeDialogue: %v", err)
	}
	if len(d.Messages) != 2 {
		t.Fatalf("expected 2 messages (grouped), got %d", len(d.Messages))
	}
	if d.Messages[1].Content != "part one part two" {
		t.Fatalf("expected grouped content, got %q", d.Messages[1].Content)
	}
}
`)
		runGoInWorkspace(t, workspace, claudeHome, "test", "./...")
	})
}

// buildWorkspace copies all dream templates into a temp directory
// and runs go mod tidy. Returns the workspace path.
func buildWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	templateDir := filepath.Join("..", "dream", "template")
	entries, err := os.ReadDir(templateDir)
	if err != nil {
		t.Fatalf("read template dir: %v", err)
	}
	for _, e := range entries {
		data, readErr := os.ReadFile(filepath.Join(templateDir, e.Name()))
		if readErr != nil {
			t.Fatalf("read template %s: %v", e.Name(), readErr)
		}
		dst := strings.TrimSuffix(e.Name(), ".tmpl")
		if writeErr := os.WriteFile(filepath.Join(dir, dst), data, 0o644); writeErr != nil {
			t.Fatalf("write %s: %v", dst, writeErr)
		}
	}

	runGoInWorkspace(t, dir, "", "mod", "tidy")
	return dir
}

func writeGoFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// runGoInWorkspace runs a go command in dir. If claudeHome is non-empty,
// CLAUDE_HOME is set in the subprocess environment so that claude.go
// resolves the projects directory from it.
func runGoInWorkspace(t *testing.T, dir, claudeHome string, args ...string) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	if claudeHome != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("CLAUDE_HOME=%s", claudeHome))
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %v failed:\n%s", args, out)
	}
}
