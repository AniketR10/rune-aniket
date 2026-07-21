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

package idelsp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
)

const fallbackTestURI = "file:///ws/main.zig"

// errIter is a canned iterator with an optional terminal error.
type errIter[T any] struct {
	items []T
	err   error
	i     int
}

func (it *errIter[T]) Next(context.Context) (T, bool) {
	var zero T
	if it.i >= len(it.items) {
		return zero, false
	}
	v := it.items[it.i]
	it.i++
	return v, true
}

func (it *errIter[T]) Err() error   { return it.err }
func (it *errIter[T]) Close() error { return nil }

// fallbackParser implements syntaxapi.Parser with canned data.
type fallbackParser struct {
	captures []syntaxapi.Result
	queryErr error
	iterErr  error
	resolves map[string][]syntaxapi.Match
	resolved []string
	names    []string
	listErr  error
}

func (p *fallbackParser) Search(
	string, []string, ...string,
) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.Empty[syntaxapi.Result](), nil
}

func (p *fallbackParser) SearchNode(
	syntaxapi.NodeCaptureName, ...string,
) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.Empty[syntaxapi.Result](), nil
}

func (p *fallbackParser) Query(
	workspaceapi.URI, string, []string,
) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.Empty[syntaxapi.Result](), nil
}

func (p *fallbackParser) QueryNode(
	workspaceapi.URI, syntaxapi.NodeCaptureName,
) (iterator.Iterator[syntaxapi.Result], error) {
	if p.queryErr != nil {
		return nil, p.queryErr
	}
	return &errIter[syntaxapi.Result]{
		items: p.captures, err: p.iterErr,
	}, nil
}

func (p *fallbackParser) Highlight(
	workspaceapi.URI, string,
) (iterator.Iterator[textapi.Location], error) {
	return iterator.Empty[textapi.Location](), nil
}

func (p *fallbackParser) ResolveSymbol(
	_ context.Context, name string, _ syntaxapi.Progress,
) (iterator.Iterator[syntaxapi.Match], error) {
	p.resolved = append(p.resolved, name)
	return iterator.FromSlice(p.resolves[name]), nil
}

func (p *fallbackParser) ListReferencedSymbols(
	context.Context,
) (iterator.Iterator[string], error) {
	if p.listErr != nil {
		return nil, p.listErr
	}
	return iterator.FromSlice(p.names), nil
}

func capr(name, text string, fy, fx, ty, tx int) syntaxapi.Result {
	return syntaxapi.Result{
		Text:        text,
		From:        term.Coordinates{Y: fy, X: fx},
		To:          term.Coordinates{Y: ty, X: tx},
		CaptureName: name,
	}
}

// baseCaptures models this synthetic locals.scm pass:
//
//	L0: fn outer(a: i32) void {   // + ref y at (0,15)
//	L1:     var x = a;            // + def y at (1,16)
//	L2:     {
//	L3:         var x = 1;
//	L4:         use(x);
//	L5:     }
//	L6:     use(x);               // + def y at (6,16)
//	L7:     use(a);               // + ref y at (7,16)
//	L8: }
func baseCaptures() []syntaxapi.Result {
	return []syntaxapi.Result{
		capr("local.scope", "", 0, 0, 8, 1),
		capr("local.scope", "", 2, 4, 5, 5),
		capr("local.definition.function", "outer", 0, 3, 0, 8),
		capr("local.definition.var", "a", 0, 9, 0, 10),
		capr("local.definition.var", "x", 1, 8, 1, 9),
		capr("local.definition.var", "x", 3, 12, 3, 13),
		capr("local.definition.var", "y", 1, 16, 1, 17),
		capr("local.definition.var", "y", 6, 16, 6, 17),
		capr("local.reference", "a", 1, 12, 1, 13),
		capr("local.reference", "x", 4, 12, 4, 13),
		capr("local.reference", "x", 6, 8, 6, 9),
		capr("local.reference", "a", 7, 8, 7, 9),
		capr("local.reference", "y", 0, 15, 0, 16),
		capr("local.reference", "y", 7, 16, 7, 17),
		capr("local.reference", "outer", 9, 0, 9, 5),
	}
}

