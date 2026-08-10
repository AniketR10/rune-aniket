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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"

	"unstable.build/go-tui/browser/browsertest"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguetui"
)

func cellRows(lines ...string) [][]term.Cell {
	rows := make([][]term.Cell, len(lines))
	for i, line := range lines {
		for _, r := range line {
			rows[i] = append(rows[i], term.Cell{Ch: r})
		}
	}
	return rows
}

func TestWordAt(t *testing.T) {
	//                     0    5    10   15   20   25   30   35   40
	//                     |....|....|....|....|....|....|....|....|
	const decl = "func handleChatAddSymbol(cmd textapi.Command) error {"
	const chain = "  pkg.Symbol.Method(x)"
	const method = "  x().Method()"
	const partial = "  pkg."
	const blanks = "a\tb c\x00d"
	rows := cellRows(decl, "", "  x := foo_bar1 + 2", chain, method, partial, blanks)

	tests := []struct {
		name string
		pos  term.Coordinates
		want string
	}{
		{"start of word", term.Coordinates{X: 5, Y: 0}, "handleChatAddSymbol"},
		{"middle of word", term.Coordinates{X: 12, Y: 0}, "handleChatAddSymbol"},
		{"last rune of word", term.Coordinates{X: 23, Y: 0}, "handleChatAddSymbol"},
		{"on punctuation", term.Coordinates{X: 24, Y: 0}, ""},
		{"underscores and digits", term.Coordinates{X: 9, Y: 2}, "foo_bar1"},
		{"empty line", term.Coordinates{X: 0, Y: 1}, ""},
		{"row out of range", term.Coordinates{X: 0, Y: 9}, ""},
		{"column out of range", term.Coordinates{X: 99, Y: 0}, ""},
		{"negative", term.Coordinates{X: -1, Y: -1}, ""},

		// A caret on a container segment names that container, not the
		// members hanging off it.
		{"container segment", term.Coordinates{X: 3, Y: 3}, "pkg"},
		{"member keeps its container", term.Coordinates{X: 8, Y: 3}, "pkg.Symbol"},
		{"method keeps the whole path", term.Coordinates{X: 15, Y: 3}, "pkg.Symbol.Method"},
		{"last rune of method", term.Coordinates{X: 18, Y: 3}, "pkg.Symbol.Method"},
		{"separator names the segment it introduces",
			term.Coordinates{X: 5, Y: 3}, "pkg.Symbol"},
		{"second separator", term.Coordinates{X: 12, Y: 3}, "pkg.Symbol.Method"},
		{"qualifier of a call result is dropped",
			term.Coordinates{X: 7, Y: 4}, "Method"},
		{"separator after a call result",
			term.Coordinates{X: 5, Y: 4}, "Method"},
		{"trailing separator falls back to the left",
			term.Coordinates{X: 5, Y: 5}, "pkg"},

		{"on a tab", term.Coordinates{X: 1, Y: 6}, ""},
		{"on a space", term.Coordinates{X: 3, Y: 6}, ""},
		{"on a NUL", term.Coordinates{X: 5, Y: 6}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, wordAt(rows, tc.pos))
		})
	}

	t.Run("qualified type in a signature", func(t *testing.T) {
		x := strings.Index(decl, "Command") + 2
		assert.Equal(t, "textapi.Command", wordAt(rows, term.Coordinates{X: x}))
	})
}

type cursorTestHandler struct {
	*browsertest.TestHandler
	uri workspaceapi.URI
}

func (h cursorTestHandler) Resource() workspaceapi.URI { return h.uri }

type cursorTestView struct{ rows [][]term.Cell }

func (v cursorTestView) RawCells() ([][]term.Cell, error) { return v.rows, nil }

type cursorTestEditor struct {
	h    textapi.Handler
	view textapi.CellView
}

func (e cursorTestEditor) Editor(workspaceapi.URI) (textapi.Handler, error) { return e.h, nil }
func (e cursorTestEditor) CellView(textapi.Handler) textapi.CellView        { return e.view }

