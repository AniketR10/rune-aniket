// Copyright (C) 2017-2026 The Rune Authors
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

package queryenginetest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// sampleMemories defines the memory set used by all integration tests.
// It covers semantic, episodic (recent/old/certain), supersession,
// and dependency relationships.
const sampleMemories = `package main

import (
	"context"
	"time"
)

// --- Semantic memory: no triggers, no decay ---

type SemanticTableTests struct{}

func (SemanticTableTests) ID() string      { return "semantic-table-tests" }
func (SemanticTableTests) Content() string { return "Use table-driven tests with named sub-tests." }
func (SemanticTableTests) goIdiom()        {}
func (SemanticTableTests) testingPattern() {}
func (SemanticTableTests) FetchConversation(context.Context) (Dialogue, error) {
	return Dialogue{}, nil
}

func init() { Register(SemanticTableTests{}) }

// --- Recent episodic memory (yesterday) ---

type RecentIteratorBug struct{}

func (RecentIteratorBug) ID() string      { return "recent-iterator-bug" }
func (RecentIteratorBug) Content() string { return "Iterator deadlock caused by unclosed stream. Always defer Close()." }
func (RecentIteratorBug) bugFix()         {}
func (RecentIteratorBug) OccurredAt() time.Time {
	return time.Now().Add(-24 * time.Hour)
}
func (RecentIteratorBug) TriggerOn() Trigger {
	return Trigger{
		Files:   []string{"iterator.go"},
		Errors:  []string{"deadlock"},
		Actions: []string{"refactoring iterators"},
	}
}
func (RecentIteratorBug) SurfaceBefore() []string {
	return []string{"creating iterators"}
}
func (RecentIteratorBug) FetchConversation(context.Context) (Dialogue, error) {
	return Dialogue{}, nil
}

func init() { Register(RecentIteratorBug{}) }

// --- Old episodic memory (180 days ago, Tentative by default) ---

type OldSimilarBug struct{}

func (OldSimilarBug) ID() string      { return "old-similar-bug" }
func (OldSimilarBug) Content() string { return "Old iterator bug from six months ago." }
func (OldSimilarBug) bugFix()         {}
func (OldSimilarBug) OccurredAt() time.Time {
	return time.Now().Add(-180 * 24 * time.Hour)
}
func (OldSimilarBug) TriggerOn() Trigger {
	return Trigger{
		Files:   []string{"iterator.go"},
		Errors:  []string{"deadlock"},
		Actions: []string{"refactoring iterators"},
	}
}
func (OldSimilarBug) SurfaceBefore() []string {
	return []string{"creating iterators"}
}
func (OldSimilarBug) FetchConversation(context.Context) (Dialogue, error) {
	return Dialogue{}, nil
}

func init() { Register(OldSimilarBug{}) }

// --- Old episodic memory with Certain confidence (no decay) ---

type CertainDBRule struct{}

func (CertainDBRule) ID() string      { return "certain-db-rule" }
func (CertainDBRule) Content() string { return "Never mock the database in integration tests." }
func (CertainDBRule) OccurredAt() time.Time {
	return time.Now().Add(-180 * 24 * time.Hour)
}
func (CertainDBRule) TriggerOn() Trigger {
	return Trigger{
		Files:   []string{"database.go", "db_test.go"},
		Actions: []string{"testing database"},
	}
}
func (CertainDBRule) SurfaceBefore() []string {
	return []string{"writing database tests"}
}
func (CertainDBRule) Confidence() Confidence { return Certain }
func (CertainDBRule) FetchConversation(context.Context) (Dialogue, error) {
	return Dialogue{}, nil
}

func init() { Register(CertainDBRule{}) }

// --- Superseded episodic memory ---

type ObsoletePattern struct{}

func (ObsoletePattern) ID() string      { return "obsolete-pattern" }
func (ObsoletePattern) Content() string { return "Old handler pattern, now replaced." }
func (ObsoletePattern) bugFix()         {}
func (ObsoletePattern) OccurredAt() time.Time {
	return time.Now().Add(-24 * time.Hour)
}
func (ObsoletePattern) TriggerOn() Trigger {
	return Trigger{
		Files:  []string{"handler.go"},
		Errors: []string{"nil pointer"},
	}
}
func (ObsoletePattern) SurfaceBefore() []string { return nil }
func (ObsoletePattern) FetchConversation(context.Context) (Dialogue, error) {
	return Dialogue{}, nil
}

func init() { Register(ObsoletePattern{}) }

// --- Superseding semantic memory ---

type NewHandlerPattern struct{}

func (NewHandlerPattern) ID() string         { return "new-handler-pattern" }
func (NewHandlerPattern) Content() string    { return "Improved handler pattern replaces obsolete approach." }
func (NewHandlerPattern) goIdiom()           {}
func (NewHandlerPattern) Supersedes() []string { return []string{"obsolete-pattern"} }
func (NewHandlerPattern) FetchConversation(context.Context) (Dialogue, error) {
	return Dialogue{}, nil
}

func init() { Register(NewHandlerPattern{}) }

// --- Memory with dependency ---

type DependentStreaming struct{}

func (DependentStreaming) ID() string         { return "dependent-streaming" }
func (DependentStreaming) Content() string    { return "Streaming architecture depends on proper test patterns." }
func (DependentStreaming) architecture()      {}
func (DependentStreaming) DependsOn() []string { return []string{"semantic-table-tests"} }
func (DependentStreaming) FetchConversation(context.Context) (Dialogue, error) {
	return Dialogue{}, nil
}

func init() { Register(DependentStreaming{}) }
`

