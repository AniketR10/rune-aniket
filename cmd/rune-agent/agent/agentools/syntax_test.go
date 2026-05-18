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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cmd/rune-agent/agent"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
)

// --- mock Parser ---

type stubParser struct {
	searchNodeFn func(syntaxapi.NodeCaptureName) (iterator.Iterator[syntaxapi.Result], error)
	queryNodeFn  func(workspaceapi.URI, syntaxapi.NodeCaptureName) (iterator.Iterator[syntaxapi.Result], error)
	searchFn     func(string, []string, ...string) (iterator.Iterator[syntaxapi.Result], error)
	queryFn      func(workspaceapi.URI, string, []string) (iterator.Iterator[syntaxapi.Result], error)
}

func (s *stubParser) SearchNode(n syntaxapi.NodeCaptureName) (iterator.Iterator[syntaxapi.Result], error) {
	if s.searchNodeFn != nil {
		return s.searchNodeFn(n)
	}
	return iterator.FromSlice[syntaxapi.Result](nil), nil
}

func (s *stubParser) QueryNode(uri workspaceapi.URI, n syntaxapi.NodeCaptureName) (iterator.Iterator[syntaxapi.Result], error) {
	if s.queryNodeFn != nil {
		return s.queryNodeFn(uri, n)
	}
	return iterator.FromSlice[syntaxapi.Result](nil), nil
}

func (s *stubParser) Search(query string, captures []string, languages ...string) (iterator.Iterator[syntaxapi.Result], error) {
	if s.searchFn != nil {
		return s.searchFn(query, captures, languages...)
	}
	return iterator.FromSlice[syntaxapi.Result](nil), nil
}

func (s *stubParser) Query(uri workspaceapi.URI, query string, captures []string) (iterator.Iterator[syntaxapi.Result], error) {
	if s.queryFn != nil {
		return s.queryFn(uri, query, captures)
	}
	return iterator.FromSlice[syntaxapi.Result](nil), nil
}

func (s *stubParser) Highlight(workspaceapi.URI, string) (iterator.Iterator[textapi.Location], error) {
	return iterator.FromSlice[textapi.Location](nil), nil
}

var _ syntaxapi.Parser = (*stubParser)(nil)

// --- helpers ---

func syntaxResult(path string, line, col int, text string) syntaxapi.Result {
	return syntaxapi.Result{
		File: func() workspaceapi.URI {
			u, _ := workspaceapi.ParseURI("file://" + path)
			return u
		}(),
		Text: text,
		From: term.Coordinates{X: col, Y: line},
	}
}

// --- tests ---

func TestSyntaxTools(t *testing.T) {
	parser := &stubParser{}
	tools := SyntaxTools(parser, localFS{}, dirURI("/workspace"), NewFileTracker())
	require.Len(t, tools, 4)

	expectedNames := map[string]bool{
		"list_symbols":      false,
		"list_file_symbols": false,
		"query_ast":         false,
		"query_file_ast":    false,
	}
	for _, tool := range tools {
		def := tool.Definition()
		assert.Equal(t, llmapi.ToolTypeFunction, def.Type)
		name := def.Function.Name
		_, ok := expectedNames[name]
		assert.True(t, ok, "unexpected tool name: %s", name)
		assert.NotEmpty(t, def.Function.Description)
		assert.NotNil(t, def.Function.Parameters)
		expectedNames[name] = true
	}
	for name, found := range expectedNames {
		assert.True(t, found, "tool %q not returned by SyntaxTools", name)
	}
}

