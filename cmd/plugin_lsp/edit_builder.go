package main

import (
	"errors"
	"sort"

	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/editor"
	"github.com/ernestrc/go-tui/term"
	"github.com/ernestrc/golang-internal-tools/lsp/protocol"
	log "github.com/sirupsen/logrus"
)

type editBuilder struct {
	f        *file
	w        editor.Writer
	original *cell.Buffer
	buf      *cell.Buffer
	colmap   protocol.ColumnMapper

	offsets struct {
		x  int
		y  int
		to term.Coordinates
	}
}

func (b *editBuilder) init(f *file, w editor.Writer, cells [][]term.Cell) {
	b.f = f
	b.w = w
	b.original = cell.CellsToBuffer(cells)
	b.buf = cell.CellsToBuffer(cells)
	b.colmap = getColumnMapper(b.f.uri, b.original)
}

func (b *editBuilder) stateOffset(from, to term.Coordinates, ed protocol.TextEdit) (
	newFrom, newTo term.Coordinates,
) {
	newFrom = from
	newTo = to

	if b.offsets.to.Y == newFrom.Y {
		newFrom.X += b.offsets.x
	}

	if b.offsets.to.Y == newTo.Y {
		newTo.X += b.offsets.x
	}

	newFrom.Y += b.offsets.y
	newTo.Y += b.offsets.y

	log.Tracef("lspEditorHandler.editBuilder.stateOffset(%#v): "+
		"newFrom: %#v; origFrom: %#v; newTo: %#v; origTo: %#v",
		ed, newFrom, from, newTo, to)

	return newFrom, newTo
}

func (b *editBuilder) setStateOffset(to term.Coordinates, y, x int) {
	b.offsets.to = to
	b.offsets.x = x
	b.offsets.y = y

	log.Tracef("lspEditorHandler.editBuilder.setStateOffset(%#v, y=%d, x=%d)", to, y, x)
}

func (b *editBuilder) setDeleteStateOffset(
	origTo, start, end term.Coordinates, ed protocol.TextEdit,
) {
	xoffset := b.offsets.x
	yoffset := b.offsets.y
	linexoffset := start.X - end.X
	if b.offsets.to.Y == end.Y {
		xoffset += linexoffset
	} else {
		xoffset = linexoffset
	}
	yoffset += int(ed.Range.Start.Line) - int(ed.Range.End.Line)
	b.setStateOffset(origTo, yoffset, xoffset)
}

func (b *editBuilder) setInsertStateOffset(
	origTo, start, end term.Coordinates, ed protocol.TextEdit,
) {
	linexoffset := 0
	xoffset := b.offsets.x
	yoffset := b.offsets.y
	if start.Y == end.Y {
		linexoffset = end.X - start.X
	} else {
		linexoffset = end.X
	}
	if b.offsets.to.Y == end.Y {
		xoffset += linexoffset
	} else {
		xoffset = linexoffset
	}
	yoffset += end.Y - start.Y
	b.setStateOffset(origTo, yoffset, xoffset)
}

func (b *editBuilder) applyEdit(ed protocol.TextEdit) error {
	from, to, ok := convertRange(ed.Range, b.original.RawCells(), b.colmap)
	if !ok {
		return errors.New("could not convert rage")
	}

	origTo := to
	from, to = b.stateOffset(from, to, ed)

	var err error

	// range is right exclusive so start == end signals no delete
	if ed.Range.Start != ed.Range.End {
		_, _, _, err = b.w.Delete(from, to)
		start, end, _ := b.buf.Delete(from, to)
		b.setDeleteStateOffset(origTo, start, end, ed)
	}

	if err != nil {
		return err
	}

	if ed.NewText != "" {
		_, _, err = b.w.Insert(from, ed.NewText)
		start, end := b.buf.InsertString(from, ed.NewText)
		b.setInsertStateOffset(origTo, start, end, ed)
	}

	if err != nil {
		return err
	}

	return nil
}

func (b *editBuilder) applyWorkspaceEdit(ed protocol.WorkspaceEdit) {
	for _, ch := range ed.DocumentChanges {
		if ch.TextDocument.TextDocumentIdentifier != b.f.docID {
			continue
		}
		b.applyEdits(ch.Edits)
	}
}

func (b *editBuilder) applyEdits(eds []protocol.TextEdit) {
	sort.Slice(eds, func(i, j int) bool {
		if eds[i].Range.Start.Line < eds[j].Range.Start.Line {
			return true
		}
		if eds[i].Range.Start.Line > eds[j].Range.Start.Line {
			return false
		}
		return eds[i].Range.Start.Character < eds[j].Range.Start.Character
	})

	for _, ed := range eds {
		err := b.applyEdit(ed)
		if err != nil {
			log.Errorf("lspEditorHandler.applyEdit(%s): %v", b.f.name, err)
			return
		}
	}
}