func newTestFallback(
	p *fallbackParser, indexed bool, content string,
) *syntaxFallback {
	return newSyntaxFallback(p, func(string) (string, error) {
		if content == "" {
			return "", errors.New("no content")
		}
		return content, nil
	}, indexed)
}

func TestNewSyntaxFallbackPanics(t *testing.T) {
	readFile := func(string) (string, error) { return "", nil }
	require.Panics(t, func() {
		newSyntaxFallback(nil, readFile, false)
	})
	require.Panics(t, func() {
		newSyntaxFallback(&fallbackParser{}, nil, false)
	})
}

func defParams(line, char int) semanticapi.DefinitionParams {
	return semanticapi.DefinitionParams{
		TextDocument: semanticapi.TextDocumentIdentifier{
			URI: fallbackTestURI,
		},
		Position: semanticapi.Position{
			Line: uint32(line), Character: uint32(char),
		},
	}
}

func wantRange(sy, sx, ey, ex int) semanticapi.Range {
	return semanticapi.Range{
		Start: semanticapi.Position{
			Line: uint32(sy), Character: uint32(sx),
		},
		End: semanticapi.Position{
			Line: uint32(ey), Character: uint32(ex),
		},
	}
}

func TestFallbackDefinitionLocals(t *testing.T) {
	tests := []struct {
		name      string
		line, col int
		want      semanticapi.Range
	}{
		{
			name: "reference resolves to outer var",
			line: 6, col: 8,
			want: wantRange(1, 8, 1, 9),
		},
		{
			name: "shadowed reference resolves to inner var",
			line: 4, col: 12,
			want: wantRange(3, 12, 3, 13),
		},
		{
			name: "reference resolves to parameter",
			line: 7, col: 8,
			want: wantRange(0, 9, 0, 10),
		},
		{
			name: "reference in initializer resolves to parameter",
			line: 1, col: 12,
			want: wantRange(0, 9, 0, 10),
		},
		{
			name: "cursor on definition resolves to itself",
			line: 3, col: 12,
			want: wantRange(3, 12, 3, 13),
		},
		{
			name: "cursor at end of identifier still hits",
			line: 6, col: 9,
			want: wantRange(1, 8, 1, 9),
		},
		{
			name: "same scope picks latest preceding definition",
			line: 7, col: 16,
			want: wantRange(6, 16, 6, 17),
		},
		{
			name: "reference before any definition picks earliest",
			line: 0, col: 15,
			want: wantRange(1, 16, 1, 17),
		},
		{
			name: "function name hoisted outside its own scope",
			line: 9, col: 0,
			want: wantRange(0, 3, 0, 8),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &fallbackParser{captures: baseCaptures()}
			s := newTestFallback(p, false, "")
			res, err := s.definition(
				context.Background(), defParams(tt.line, tt.col),
			)
			require.NoError(t, err)
			require.NotNil(t, res.Location)
			assert.Equal(t, fallbackTestURI, res.Location.URI)
			assert.Equal(t, tt.want, res.Location.Range)
			assert.Empty(t, p.resolved,
				"local resolution must not hit the symbol index")
		})
	}
}

func qualifiedContent() string {
	return strings.Repeat("\n", 9) + "const top = std.mem.copy;\n"
}

func TestFallbackDefinitionQualified(t *testing.T) {
	match := syntaxapi.Match{
		URI: "file:///dep/std.zig",
		Pos: term.Coordinates{Y: 2, X: 4},
	}
	wantLoc := semanticapi.Location{
		URI:   "file:///dep/std.zig",
		Range: wantRange(2, 4, 2, 4),
	}
	tests := []struct {
		name         string
		col          int
		resolves     map[string][]syntaxapi.Match
		wantResolved []string
	}{
		{
			name: "full dotted token resolves",
			col:  21, // on "copy"
			resolves: map[string][]syntaxapi.Match{
				"std.mem.copy": {match},
			},
			wantResolved: []string{"std.mem.copy"},
		},
		{
			name: "shorter dotted suffix retried",
			col:  21,
			resolves: map[string][]syntaxapi.Match{
				"mem.copy": {match},
			},
			wantResolved: []string{"std.mem.copy", "mem.copy"},
		},
		{
			name: "token truncated after cursor segment",
			col:  17, // on "mem"
			resolves: map[string][]syntaxapi.Match{
				"std.mem": {match},
			},
			wantResolved: []string{"std.mem"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &fallbackParser{
				captures: baseCaptures(),
				resolves: tt.resolves,
			}
			s := newTestFallback(p, false, qualifiedContent())
			res, err := s.definition(
				context.Background(), defParams(9, tt.col),
			)
			require.NoError(t, err)
			require.Len(t, res.Locations, 1)
			assert.Equal(t, wantLoc, res.Locations[0])
			assert.Equal(t, tt.wantResolved, p.resolved)
		})
	}
}

