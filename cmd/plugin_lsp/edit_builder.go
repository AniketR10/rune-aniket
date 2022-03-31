package main

import (
	"errors"
	"sort"

	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
	"github.com/ernestrc/go-tui/text"
	"github.com/ernestrc/golang-internal-tools/lsp/protocol"
	log "github.com/sirupsen/logrus"
)

type editBuilder struct {
	f        *file
	w        text.CellEditor
	original *cell.Buffer
	buf      *cell.Buffer
	colmap   protocol.ColumnMapper
}

func (b *editBuilder) init(f *file, w text.CellEditor, cells [][]term.Cell) {
	b.f = f
	b.w = w
	b.original = cell.CellsToBuffer(cells)
	b.buf = cell.CellsToBuffer(cells)
	spanURI := workspaceURIToSpan(b.f.uri)
	b.colmap = getColumnMapper(spanURI, b.original)
}

func (b *editBuilder) applyEdit(ed protocol.TextEdit) error {
	start, end, ok := convertRange(ed.Range, b.original.RawCells(), b.colmap)
	if !ok {
		return errors.New("could not convert rage")
	}

	// update remote and local buffer
	_, _, _, err := b.w.Edit(start, end, ed.NewText)
	_, _, _ = b.buf.Edit(start, end, ed.NewText)
	return err
}

func (b *editBuilder) applyWorkspaceEdit(ed protocol.WorkspaceEdit) {
	for _, ch := range ed.DocumentChanges {
		if ch.TextDocument.TextDocumentIdentifier != b.f.docID {
			continue
		}
		b.applyEdits(ch.Edits)
	}
}

func sortEdits(eds []protocol.TextEdit) {
	sort.Slice(eds, func(i, j int) bool {
		if eds[i].Range.Start.Line > eds[j].Range.Start.Line {
			return true
		}
		if eds[i].Range.Start.Line < eds[j].Range.Start.Line {
			return false
		}
		if eds[i].Range.Start.Character > eds[j].Range.Start.Character {
			return true
		}
		if eds[i].Range.Start.Character < eds[j].Range.Start.Character {
			return false
		}
		return i > j
	})
}

func (b *editBuilder) applyEdits(eds []protocol.TextEdit) {
	sortEdits(eds)
	for _, ed := range eds {
		err := b.applyEdit(ed)
		if err != nil {
			log.Errorf("lspEditorHandler.applyEdit(%s): %v", b.f.uri, err)
			return
		}
	}
}