func TestHandleChatAddSymbolDeliversExactCursorContext(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "scope.go")
	source := "package scope\nfunc f() {\n\tvalue := 1\n\t_ = value\n}"
	require.NoError(t, os.WriteFile(path, []byte(source), 0o600))
	uri, err := workspaceapi.ParseURI("file://" + path)
	require.NoError(t, err)
	rootURI, err := workspaceapi.ParseURI("file://" + root)
	require.NoError(t, err)
	windowPos := term.Coordinates{X: 6, Y: 0}
	contentPos := term.Coordinates{X: 6, Y: 3}
	lspPos := semanticapi.Position{Line: 3, Character: 6}
	assertParams := func(doc semanticapi.TextDocumentIdentifier, got semanticapi.Position) {
		assert.Equal(t, "file://"+path, doc.URI)
		assert.Equal(t, lspPos, got)
	}
	lsp := stubLSP{
		workspaceSymbolFn: func(semanticapi.WorkspaceSymbolParams) ([]semanticapi.SymbolInformation, error) {
			t.Fatal("chataddsymbol must not call WorkspaceSymbol")
			return nil, nil
		},
		definitionFn: func(p semanticapi.DefinitionParams) (semanticapi.LocationResult, error) {
			assertParams(p.TextDocument, p.Position)
			loc := semanticapi.Location{URI: "file://" + path,
				Range: semanticapi.Range{Start: semanticapi.Position{Line: 2, Character: 1}}}
			return semanticapi.LocationResult{Location: &loc}, nil
		},
		referencesFn: func(p semanticapi.ReferenceParams) ([]semanticapi.Location, error) {
			assertParams(p.TextDocument, p.Position)
			return []semanticapi.Location{{URI: "file://" + path,
				Range: semanticapi.Range{Start: semanticapi.Position{Line: 3, Character: 6}}}}, nil
		},
		hoverFn: func(p semanticapi.HoverParams) (*semanticapi.Hover, error) {
			assertParams(p.TextDocument, p.Position)
			return &semanticapi.Hover{Contents: semanticapi.MarkupContent{
				Value: "```go\nvar value int\n```",
			}}, nil
		},
	}
	th := cursorTestHandler{TestHandler: browsertest.NewTestHandler(), uri: uri}
	ch := make(chan dialoguetui.MessageEvent, 1)
	h := &aiEditorHandler{
		ctx: t.Context(), fs: testLocalFS{root: root}, cwd: rootURI,
		lsp: lsp, n: stubNotifications{},
		ed: cursorTestEditor{h: th, view: cursorTestView{
			rows: cellRows(strings.Split(source, "\n")...),
		}},
	}
	h.openChatTx.Store("rolling-fox", (chan<- dialoguetui.MessageEvent)(ch))
	h.recordFocusedChat("rolling-fox")
	h.recordCursor(textapi.Event{URI: uri, Start: windowPos, From: contentPos})

	require.NoError(t, h.handleChatAddSymbol(textapi.Command{Name: commandAddSymbol}))

	select {
	case ev := <-ch:
		assert.Equal(t, dialoguetui.MessageEventAttachment, ev.Type)
		assert.Equal(t, dialoguetui.AttachmentSymbol, ev.Attachment.Kind)
		assert.Equal(t, "value", ev.Attachment.Symbol)
		assert.Equal(t, strings.Join([]string{
			"## Definition",
			"- `scope.go:3`",
			"",
			"## References",
			"- `scope.go:4:\t_ = value`",
			"",
			"## Documentation",
			"```go",
			"var value int",
			"```",
		}, "\n"), ev.Attachment.Content)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the symbol attachment event")
	}
}

func TestHandleChatAddSymbolWithNameUsesNameBasedAttachment(t *testing.T) {
	ch := make(chan dialoguetui.MessageEvent, 1)
	h := &aiEditorHandler{ctx: t.Context()}
	h.openChatTx.Store("rolling-fox", (chan<- dialoguetui.MessageEvent)(ch))
	h.recordFocusedChat("rolling-fox")

	require.NoError(t, h.handleChatAddSymbol(textapi.Command{
		Name: commandAddSymbol, Args: []string{"pkg.Symbol"},
	}))

	select {
	case ev := <-ch:
		assert.Equal(t, dialoguetui.MessageEventAttachment, ev.Type)
		assert.Equal(t, dialoguetui.NewSymbolAttachment("pkg.Symbol"), ev.Attachment)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the symbol attachment event")
	}
}

func TestHandleChatAddSymbolErrors(t *testing.T) {
	ctx := context.Background()
	h := &aiEditorHandler{ctx: ctx}

	// At most one name is accepted.
	require.Error(t, h.handleChatAddSymbol(textapi.Command{
		Name: commandAddSymbol, Args: []string{"pkg.Symbol", "other.Symbol"},
	}))

	// A cursor is required before chat lookup or semantic resolution.
	h.openChatTx.Store("rolling-fox",
		(chan<- dialoguetui.MessageEvent)(make(chan dialoguetui.MessageEvent, 1)))
	require.Error(t, h.handleChatAddSymbol(textapi.Command{Name: commandAddSymbol}))
}

func TestHandleChatAddSymbolRejectsWhitespaceAtCursor(t *testing.T) {
	uri, err := workspaceapi.ParseURI("file:///workspace/scope.go")
	require.NoError(t, err)
	pos := term.Coordinates{X: 5}
	th := cursorTestHandler{TestHandler: browsertest.NewTestHandler(), uri: uri}
	h := &aiEditorHandler{
		ctx: t.Context(),
		ed: cursorTestEditor{h: th, view: cursorTestView{
			rows: cellRows("value other"),
		}},
	}
	h.recordCursor(textapi.Event{URI: uri, From: pos})

	err = h.handleChatAddSymbol(textapi.Command{Name: commandAddSymbol})
	require.ErrorContains(t, err, "no symbol under the cursor")
}