func TestParseNodeTypes(t *testing.T) {
	tests := []struct {
		input   string
		want    syntaxapi.NodeCaptureName
		wantErr bool
	}{
		{"func", syntaxapi.NodeCaptureDefinitionFunc, false},
		{"method", syntaxapi.NodeCaptureDefinitionMethod, false},
		{"type", syntaxapi.NodeCaptureDefinitionType, false},
		{"var", syntaxapi.NodeCaptureDefinitionVar, false},
		{"scope", syntaxapi.NodeCaptureScope, false},
		{"namespace", syntaxapi.NodeCaptureDefinitionNamespace, false},
		{"reference", syntaxapi.NodeCaptureReference, false},
		{"func|method", syntaxapi.NodeCaptureDefinitionFunc | syntaxapi.NodeCaptureDefinitionMethod, false},
		{"func|type|var", syntaxapi.NodeCaptureDefinitionFunc | syntaxapi.NodeCaptureDefinitionType | syntaxapi.NodeCaptureDefinitionVar, false},
		{"FUNC", syntaxapi.NodeCaptureDefinitionFunc, false}, // case insensitive
		{"bogus", 0, true},
		{"func|bogus", 0, true},
		{"", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseNodeTypes(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestListSymbols(t *testing.T) {
	root := "/workspace"

	t.Run("happy path", func(t *testing.T) {
		parser := &stubParser{
			searchNodeFn: func(n syntaxapi.NodeCaptureName) (iterator.Iterator[syntaxapi.Result], error) {
				assert.Equal(t, syntaxapi.NodeCaptureDefinitionFunc, n)
				return iterator.FromSlice([]syntaxapi.Result{
					syntaxResult("/workspace/main.go", 4, 0, "func main()"),
					syntaxResult("/workspace/handler.go", 19, 5, "func Handle()"),
				}), nil
			},
		}
		tool := &listSymbolsTool{parser: parser, cwd: dirURI(root)}
		result := tool.Execute(context.Background(), `{"node_types":"func"}`)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "main.go:5:func main()")
		assert.Contains(t, result.Content, "handler.go:20:func Handle()")
	})

	t.Run("no results", func(t *testing.T) {
		parser := &stubParser{
			searchNodeFn: func(n syntaxapi.NodeCaptureName) (iterator.Iterator[syntaxapi.Result], error) {
				return iterator.FromSlice[syntaxapi.Result](nil), nil
			},
		}
		tool := &listSymbolsTool{parser: parser, cwd: dirURI(root)}
		result := tool.Execute(context.Background(), `{"node_types":"func"}`)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "no results found")
	})

	t.Run("invalid node type", func(t *testing.T) {
		tool := &listSymbolsTool{parser: &stubParser{}, cwd: dirURI(root)}
		result := tool.Execute(context.Background(), `{"node_types":"invalid"}`)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "unknown node type")
	})

	t.Run("parser error", func(t *testing.T) {
		parser := &stubParser{
			searchNodeFn: func(n syntaxapi.NodeCaptureName) (iterator.Iterator[syntaxapi.Result], error) {
				return nil, fmt.Errorf("parser offline")
			},
		}
		tool := &listSymbolsTool{parser: parser, cwd: dirURI(root)}
		result := tool.Execute(context.Background(), `{"node_types":"func"}`)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "parser offline")
	})

	t.Run("invalid JSON", func(t *testing.T) {
		tool := &listSymbolsTool{parser: &stubParser{}, cwd: dirURI(root)}
		result := tool.Execute(context.Background(), `bad`)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "invalid arguments")
	})

	t.Run("summary", func(t *testing.T) {
		tool := &listSymbolsTool{parser: &stubParser{}, cwd: dirURI(root)}
		assert.Equal(t, "func|method", tool.Summary(`{"node_types":"func|method"}`))
		assert.Equal(t, "", tool.Summary(`bad`))
	})
}

func TestListFileSymbols(t *testing.T) {
	root := "/workspace"

	t.Run("happy path", func(t *testing.T) {
		parser := &stubParser{
			queryNodeFn: func(uri workspaceapi.URI, n syntaxapi.NodeCaptureName) (iterator.Iterator[syntaxapi.Result], error) {
				assert.Equal(t, syntaxapi.NodeCaptureDefinitionFunc|syntaxapi.NodeCaptureDefinitionMethod, n)
				return iterator.FromSlice([]syntaxapi.Result{
					syntaxResult("/workspace/handler.go", 9, 0, "func Handle()"),
				}), nil
			},
		}
		tool := &listFileSymbolsTool{parser: parser, fs: localFS{}, cwd: dirURI(root)}
		result := tool.Execute(context.Background(), `{"file_path":"handler.go","node_types":"func|method"}`)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "handler.go:10:func Handle()")
	})

	t.Run("summary", func(t *testing.T) {
		tool := &listFileSymbolsTool{parser: &stubParser{}, fs: localFS{}, cwd: dirURI(root)}
		assert.Equal(t, "handler.go func", tool.Summary(`{"file_path":"handler.go","node_types":"func"}`))
	})
}

