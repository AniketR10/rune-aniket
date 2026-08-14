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

package lspcmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// PosToCoord converts an LSP Position to terminal Coordinates.
func PosToCoord(p semanticapi.Position) term.Coordinates {
	return term.Coordinates{X: int(p.Character), Y: int(p.Line)}
}

// CoordToPos converts terminal Coordinates to an LSP Position.
func CoordToPos(c term.Coordinates) semanticapi.Position {
	return semanticapi.Position{Line: uint32(c.Y), Character: uint32(c.X)}
}

// TextDocID converts a workspace URI to an LSP TextDocumentIdentifier.
func TextDocID(uri workspaceapi.URI) semanticapi.TextDocumentIdentifier {
	return semanticapi.TextDocumentIdentifier{
		URI: URIToLSP(uri),
	}
}

// URIToLSP converts a workspace URI to an LSP-compatible file:// URI string.
// The language server runs on the workspace host, so only the host-local
// path is sent; LspToURI restores the workspace scheme on the way back.
func URIToLSP(u workspaceapi.URI) string {
	return fmt.Sprintf("file://%s", u.Path())
}

// LspToURI converts an LSP file:// URI string to a workspace URI.
// base is any URI of the workspace the LSP server belongs to (typically
// the workspace root): when base is remote (e.g. ssh://), the file://
// path returned by the server is rebased onto base's scheme and
// authority, since the path is local to the workspace host, not to the
// machine running the IDE.
func LspToURI(base workspaceapi.URI, s string) (workspaceapi.URI, error) {
	u, err := workspaceapi.ParseURI(s)
	if err != nil {
		return workspaceapi.URI{}, err
	}
	if u.Scheme() != "file" || base.Scheme() == "" || base.Scheme() == "file" {
		return u, nil
	}
	return workspaceapi.WithPath(base, u.Path())
}

// ApplyEdits applies a set of LSP TextEdits to a CellEditor in reverse
// document order so that earlier positions remain valid.
//
// Per the LSP spec, when multiple inserts share the same position,
// the array order defines the order in which the inserted strings
// appear in the resulting text. Since we apply edits sequentially
// from bottom to top, same-position inserts must be reversed so the
// first-in-array insert is applied last (ending up first in the text).
func ApplyEdits(
	ctx context.Context, ce textapi.CellEditor, edits []semanticapi.TextEdit,
) error {
	sorted := make([]semanticapi.TextEdit, len(edits))
	copy(sorted, edits)

	sort.SliceStable(sorted, func(i, j int) bool {
		si := sorted[i].Range.Start
		sj := sorted[j].Range.Start
		if si.Line != sj.Line {
			return si.Line > sj.Line
		}
		return si.Character > sj.Character
	})
	reverseSameStartEdits(sorted)

	for _, edit := range sorted {
		start := PosToCoord(edit.Range.Start)
		end := PosToCoord(edit.Range.End)
		if _, _, _, err := ce.Edit(
			ctx, start, end, edit.NewText,
		); err != nil {
			return err
		}
	}
	return nil
}

// reverseSameStartEdits reverses each contiguous group of edits
// sharing the same start position. This is needed because when
// applying edits bottom-to-top, each insert at position P pushes
// previous text at P downward. Reversing ensures the first-in-array
// insert ends up first in the resulting text, per the LSP spec.
func reverseSameStartEdits(edits []semanticapi.TextEdit) {
	for i := 0; i < len(edits); {
		j := i + 1
		for j < len(edits) &&
			edits[j].Range.Start == edits[i].Range.Start {
			j++
		}
		if j-i > 1 {
			for l, r := i, j-1; l < r; l, r = l+1, r-1 {
				edits[l], edits[r] = edits[r], edits[l]
			}
		}
		i = j
	}
}

// EditsChangeText reports whether applying edits to the document held in
// cells produces different text. Edits that cannot be resolved against
// cells -- out of bounds, overlapping, or in an empty document -- count
// as a change, so a caller that is unsure applies them.
//
// Edit ranges address the document as it is now, so the result is
// stitched together in document order without mutating anything.
func EditsChangeText(cells [][]term.Cell, edits []semanticapi.TextEdit) bool {
	if len(cells) == 0 {
		return true
	}
	sorted := make([]semanticapi.TextEdit, len(edits))
	copy(sorted, edits)
	sort.SliceStable(sorted, func(i, j int) bool {
		si, sj := sorted[i].Range.Start, sorted[j].Range.Start
		if si.Line != sj.Line {
			return si.Line < sj.Line
		}
		return si.Character < sj.Character
	})

	eof := term.Coordinates{Y: len(cells) - 1, X: len(cells[len(cells)-1])}
	var out strings.Builder
	cur := term.Coordinates{}
	for _, edit := range sorted {
		start := PosToCoord(edit.Range.Start)
		end := PosToCoord(edit.Range.End)
		if !coordInDoc(cells, start) || !coordInDoc(cells, end) ||
			coordLess(start, cur) || coordLess(end, start) {
			return true
		}
		out.WriteString(textBetween(cells, cur, start))
		out.WriteString(edit.NewText)
		cur = end
	}
	out.WriteString(textBetween(cells, cur, eof))
	return out.String() != textBetween(cells, term.Coordinates{}, eof)
}

func coordLess(a, b term.Coordinates) bool {
	if a.Y != b.Y {
		return a.Y < b.Y
	}
	return a.X < b.X
}

// coordInDoc also accepts the position one past the last row at column
// zero, which is how servers spell "to the end of the document".
func coordInDoc(cells [][]term.Cell, c term.Coordinates) bool {
	if c.X < 0 || c.Y < 0 {
		return false
	}
	if c.Y == len(cells) {
		return c.X == 0
	}
	return c.Y < len(cells) && c.X <= len(cells[c.Y])
}

func textBetween(cells [][]term.Cell, from, to term.Coordinates) string {
	var b strings.Builder
	for y := from.Y; y <= to.Y && y < len(cells); y++ {
		startX, endX := 0, len(cells[y])
		if y == from.Y {
			startX = from.X
		}
		if y == to.Y && to.X < endX {
			endX = to.X
		}
		for x := startX; x < endX; x++ {
			c := cells[y][x]
			if comb := c.CombiningRunes(); comb != nil {
				b.WriteString(string(append([]rune{c.Ch}, comb...)))
			} else {
				b.WriteRune(c.Ch)
			}
		}
		if y != to.Y {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