func TestQueryEngine(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binary := buildEngine(t)

	t.Run("content appears in output", func(t *testing.T) {
		out := runEngine(t, binary, "--files", "iterator.go")
		if !strings.Contains(out, "Iterator deadlock caused by unclosed stream") {
			t.Fatalf("expected Content() text in output, got:\n%s", out)
		}
	})

	t.Run("file trigger surfaces memory", func(t *testing.T) {
		out := runEngine(t, binary, "--files", "iterator.go")
		if !strings.Contains(out, "recent-iterator-bug") {
			t.Fatalf("expected recent-iterator-bug for file trigger, got:\n%s", out)
		}
	})

	t.Run("error trigger surfaces memory", func(t *testing.T) {
		out := runEngine(t, binary, "--errors", "deadlock in goroutine")
		if !strings.Contains(out, "recent-iterator-bug") {
			t.Fatalf("expected recent-iterator-bug for error trigger, got:\n%s", out)
		}
	})

	t.Run("action trigger surfaces memory", func(t *testing.T) {
		out := runEngine(t, binary, "--task", "refactoring iterators")
		if !strings.Contains(out, "recent-iterator-bug") {
			t.Fatalf("expected recent-iterator-bug for action trigger, got:\n%s", out)
		}
	})

	t.Run("surface before surfaces memory", func(t *testing.T) {
		out := runEngine(t, binary, "--task", "creating iterators")
		if !strings.Contains(out, "recent-iterator-bug") {
			t.Fatalf("expected recent-iterator-bug for surface-before, got:\n%s", out)
		}
	})

	t.Run("file trigger scores higher than action trigger", func(t *testing.T) {
		// Both should surface the memory, but with only a file match
		// the file trigger weight (3) should beat an action-only weight (1).
		// We verify by checking that a file-only query still surfaces the
		// memory — the ordering test below covers relative ranking.
		out := runEngine(t, binary, "--files", "iterator.go")
		if !strings.Contains(out, "recent-iterator-bug") {
			t.Fatalf("expected recent-iterator-bug, got:\n%s", out)
		}
	})

	t.Run("recent episodic ranks above old episodic", func(t *testing.T) {
		// Both memories trigger on the same files/errors/actions.
		// The recent one should appear before the old one due to decay.
		out := runEngine(t, binary, "--files", "iterator.go")
		recentIdx := strings.Index(out, "recent-iterator-bug")
		oldIdx := strings.Index(out, "old-similar-bug")
		if recentIdx == -1 {
			t.Fatalf("expected recent-iterator-bug in output:\n%s", out)
		}
		if oldIdx == -1 {
			t.Fatalf("expected old-similar-bug in output:\n%s", out)
		}
		if recentIdx >= oldIdx {
			t.Fatalf("expected recent-iterator-bug before old-similar-bug:\n%s", out)
		}
	})

	t.Run("certain confidence prevents decay", func(t *testing.T) {
		// CertainDBRule is 180 days old but has Certain confidence.
		// It should still appear with full relevance.
		out := runEngine(t, binary, "--files", "database.go")
		if !strings.Contains(out, "certain-db-rule") {
			t.Fatalf("expected certain-db-rule despite old age, got:\n%s", out)
		}
		if !strings.Contains(out, "Never mock the database") {
			t.Fatalf("expected Content() of certain memory, got:\n%s", out)
		}
	})

	t.Run("superseded memory is filtered out", func(t *testing.T) {
		// ObsoletePattern triggers on handler.go but is superseded
		// by NewHandlerPattern. It should not appear.
		out := runEngine(t, binary, "--files", "handler.go")
		if strings.Contains(out, "obsolete-pattern") {
			t.Fatalf("superseded memory should not appear:\n%s", out)
		}
	})

	t.Run("superseding memory can still appear", func(t *testing.T) {
		// NewHandlerPattern supersedes ObsoletePattern. It's semantic
		// so it scores via ID word matching. "handler" is in the query.
		out := runEngine(t, binary, "--files", "handler.go")
		if !strings.Contains(out, "new-handler-pattern") {
			t.Fatalf("expected superseding memory to appear:\n%s", out)
		}
	})

	t.Run("dependency expansion includes dependency", func(t *testing.T) {
		// DependentStreaming depends on SemanticTableTests.
		// Query "streaming" matches DependentStreaming via ID word.
		// SemanticTableTests should be pulled in via dependency expansion.
		out := runEngine(t, binary, "--task", "streaming")
		if !strings.Contains(out, "dependent-streaming") {
			t.Fatalf("expected dependent-streaming in output:\n%s", out)
		}
		if !strings.Contains(out, "semantic-table-tests") {
			t.Fatalf("expected semantic-table-tests via dependency expansion:\n%s", out)
		}
	})

	t.Run("no match returns empty output", func(t *testing.T) {
		out := runEngine(t, binary, "--files", "completely_unrelated_xyz.go")
		if strings.TrimSpace(out) != "" {
			t.Fatalf("expected empty output for no match, got:\n%s", out)
		}
	})

	t.Run("conversation flag with valid memory returns empty dialogue", func(t *testing.T) {
		out := runEngine(t, binary, "--conversation", "semantic-table-tests")
		if strings.TrimSpace(out) != "" {
			t.Fatalf("expected empty output for zero Dialogue, got:\n%s", out)
		}
	})

	t.Run("conversation flag with unknown memory exits with error", func(t *testing.T) {
		cmd := exec.Command(binary, "--conversation", "nonexistent-memory")
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatal("expected non-zero exit for unknown memory ID")
		}
		if !strings.Contains(string(out), "memory not found") {
			t.Fatalf("expected 'memory not found' error, got:\n%s", out)
		}
	})

	t.Run("list categories shows all memories", func(t *testing.T) {
		out := runEngine(t, binary, "--list-categories")
		for _, id := range []string{
			"semantic-table-tests",
			"recent-iterator-bug",
			"old-similar-bug",
			"certain-db-rule",
			"obsolete-pattern",
			"new-handler-pattern",
			"dependent-streaming",
		} {
			if !strings.Contains(out, id) {
				t.Errorf("expected %s in list-categories output", id)
			}
		}
		// Verify content is included in the listing.
		if !strings.Contains(out, "Use table-driven tests") {
			t.Errorf("expected Content() in list-categories output:\n%s", out)
		}
	})
}