func TestFallbackDefinitionUnqualified(t *testing.T) {
	content := strings.Repeat("\n", 10) + "    Foo()\n"
	newParser := func() *fallbackParser {
		return &fallbackParser{
			captures: append(baseCaptures(),
				capr("local.reference", "Foo", 10, 4, 10, 7)),
			names: []string{"p1.Foo", "p2.Foo", "px.Bar"},
			resolves: map[string][]syntaxapi.Match{
				"p1.Foo": {{
					URI: "file:///ws/p1.zig",
					Pos: term.Coordinates{Y: 1, X: 0},
				}},
				"p2.Foo": {{
					URI: "file:///ws/p2.zig",
					Pos: term.Coordinates{Y: 2, X: 0},
				}},
			},
		}
	}

	t.Run("indexed returns all namespace candidates", func(t *testing.T) {
		p := newParser()
		s := newTestFallback(p, true, content)
		res, err := s.definition(context.Background(), defParams(10, 4))
		require.NoError(t, err)
		require.Len(t, res.Locations, 2)
		assert.Equal(t, "file:///ws/p1.zig", res.Locations[0].URI)
		assert.Equal(t, "file:///ws/p2.zig", res.Locations[1].URI)
	})

	t.Run("not indexed skips workspace enumeration", func(t *testing.T) {
		p := newParser()
		s := newTestFallback(p, false, content)
		res, err := s.definition(context.Background(), defParams(10, 4))
		require.NoError(t, err)
		assert.Nil(t, res.Location)
		assert.Empty(t, res.Locations)
		assert.Empty(t, p.resolved)
	})
}

func TestFallbackWorkspaceSymbolNotIndexed(t *testing.T) {
	p := &fallbackParser{names: []string{"mypkg.Frobnicate"}}
	s := newTestFallback(p, false, "")
	syms, err := s.workspaceSymbol(
		context.Background(),
		semanticapi.WorkspaceSymbolParams{Query: "frob"},
	)
	require.NoError(t, err)
	assert.Empty(t, syms)
	assert.Empty(t, p.resolved,
		"unindexed fallback must not enumerate the workspace")
}

func TestFallbackDefinitionErrors(t *testing.T) {
	t.Run("query error propagates", func(t *testing.T) {
		p := &fallbackParser{queryErr: errors.New("not installed")}
		s := newTestFallback(p, false, "")
		_, err := s.definition(context.Background(), defParams(0, 0))
		require.ErrorContains(t, err, "not installed")
	})

	t.Run("iterator terminal error propagates", func(t *testing.T) {
		p := &fallbackParser{
			captures: baseCaptures(),
			iterErr:  errors.New("grammar broke"),
		}
		s := newTestFallback(p, false, "")
		_, err := s.definition(context.Background(), defParams(6, 8))
		require.ErrorContains(t, err, "grammar broke")
	})

	t.Run("nothing under cursor yields empty result", func(t *testing.T) {
		p := &fallbackParser{captures: baseCaptures()}
		s := newTestFallback(p, false, "")
		res, err := s.definition(context.Background(), defParams(2, 0))
		require.NoError(t, err)
		assert.Nil(t, res.Location)
		assert.Empty(t, res.Locations)
	})
}

func refParams(
	line, char int, includeDecl bool,
) semanticapi.ReferenceParams {
	return semanticapi.ReferenceParams{
		TextDocument: semanticapi.TextDocumentIdentifier{
			URI: fallbackTestURI,
		},
		Position: semanticapi.Position{
			Line: uint32(line), Character: uint32(char),
		},
		Context: semanticapi.ReferenceContext{
			IncludeDeclaration: includeDecl,
		},
	}
}

