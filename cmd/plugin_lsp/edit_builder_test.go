package main

import (
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/text"
	"github.com/ernestrc/golang-internal-tools/lsp/protocol"
	"github.com/ernestrc/golang-internal-tools/span"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"testing"
)

const (
	fixture1 = `package config

import (
	"io"
	"os"
	"go.uber.org/config"
	"strings"
)
`
	expected1 = `package config

import (
	"go.uber.org/config"
	"strings"
)
`
	expected2 = `package config

import (
	"go.uber.org/config"
	"io"
	"os"
	"strings"
)
`
)

var (
	edit1 = protocol.TextEdit{
		Range: protocol.Range{
			Start: protocol.Position{Line: 3, Character: 2},
			End:   protocol.Position{Line: 5, Character: 2},
		},
	}
	edit2 = protocol.TextEdit{
		Range: protocol.Range{
			Start: protocol.Position{Line: 5, Character: 20},
			End:   protocol.Position{Line: 5, Character: 20},
		},
		NewText: "\"\n\t\"io\"\n\t\"os",
	}
	edit3 = protocol.TextEdit{Range: protocol.Range{
		Start: protocol.Position{Line: 2, Character: 0},
		End:   protocol.Position{Line: 2, Character: 6}},
	}
	edit4 = protocol.TextEdit{Range: protocol.Range{
		Start: protocol.Position{Line: 2, Character: 6},
		End:   protocol.Position{Line: 2, Character: 6}},
		NewText: "imp",
	}
	edit5 = protocol.TextEdit{Range: protocol.Range{
		Start: protocol.Position{Line: 2, Character: 6},
		End:   protocol.Position{Line: 2, Character: 6}},
		NewText: "ort",
	}
	edits1 = []protocol.TextEdit{edit1}
	edits2 = []protocol.TextEdit{edit1, edit2}
	edits3 = []protocol.TextEdit{edit3, edit4, edit5}
)

func makeFile() *file {
	name := "gopls_espavila.go"
	uri := span.URIFromPath(name)
	docID := protocol.TextDocumentIdentifier{
		URI: protocol.URIFromSpanURI(uri),
	}
	return &file{uri: uri, docID: docID, name: name}
}

func TestApplyEdits(t *testing.T) {
	log.SetLevel(log.TraceLevel)

	tsuite := []struct {
		input  string
		ed     []protocol.TextEdit
		output string
	}{
		{"a", nil, "a"},
		{fixture1, edits1, expected1},
		{fixture1, edits2, expected2},
		{fixture1, edits3, fixture1},
	}

	for _, tcase := range tsuite {
		var out cell.Buffer
		out.Init()
		out.WriteString(tcase.input)

		var b editBuilder
		b.init(makeFile(), text.NewCellEditor(out.Editor()), cell.StringToCells(tcase.input))
		edits := make([]protocol.TextEdit, len(tcase.ed))
		copy(edits, tcase.ed)
		b.applyEdits(edits)
		assert.Equal(t, tcase.output, b.buf.String())
		assert.Equal(t, tcase.output, out.String())
	}
}

func TestSortEdits(t *testing.T) {
	tsuite := []struct {
		in  []protocol.TextEdit
		out []protocol.TextEdit
	}{
		{edits1, edits1},
		{edits2, []protocol.TextEdit{edit2, edit1}},
		{edits3, []protocol.TextEdit{edit5, edit4, edit3}},
	}
	for _, tcase := range tsuite {
		edits := make([]protocol.TextEdit, len(tcase.in))
		copy(edits, tcase.in)
		sortEdits(edits)
		assert.Equal(t, tcase.out, edits)
	}
}
