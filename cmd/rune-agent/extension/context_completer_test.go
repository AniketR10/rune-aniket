// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package extension

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguetui"
)

type symbolsParser struct {
	syntaxapi.Parser
	symbols []string
}

func (p *symbolsParser) ListReferencedSymbols(
	context.Context,
) (iterator.Iterator[string], error) {
	return iterator.FromSlice(p.symbols), nil
}

var _ syntaxapi.Parser = (*symbolsParser)(nil)

func TestContextCompleterResolve(t *testing.T) {
	c := &contextCompleter{}
	tests := []struct {
		name      string
		candidate string
		wantOK    bool
		want      dialoguetui.Attachment
	}{{
		name:      "file",
		candidate: string(dialoguetui.WorkspaceFileIcon) + " pkg/main.go",
		wantOK:    true,
		want:      dialoguetui.NewWorkspaceFileAttachment("pkg/main.go"),
	}, {
		name:      "symbol",
		candidate: string(dialoguetui.SymbolIcon) + " pkg.Symbol",
		wantOK:    true,
		want:      dialoguetui.NewSymbolAttachment("pkg.Symbol"),
	}, {
		name:      "unknown icon",
		candidate: "? pkg/main.go",
	}, {
		name:      "missing separator",
		candidate: string(dialoguetui.WorkspaceFileIcon) + "pkg/main.go",
	}, {
		name:      "empty",
		candidate: "",
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := c.Resolve(tc.candidate)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestContextCompleterCandidates(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "pkg"), 0o700))
	require.NoError(t, os.WriteFile(
		filepath.Join(root, "pkg", "main.go"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(
		filepath.Join(root, "README.md"), []byte("x"), 0o600))

	c := newContextCompleter(testLocalFS{root: root},
		&symbolsParser{symbols: []string{"pkg.A", "pkg.B", "pkg.A"}})

	it, err := c.Candidates(context.Background(), "")
	require.NoError(t, err)
	defer func() { require.NoError(t, it.Close()) }()

	var got []string
	for {
		v, ok := it.Next(context.Background())
		if !ok {
			break
		}
		got = append(got, v)
	}

	filePrefix := string(dialoguetui.WorkspaceFileIcon) + " "
	symPrefix := string(dialoguetui.SymbolIcon) + " "
	assert.Contains(t, got, filePrefix+"pkg/main.go")
	assert.Contains(t, got, filePrefix+"README.md")
	assert.Equal(t, []string{symPrefix + "pkg.A", symPrefix + "pkg.B"},
		filterPrefix(got, symPrefix), "symbol duplicates must collapse")
}

type homeTestFS struct {
	testLocalFS
	home string
}

func (f homeTestFS) URI(path string) (workspaceapi.URI, error) {
	switch {
	case path == "~":
		path = f.home
	case strings.HasPrefix(path, "~/"):
		path = filepath.Join(f.home, path[2:])
	}
	return f.testLocalFS.URI(path)
}

func TestContextCompleterCandidatesRebaseHomePath(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	desktop := filepath.Join(home, "Desktop")
	require.NoError(t, os.MkdirAll(desktop, 0o700))
	require.NoError(t, os.WriteFile(
		filepath.Join(desktop, "notes.txt"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(
		filepath.Join(home, "outside.txt"), []byte("x"), 0o600))

	c := newContextCompleter(homeTestFS{
		testLocalFS: testLocalFS{root: root}, home: home,
	}, &symbolsParser{symbols: []string{"pkg.DesktopNotes"}})
	it, err := c.Candidates(t.Context(), "~/Desktop/")
	require.NoError(t, err)
	defer func() { require.NoError(t, it.Close()) }()

	var got []string
	for {
		candidate, ok := it.Next(t.Context())
		if !ok {
			break
		}
		got = append(got, candidate)
	}

	filePrefix := string(dialoguetui.WorkspaceFileIcon) + " "
	symPrefix := string(dialoguetui.SymbolIcon) + " "
	assert.Contains(t, got, filePrefix+"~/Desktop/notes.txt")
	assert.NotContains(t, got, filePrefix+"~/outside.txt")
	assert.Contains(t, got, symPrefix+"pkg.DesktopNotes")
}

// TestContextCompleterSkipsHiddenEntries covers both walk roots: inside
// the workspace the gitignore matcher may already hide dot entries, but
// outside it there is no gitignore to fall back on, so hidden trees like
// ~/.cache leaked into the completion band.
func TestContextCompleterSkipsHiddenEntries(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	for _, dir := range []string{
		filepath.Join(root, ".git"),
		filepath.Join(root, "pkg"),
		filepath.Join(home, ".cache", "zigcorpus"),
		filepath.Join(home, "Desktop"),
	} {
		require.NoError(t, os.MkdirAll(dir, 0o700))
	}
	for _, f := range []string{
		filepath.Join(root, ".git", "HEAD"),
		filepath.Join(root, ".env"),
		filepath.Join(root, "pkg", "main.go"),
		filepath.Join(home, ".cache", "zigcorpus", "blob.zig"),
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, "Desktop", "notes.txt"),
	} {
		require.NoError(t, os.WriteFile(f, []byte("x"), 0o600))
	}

	filePrefix := string(dialoguetui.WorkspaceFileIcon) + " "
	newCompleter := func() *contextCompleter {
		return newContextCompleter(homeTestFS{
			testLocalFS: testLocalFS{root: root}, home: home,
		}, &symbolsParser{})
	}

	for _, tc := range []struct {
		name    string
		query   string
		want    string
		exclude []string
	}{
		{
			name:  "workspace root",
			query: "",
			want:  "pkg/main.go",
			exclude: []string{
				".git/HEAD", ".env",
			},
		},
		{
			name:  "home outside the workspace",
			query: "~/",
			want:  "~/Desktop/notes.txt",
			exclude: []string{
				"~/.cache/zigcorpus/blob.zig", "~/.zshrc",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newCompleter()
			it, err := c.Candidates(t.Context(), tc.query)
			require.NoError(t, err)
			defer func() { require.NoError(t, it.Close()) }()

			var got []string
			for {
				candidate, ok := it.Next(t.Context())
				if !ok {
					break
				}
				got = append(got, candidate)
			}

			assert.Contains(t, got, filePrefix+tc.want)
			for _, e := range tc.exclude {
				assert.NotContains(t, got, filePrefix+e)
			}
		})
	}
}

func TestContextCompleterCandidatesReevaluateFileRoot(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "pkg"), 0o700))
	require.NoError(t, os.WriteFile(
		filepath.Join(root, "pkg", "main.go"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(
		filepath.Join(root, "root.go"), []byte("x"), 0o600))
	external := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(external, "notes.txt"), []byte("x"), 0o600))

	c := newContextCompleter(testLocalFS{root: root},
		&symbolsParser{symbols: []string{"pkg.Symbol"}})
	filePrefix := string(dialoguetui.WorkspaceFileIcon) + " "
	symbol := string(dialoguetui.SymbolIcon) + " pkg.Symbol"
	tests := []struct {
		name    string
		query   string
		want    string
		notWant string
	}{
		{"relative directory", "pkg/", filePrefix + "pkg/main.go", filePrefix + "root.go"},
		{"dot relative directory", "./pkg/", filePrefix + "./pkg/main.go", filePrefix + "root.go"},
		{"relative partial file", "pkg/ma", filePrefix + "pkg/main.go", filePrefix + "root.go"},
		{"absolute workspace directory", filepath.Join(root, "pkg") + string(filepath.Separator),
			filePrefix + filepath.Join(root, "pkg", "main.go"), filePrefix + "root.go"},
		{"absolute directory", external + string(filepath.Separator),
			filePrefix + filepath.Join(external, "notes.txt"), filePrefix + "root.go"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			it, err := c.Candidates(t.Context(), tc.query)
			require.NoError(t, err)
			defer func() { require.NoError(t, it.Close()) }()
			var got []string
			for {
				candidate, ok := it.Next(t.Context())
				if !ok {
					break
				}
				got = append(got, candidate)
			}
			assert.Contains(t, got, tc.want)
			assert.NotContains(t, got, tc.notWant)
			assert.Contains(t, got, symbol)
		})
	}
}