func TestQueryAST(t *testing.T) {
	root := "/workspace"

	t.Run("happy path", func(t *testing.T) {
		parser := &stubParser{
			searchFn: func(query string, captures []string, languages ...string) (iterator.Iterator[syntaxapi.Result], error) {
				assert.Equal(t, "(call_expression)", query)
				assert.Equal(t, []string{"call"}, captures)
				return iterator.FromSlice([]syntaxapi.Result{
					syntaxResult("/workspace/main.go", 9, 1, "fmt.Println()"),
				}), nil
			},
		}
		tool := &queryASTTool{parser: parser, cwd: dirURI(root)}
		result := tool.Execute(context.Background(), `{"query":"(call_expression)","captures":["call"]}`)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "main.go:10:fmt.Println()")
	})

	t.Run("no captures parameter", func(t *testing.T) {
		parser := &stubParser{
			searchFn: func(query string, captures []string, languages ...string) (iterator.Iterator[syntaxapi.Result], error) {
				assert.Nil(t, captures)
				return iterator.FromSlice[syntaxapi.Result](nil), nil
			},
		}
		tool := &queryASTTool{parser: parser, cwd: dirURI(root)}
		result := tool.Execute(context.Background(), `{"query":"(function_declaration)"}`)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "no results found")
	})

	t.Run("parser error", func(t *testing.T) {
		parser := &stubParser{
			searchFn: func(query string, captures []string, languages ...string) (iterator.Iterator[syntaxapi.Result], error) {
				return nil, fmt.Errorf("invalid query syntax")
			},
		}
		tool := &queryASTTool{parser: parser, cwd: dirURI(root)}
		result := tool.Execute(context.Background(), `{"query":"(bad"}`)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "invalid query syntax")
	})

	t.Run("summary truncation", func(t *testing.T) {
		tool := &queryASTTool{parser: &stubParser{}, cwd: dirURI(root)}
		short := `{"query":"(fn)"}`
		assert.Equal(t, "(fn)", tool.Summary(short))

		longQuery := `{"query":"` + string(make([]byte, 100)) + `"}`
		summary := tool.Summary(longQuery)
		assert.LessOrEqual(t, len(summary), 63)
	})
}

func TestQueryFileAST(t *testing.T) {
	root := "/workspace"

	t.Run("happy path", func(t *testing.T) {
		parser := &stubParser{
			queryFn: func(uri workspaceapi.URI, query string, captures []string) (iterator.Iterator[syntaxapi.Result], error) {
				assert.Equal(t, "(call_expression)", query)
				return iterator.FromSlice([]syntaxapi.Result{
					syntaxResult("/workspace/main.go", 2, 0, "os.Exit(1)"),
				}), nil
			},
		}
		tool := &queryFileASTTool{parser: parser, fs: localFS{}, cwd: dirURI(root)}
		result := tool.Execute(context.Background(), `{"file_path":"main.go","query":"(call_expression)"}`)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "main.go:3:os.Exit(1)")
	})

	t.Run("summary", func(t *testing.T) {
		tool := &queryFileASTTool{parser: &stubParser{}, fs: localFS{}, cwd: dirURI(root)}
		assert.Equal(t, "main.go", tool.Summary(`{"file_path":"main.go","query":"(fn)"}`))
	})
}

func TestCollectResults(t *testing.T) {
	t.Run("respects limit", func(t *testing.T) {
		items := make([]syntaxapi.Result, 10)
		for i := range items {
			items[i] = syntaxapi.Result{Text: fmt.Sprintf("item%d", i)}
		}
		it := iterator.FromSlice(items)
		results, truncated := collectResults(context.Background(), it, 3)

		assert.Len(t, results, 3)
		assert.True(t, truncated)
	})

	t.Run("under limit", func(t *testing.T) {
		items := []syntaxapi.Result{{Text: "a"}, {Text: "b"}}
		it := iterator.FromSlice(items)
		results, truncated := collectResults(context.Background(), it, 100)

		assert.Len(t, results, 2)
		assert.False(t, truncated)
	})

	t.Run("empty", func(t *testing.T) {
		it := iterator.FromSlice[syntaxapi.Result](nil)
		results, truncated := collectResults(context.Background(), it, 100)

		assert.Empty(t, results)
		assert.False(t, truncated)
	})
}

func TestSyntaxToolSummaries(t *testing.T) {
	parser := &stubParser{}
	fs := localFS{}
	root := "/workspace"

	tests := []struct {
		name     string
		tool     agent.Tool
		args     string
		expected string
	}{
		{"list_symbols", &listSymbolsTool{parser: parser, cwd: dirURI(root)}, `{"node_types":"func"}`, "func"},
		{"list_symbols invalid", &listSymbolsTool{parser: parser, cwd: dirURI(root)}, `bad`, ""},
		{"list_file_symbols", &listFileSymbolsTool{parser: parser, fs: fs, cwd: dirURI(root)}, `{"file_path":"a.go","node_types":"type"}`, "a.go type"},
		{"query_ast", &queryASTTool{parser: parser, cwd: dirURI(root)}, `{"query":"(fn)"}`, "(fn)"},
		{"query_file_ast", &queryFileASTTool{parser: parser, fs: fs, cwd: dirURI(root)}, `{"file_path":"a.go","query":"(fn)"}`, "a.go"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.tool.Summary(tt.args))
		})
	}
}