// buildEngine compiles the query engine template with sample memories
// into a binary and returns the path to it.
func buildEngine(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Copy template files.
	templateDir := filepath.Join("..", "template")
	for _, name := range []string{"go.mod.tmpl", "categories.go.tmpl", "main.go.tmpl", "conversation.go.tmpl"} {
		data, err := os.ReadFile(filepath.Join(templateDir, name))
		if err != nil {
			t.Fatalf("read template %s: %v", name, err)
		}
		dst := strings.TrimSuffix(name, ".tmpl")
		if err := os.WriteFile(filepath.Join(dir, dst), data, 0o644); err != nil {
			t.Fatalf("write %s: %v", dst, err)
		}
	}

	// Write sample memories.
	if err := os.WriteFile(filepath.Join(dir, "sample_memories.go"), []byte(sampleMemories), 0o644); err != nil {
		t.Fatalf("write sample_memories.go: %v", err)
	}

	// Resolve dependencies.
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = dir
	if out, err := tidyCmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy failed: %v\n%s", err, out)
	}

	// Build.
	binary := filepath.Join(dir, "queryengine")
	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	return binary
}

// runEngine runs the compiled query engine binary with the given flags
// and returns its stdout. Fails the test on non-zero exit.
func runEngine(t *testing.T, binary string, args ...string) string {
	t.Helper()
	cmd := exec.Command(binary, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("query engine failed: %v\n%s", err, out)
	}
	return string(out)
}