func filterPrefix(in []string, prefix string) []string {
	var out []string
	for _, s := range in {
		if strings.HasPrefix(s, prefix) {
			out = append(out, s)
		}
	}
	return out
}

type stubSymbolTool struct {
	agent.Tool
	name    string
	content string
	isError bool
	gotArgs string
}

func (s *stubSymbolTool) Definition() llmapi.Tool {
	return llmapi.Tool{
		Type:     llmapi.ToolTypeFunction,
		Function: llmapi.FunctionDefinition{Name: s.name},
	}
}

func (s *stubSymbolTool) Execute(
	_ context.Context, arguments string,
) agent.ToolResult {
	s.gotArgs = arguments
	return agent.ToolResult{Content: s.content, IsError: s.isError}
}

func TestSymbolContext(t *testing.T) {
	def := &stubSymbolTool{name: "find_definition", content: "a.go:1"}
	refs := &stubSymbolTool{name: "find_references", content: "boom", isError: true}
	doc := &stubSymbolTool{name: "describe_symbol", content: "docs"}
	h := &aiEditorHandler{toolRegistry: agent.NewRegistry(def, refs, doc)}

	got := h.symbolContext(context.Background(), "pkg.Symbol")

	assert.Equal(t, strings.Join([]string{
		"## Definition",
		"- `a.go:1`",
		"",
		"## References",
		"(find_references failed: boom)",
		"",
		"## Documentation",
		"docs",
	}, "\n"), got)
	assert.JSONEq(t, `{"symbol":"pkg.Symbol"}`, def.gotArgs)
}

func TestSymbolContextSkipsMissingTools(t *testing.T) {
	doc := &stubSymbolTool{name: "describe_symbol", content: "docs"}
	h := &aiEditorHandler{toolRegistry: agent.NewRegistry(doc)}

	got := h.symbolContext(context.Background(), "pkg.Symbol")

	assert.Contains(t, got, "(find_definition is unavailable)")
	assert.Contains(t, got, "(find_references is unavailable)")
	assert.Contains(t, got, "docs")
}

func TestLocationList(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"empty", "", ""},
		{
			name:    "one item per line",
			content: "a.go:1:x\nb.go:2:y",
			want:    "- `a.go:1:x`\n- `b.go:2:y`",
		},
		{
			name:    "blank and trailing-space lines are dropped",
			content: "a.go:1:x\n  \n\nb.go:2:y \n",
			want:    "- `a.go:1:x`\n- `b.go:2:y`",
		},
		{
			name:    "identifiers with emphasis characters stay literal",
			content: "a.go:1:func foo_bar(x *T) *T",
			want:    "- `a.go:1:func foo_bar(x *T) *T`",
		},
		{
			name:    "embedded backticks widen the fence",
			content: "a.go:1:s := `raw`",
			want:    "- `` a.go:1:s := `raw` ``",
		},
		{
			name:    "leading backtick is padded",
			content: "`a.go:1",
			want:    "- `` `a.go:1 ``",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, locationList(tc.content))
		})
	}
}

var _ workspaceapi.FileSystem = testLocalFS{}