func TestFallbackReferences(t *testing.T) {
	tests := []struct {
		name        string
		line, col   int
		includeDecl bool
		want        []semanticapi.Range
	}{
		{
			name: "outer var excludes shadowed sites",
			line: 6, col: 8, includeDecl: true,
			want: []semanticapi.Range{
				wantRange(1, 8, 1, 9), // declaration
				wantRange(6, 8, 6, 9),
			},
		},
		{
			name: "without declaration",
			line: 6, col: 8, includeDecl: false,
			want: []semanticapi.Range{wantRange(6, 8, 6, 9)},
		},
		{
			name: "cursor on inner definition finds its references",
			line: 3, col: 12, includeDecl: true,
			want: []semanticapi.Range{
				wantRange(3, 12, 3, 13), // declaration
				wantRange(4, 12, 4, 13),
			},
		},
		{
			name: "parameter references",
			line: 0, col: 9, includeDecl: false,
			want: []semanticapi.Range{
				wantRange(1, 12, 1, 13),
				wantRange(7, 8, 7, 9),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &fallbackParser{captures: baseCaptures()}
			s := newTestFallback(p, false, "")
			locs, err := s.references(
				context.Background(),
				refParams(tt.line, tt.col, tt.includeDecl),
			)
			require.NoError(t, err)
			got := make([]semanticapi.Range, 0, len(locs))
			for _, l := range locs {
				assert.Equal(t, fallbackTestURI, l.URI)
				got = append(got, l.Range)
			}
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}

func TestFallbackReferencesQualified(t *testing.T) {
	p := &fallbackParser{
		captures: baseCaptures(),
		resolves: map[string][]syntaxapi.Match{
			"std.mem.copy": {
				{URI: "file:///a.zig", Pos: term.Coordinates{Y: 1, X: 1}},
				{URI: "file:///b.zig", Pos: term.Coordinates{Y: 2, X: 2}},
			},
		},
	}
	s := newTestFallback(p, false, qualifiedContent())
	locs, err := s.references(
		context.Background(), refParams(9, 21, true),
	)
	require.NoError(t, err)
	require.Len(t, locs, 2)
	assert.Equal(t, "file:///a.zig", locs[0].URI)
	assert.Equal(t, "file:///b.zig", locs[1].URI)
}

func TestFallbackWorkspaceSymbol(t *testing.T) {
	newParser := func() *fallbackParser {
		return &fallbackParser{
			names: []string{
				"mypkg.Frobnicate",
				"mypkg.Widget.Render",
				"other.Unrelated",
			},
			resolves: map[string][]syntaxapi.Match{
				"mypkg.Frobnicate": {{
					URI: "file:///ws/frob.zig",
					Pos: term.Coordinates{Y: 3, X: 0},
				}},
				"mypkg.Widget.Render": {{
					URI: "file:///ws/widget.zig",
					Pos: term.Coordinates{Y: 8, X: 0},
				}},
			},
		}
	}

	t.Run("fuzzy query picks function", func(t *testing.T) {
		s := newTestFallback(newParser(), true, "")
		syms, err := s.workspaceSymbol(
			context.Background(),
			semanticapi.WorkspaceSymbolParams{Query: "frob"},
		)
		require.NoError(t, err)
		require.Len(t, syms, 1)
		assert.Equal(t, "mypkg.Frobnicate", syms[0].Name)
		assert.Equal(t, semanticapi.SymbolKindFunction, syms[0].Kind)
		assert.Equal(t, "file:///ws/frob.zig", syms[0].Location.URI)
	})

	t.Run("method kind from container segment", func(t *testing.T) {
		s := newTestFallback(newParser(), true, "")
		syms, err := s.workspaceSymbol(
			context.Background(),
			semanticapi.WorkspaceSymbolParams{Query: "render"},
		)
		require.NoError(t, err)
		require.Len(t, syms, 1)
		assert.Equal(t, "mypkg.Widget.Render", syms[0].Name)
		assert.Equal(t, semanticapi.SymbolKindMethod, syms[0].Kind)
	})

	t.Run("unresolvable names are dropped", func(t *testing.T) {
		s := newTestFallback(newParser(), true, "")
		syms, err := s.workspaceSymbol(
			context.Background(),
			semanticapi.WorkspaceSymbolParams{Query: "unrelated"},
		)
		require.NoError(t, err)
		assert.Empty(t, syms)
	})
}

func TestLanguageNotSupportedSentinel(t *testing.T) {
	_, err := languageForFilename("/ws/notes.md")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrLanguageNotSupported))
	assert.Contains(t, err.Error(), "LSP is not supported yet")
	assert.Contains(t, err.Error(), "markdown")
	assert.True(t, isNoServer(err))
	assert.True(t, isNoServer(ErrNoServer))
	assert.False(t, isNoServer(errors.New("boom")))
}

func TestManagerDefinitionFallback(t *testing.T) {
	uri, err := workspaceapi.ParseURI("file:///ws")
	require.NoError(t, err)

	t.Run("no server falls back to syntax", func(t *testing.T) {
		p := &fallbackParser{captures: baseCaptures()}
		m := New(uri, newTestScheme(), nil, nil, nil, nil, Config{
			NoInitializeServer: true, Parser: p,
		})
		defer m.Close() // nolint:errcheck
		res, derr := m.Definition(
			context.Background(), defParams(6, 8),
		)
		require.NoError(t, derr)
		require.NotNil(t, res.Location)
		assert.Equal(t, wantRange(1, 8, 1, 9), res.Location.Range)
	})

	t.Run("no parser preserves error", func(t *testing.T) {
		m := New(uri, newTestScheme(), nil, nil, nil, nil, Config{
			NoInitializeServer: true,
		})
		defer m.Close() // nolint:errcheck
		_, derr := m.Definition(
			context.Background(), defParams(6, 8),
		)
		require.Error(t, derr)
		assert.True(t, errors.Is(derr, ErrLanguageNotSupported))
	})

	t.Run("fallback failure preserves original error", func(t *testing.T) {
		p := &fallbackParser{queryErr: errors.New("not installed")}
		m := New(uri, newTestScheme(), nil, nil, nil, nil, Config{
			NoInitializeServer: true, Parser: p,
		})
		defer m.Close() // nolint:errcheck
		_, derr := m.Definition(
			context.Background(), defParams(6, 8),
		)
		require.Error(t, derr)
		assert.True(t, errors.Is(derr, ErrLanguageNotSupported))
	})
}

func TestManagerReferencesFallback(t *testing.T) {
	uri, err := workspaceapi.ParseURI("file:///ws")
	require.NoError(t, err)
	p := &fallbackParser{captures: baseCaptures()}
	m := New(uri, newTestScheme(), nil, nil, nil, nil, Config{
		NoInitializeServer: true, Parser: p,
	})
	defer m.Close() // nolint:errcheck
	locs, rerr := m.References(
		context.Background(), refParams(6, 8, true),
	)
	require.NoError(t, rerr)
	require.Len(t, locs, 2)
}

func TestManagerWorkspaceSymbolFallback(t *testing.T) {
	uri, err := workspaceapi.ParseURI("file:///ws")
	require.NoError(t, err)
	newParser := func() *fallbackParser {
		return &fallbackParser{
			names: []string{"mypkg.Frobnicate"},
			resolves: map[string][]syntaxapi.Match{
				"mypkg.Frobnicate": {{
					URI: "file:///ws/frob.zig",
					Pos: term.Coordinates{Y: 3, X: 0},
				}},
			},
		}
	}

	t.Run("indexed serves fuzzy results", func(t *testing.T) {
		m := New(uri, newTestScheme(), nil, nil, nil, nil, Config{
			NoInitializeServer: true, Parser: newParser(),
			IndexedSymbols: true,
		})
		defer m.Close() // nolint:errcheck
		syms, serr := m.WorkspaceSymbol(
			context.Background(),
			semanticapi.WorkspaceSymbolParams{Query: "frob"},
		)
		require.NoError(t, serr)
		require.Len(t, syms, 1)
		assert.Equal(t, "mypkg.Frobnicate", syms[0].Name)
	})

	t.Run("not indexed keeps empty result", func(t *testing.T) {
		m := New(uri, newTestScheme(), nil, nil, nil, nil, Config{
			NoInitializeServer: true, Parser: newParser(),
		})
		defer m.Close() // nolint:errcheck
		syms, serr := m.WorkspaceSymbol(
			context.Background(),
			semanticapi.WorkspaceSymbolParams{Query: "frob"},
		)
		require.NoError(t, serr)
		assert.Empty(t, syms)
	})
}
